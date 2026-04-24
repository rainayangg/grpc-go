/*
 *
 * Copyright 2026 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package transport

import (
	"context"
	"sync"
	"testing"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

func testKomaMetaHeadersFrame(streamID uint32, method string) *http2.MetaHeadersFrame {
	return &http2.MetaHeadersFrame{
		HeadersFrame: &http2.HeadersFrame{
			FrameHeader: http2.FrameHeader{
				Type:     http2.FrameHeaders,
				StreamID: streamID,
			},
		},
		Fields: []hpack.HeaderField{
			{Name: ":method", Value: "POST"},
			{Name: ":path", Value: method},
			{Name: "content-type", Value: "application/grpc"},
		},
	}
}

func testKomaDataFrame(streamID uint32, endStream bool) *http2.DataFrame {
	flags := http2.Flags(0)
	if endStream {
		flags |= http2.FlagDataEndStream
	}
	return &http2.DataFrame{
		FrameHeader: http2.FrameHeader{
			Type:     http2.FrameData,
			Flags:    flags,
			StreamID: streamID,
		},
	}
}

func TestAnalyzeKomaBatchSingleStream(t *testing.T) {
	frames := []http2.Frame{
		testKomaMetaHeadersFrame(1, "/svc.Foo/Bar"),
		testKomaDataFrame(1, true),
	}

	got := analyzeKomaBatch(frames)
	if got.uniqueStreamIDs != 1 {
		t.Fatalf("analyzeKomaBatch(single stream) uniqueStreamIDs = %d, want 1", got.uniqueStreamIDs)
	}
	if got.metaHeaders != 1 {
		t.Fatalf("analyzeKomaBatch(single stream) metaHeaders = %d, want 1", got.metaHeaders)
	}
	if len(got.frameKinds) != len(frames) {
		t.Fatalf("analyzeKomaBatch(single stream) frameKinds len = %d, want %d", len(got.frameKinds), len(frames))
	}
}

func TestAnalyzeKomaBatchMixedStreams(t *testing.T) {
	frames := []http2.Frame{
		testKomaMetaHeadersFrame(1, "/svc.Foo/Bar"),
		testKomaDataFrame(3, false),
		testKomaMetaHeadersFrame(5, "/svc.Baz/Qux"),
	}

	got := analyzeKomaBatch(frames)
	if got.uniqueStreamIDs != 3 {
		t.Fatalf("analyzeKomaBatch(mixed stream) uniqueStreamIDs = %d, want 3", got.uniqueStreamIDs)
	}
	if got.metaHeaders != 2 {
		t.Fatalf("analyzeKomaBatch(mixed stream) metaHeaders = %d, want 2", got.metaHeaders)
	}
	if len(got.frameKinds) != len(frames) {
		t.Fatalf("analyzeKomaBatch(mixed stream) frameKinds len = %d, want %d", len(got.frameKinds), len(frames))
	}
}

func TestOperateHeadersKomaConcurrentReplyHandles(t *testing.T) {
	st := &http2Server{
		initialWindowSize: initialWindowSize,
	}
	st.logger = prefixLoggerForServerTransport(st)

	type result struct {
		stream *ServerStream
		err    error
	}

	tests := []struct {
		streamID    uint32
		method      string
		replyHandle uint64
	}{
		{streamID: 1, method: "/svc.Foo/Bar", replyHandle: 101},
		{streamID: 3, method: "/svc.Baz/Qux", replyHandle: 202},
	}

	results := make([]result, len(tests))
	var wg sync.WaitGroup
	wg.Add(len(tests))
	for i, test := range tests {
		go func(i int, test struct {
			streamID    uint32
			method      string
			replyHandle uint64
		}) {
			defer wg.Done()
			results[i].stream, results[i].err = st.operateHeadersKoma(context.Background(), testKomaMetaHeadersFrame(test.streamID, test.method), test.replyHandle)
		}(i, test)
	}
	wg.Wait()

	for i, test := range tests {
		if results[i].err != nil {
			t.Fatalf("operateHeadersKoma(stream %d) returned err %v", test.streamID, results[i].err)
		}
		if results[i].stream == nil {
			t.Fatalf("operateHeadersKoma(stream %d) returned nil stream", test.streamID)
		}
		if got := results[i].stream.id; got != test.streamID {
			t.Fatalf("operateHeadersKoma(stream %d) stream id = %d, want %d", test.streamID, got, test.streamID)
		}
		if got := results[i].stream.KomaReplyHandle; got != test.replyHandle {
			t.Fatalf("operateHeadersKoma(stream %d) reply handle = %d, want %d", test.streamID, got, test.replyHandle)
		}
		if got := results[i].stream.Method(); got != test.method {
			t.Fatalf("operateHeadersKoma(stream %d) method = %q, want %q", test.streamID, got, test.method)
		}
	}
}
