package timetrace

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sys/unix"
)

type Options struct {
	NumShards    int
	BufSizeExp   int
	AutoDumpPath string
}

var BootTime int64

// CalBootTime returns the system boot time as a wall-clock Time.
func CalBootTime() (int64, error) {
	var ts unix.Timespec
	// CLOCK_BOOTTIME includes time spent suspended (sleep).
	if err := unix.ClockGettime(unix.CLOCK_BOOTTIME, &ts); err != nil {
		return 0, fmt.Errorf("ClockGettime: %w", err)
	}
	now := time.Now().UnixNano()
	monoNs := unix.TimespecToNsec(ts)
	return now - monoNs, nil
}

func DefaultOptions() Options {
	return Options{
		NumShards:    runtime.GOMAXPROCS(0) * 4, //4*a shards (a being the max number of cores we use)
		BufSizeExp:   16,
		AutoDumpPath: "/root/grpc-go/timetrace/dump.log",
	}
}

type event struct {
	ts             int64
	format         string
	a0, a1, a2, a3 uint32
	used           uint32
}

type ring struct {
	events []event
	mask   uint32
	next   uint32
}

type shard struct {
	r  ring
	id int
}

var (
	global struct {
		ok     atomic.Bool
		shards []shard
		dumpMu sync.Mutex
	}
)

func Init(opts Options) {
	if global.ok.Load() {
		return // return if already initialized
	}
	var err error
	BootTime, err = CalBootTime()
	if err != nil {
		fmt.Print("timetrace: unable to get boot time:", err)
		os.Exit(1)
	}
	if opts.NumShards <= 0 {
		opts.NumShards = runtime.GOMAXPROCS(0) * 4
	}
	n := 1
	for n < opts.NumShards {
		n <<= 1
	}
	opts.NumShards = n
	if opts.BufSizeExp <= 0 {
		opts.BufSizeExp = 16
	}
	size := 1 << opts.BufSizeExp

	global.shards = make([]shard, opts.NumShards)
	for i := range global.shards {
		global.shards[i] = shard{
			r: ring{
				events: make([]event, size),
				mask:   uint32(size - 1),
			},
			id: i,
		}
	}
	global.ok.Store(true)
}

func nowNS() int64 { return time.Now().UnixNano() - BootTime }

func Record0(format string)                        { rec(format, 0, 0, 0, 0) }
func Record1(format string, a0 uint32)             { rec(format, a0, 0, 0, 0) }
func Record2(format string, a0, a1 uint32)         { rec(format, a0, a1, 0, 0) }
func Record3(format string, a0, a1, a2 uint32)     { rec(format, a0, a1, a2, 0) }
func Record4(format string, a0, a1, a2, a3 uint32) { rec(format, a0, a1, a2, a3) }

func rec(format string, a0, a1, a2, a3 uint32) {
	if !global.ok.Load() {
		return
	}
	s := &global.shards[unix.Gettid()&(len(global.shards)-1)]
	i := s.r.next
	s.r.next += 1
	idx := i & s.r.mask
	ev := &s.r.events[idx]
	ev.ts = nowNS()
	ev.format = format
	ev.a0, ev.a1, ev.a2, ev.a3 = a0, a1, a2, a3
	atomic.StoreUint32(&ev.used, 1)
}

func DumpToFile(path string) error {
	if !global.ok.Load() {
		return fmt.Errorf("timetrace not initialized")
	}
	global.dumpMu.Lock()
	defer global.dumpMu.Unlock()

	global.ok.Store(false)

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	type row struct {
		ts   int64
		text string
	}
	var rows []row

	for sid := range global.shards {
		r := &global.shards[sid].r
		for i := range r.events {
			ev := &r.events[i]
			if atomic.LoadUint32(&ev.used) == 0 || ev.ts == 0 {
				continue
			}
			line := fmt.Sprintf("%d [C%02d] ", ev.ts, sid) +
				fmt.Sprintf(ev.format, ev.a0, ev.a1, ev.a2, ev.a3)
			rows = append(rows, row{ts: ev.ts, text: line})
		}
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].ts < rows[j].ts })
	for _, r := range rows {
		if _, err := fmt.Fprintln(f, r.text); err != nil {
			return err
		}
	}
	return nil
}

func Freeze(path string) error { return DumpToFile(path) }
