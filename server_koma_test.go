package grpc

import "testing"

func TestEffectiveInitialConnWindowSize(t *testing.T) {
	tests := []struct {
		name       string
		server     Server
		wantWindow int32
	}{
		{
			name:       "koma disabled uses configured zero",
			server:     Server{opts: serverOptions{}, komaEnabled: false},
			wantWindow: 0,
		},
		{
			name:       "koma enabled falls back to fixed default when unset",
			server:     Server{opts: serverOptions{}, komaEnabled: true},
			wantWindow: komaDefaultInitialConnWindowSize,
		},
		{
			name: "koma enabled falls back when below http2 minimum",
			server: Server{
				opts:        serverOptions{initialConnWindowSize: minHTTP2FlowControlWindow - 1},
				komaEnabled: true,
			},
			wantWindow: komaDefaultInitialConnWindowSize,
		},
		{
			name: "koma enabled keeps explicit minimum-or-larger value",
			server: Server{
				opts:        serverOptions{initialConnWindowSize: minHTTP2FlowControlWindow},
				komaEnabled: true,
			},
			wantWindow: minHTTP2FlowControlWindow,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.server.effectiveInitialConnWindowSize(); got != test.wantWindow {
				t.Fatalf("effectiveInitialConnWindowSize() = %d, want %d", got, test.wantWindow)
			}
		})
	}
}
