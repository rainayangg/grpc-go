# Koma Worker / Socket Designs

## Context

In this fork, each Koma worker currently:

- creates one Koma socket
- pulls ready requests from that socket
- processes the request to completion
- sends the response back on the Koma transport path

Today, workers call `runtime.LockOSThread()` and pin their OS thread to a CPU from `GRPC_KOMA_CORES`.

Koma itself performs request routing in the kernel. Ready requests are placed into a logically centralized queue, and any Koma socket may pull them. This means user-space does not need to map a TCP connection to a specific worker/socket for load distribution.

## Problem

With one worker per core, a blocking handler can stall request intake for that worker even if more ready work exists. That can waste CPU opportunity, especially when the worker is pinned to a core and processes requests synchronously.

## Design A: Multiple Pinned Workers Per Core

### Idea

Run multiple workers on the same core, with one Koma socket per worker.

Example:

- 1 core
- 3 workers
- 3 Koma sockets

Each worker remains:

- one goroutine
- locked to one OS thread
- pinned to the same Linux CPU

### Expected behavior

- if one worker blocks, another worker on the same core may continue pulling and processing requests
- Koma can still distribute ready requests across the available sockets

### Pros

- minimal conceptual change from the current design
- likely straightforward to implement and test
- preserves explicit per-core placement

### Cons

- multiple pinned OS threads on one core will be scheduled by Linux
- runnable workers will time-slice on that core and incur thread context-switch overhead
- this gives up much of the normal efficiency benefit of lightweight goroutine scheduling

## Design B: Multiple Unlocked Workers With One Koma Socket Each

### Idea

Still run multiple workers, and still give each worker its own Koma socket, but remove:

- `runtime.LockOSThread()`
- per-worker CPU pinning

Each worker remains a goroutine that owns a Koma socket and runs the same pull/process/respond loop, but it is no longer tied to a specific OS thread or CPU.

### Expected behavior

- blocked handlers no longer imply multiple pinned threads fighting on the same core
- the Go runtime can schedule workers more flexibly across OS threads
- a worker goroutine may run on different OS threads, and therefore on different cores, over time
- the overall design stays closer to the existing Koma processing model than native gRPC's "poller plus handler pool" design

### Runtime scheduling notes

- OS thread count is no longer fixed at one thread per worker
- the Go runtime decides how many OS threads to keep active
- `GOMAXPROCS` bounds how many goroutines may execute Go code at the same time
- if goroutines block in syscalls, cgo, or other runtime-visible blocking paths, the runtime may create additional OS threads so other goroutines can continue running

In other words, Design B keeps multiple workers as goroutines, but lets the runtime multiplex them onto a runtime-managed pool of OS threads instead of constructing one pinned OS thread per worker.

### Pros

- preserves the simple "worker owns socket and handles request to completion" model
- allows overlap across multiple workers without requiring multiple pinned threads per core
- should recover more of Go's normal goroutine scheduling benefits

### Cons

- explicit CPU affinity is lost
- if Koma benefits from strong socket-to-core locality, that benefit may be reduced
- behavior becomes more dependent on Go runtime scheduling instead of explicit placement

## Current Configuration Model

Today:

- `GRPC_KOMA_CORES` defines the list of CPU IDs eligible for Koma workers
- `grpc.NumStreamWorkers(n)` defines the total worker count
- each worker creates exactly one Koma socket

So the current code already supports "more workers than cores" indirectly by setting `n` larger than the number of entries in `GRPC_KOMA_CORES`.

## Possible Future Configuration

Two separate inputs likely make sense:

- `GRPC_KOMA_CORES`
- a new env var such as `GRPC_KOMA_WORKERS_PER_CORE`

These should complement each other rather than be mutually exclusive.

The intended meaning would be:

- `GRPC_KOMA_CORES` selects which Linux CPU IDs are in the Koma worker pool
- `GRPC_KOMA_WORKERS_PER_CORE` selects how many workers, and therefore Koma sockets, should be created for each listed core

This maps naturally to both designs:

- Design A: workers are pinned to the listed cores, with multiple pinned workers possible on one core
- Design B: the same worker count can be used, but workers are not locked to a specific OS thread or CPU after startup

If both designs are supported in one codebase, a separate mode env var could choose the scheduling model, for example:

- `GRPC_KOMA_WORKER_MODE=pinned`
- `GRPC_KOMA_WORKER_MODE=runtime`

## Recommendation

For a first experiment, Design B looks like the better option:

- it targets the blocking problem directly
- it avoids multiplying pinned OS threads on the same core
- it keeps the current Koma ownership model intact

Design A is still a useful fallback because it is closer to the current implementation and may be simpler to evaluate quickly.

## Open Questions

- Does Koma materially benefit from worker/socket CPU affinity in practice?
- How much blocking time in handlers actually exists in the target workloads?
- Is the main bottleneck request intake, handler execution, or response transmission?
- Should worker count remain application-configured, or should a future env var express workers-per-core directly?
- If both designs are supported, should runtime scheduling be opt-in or become the default?
