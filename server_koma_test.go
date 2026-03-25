package grpc

import (
	"reflect"
	"strings"
	"testing"
)

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

func TestParseKomaConfig(t *testing.T) {
	tests := []struct {
		name              string
		coresEnv          string
		workersPerCoreEnv string
		workerModeEnv     string
		configuredWorkers uint32
		want              komaConfig
		wantErr           string
	}{
		{
			name: "disabled by default",
			want: komaConfig{workerMode: komaWorkerModePinned},
		},
		{
			name:     "cores enable koma with one worker per core by default",
			coresEnv: "0,1",
			want: komaConfig{
				enabled:    true,
				cores:      []int{0, 1},
				numWorkers: 2,
				workerMode: komaWorkerModePinned,
			},
		},
		{
			name:              "explicit server option worker count is preserved",
			coresEnv:          "2,4",
			configuredWorkers: 5,
			want: komaConfig{
				enabled:    true,
				cores:      []int{2, 4},
				numWorkers: 5,
				workerMode: komaWorkerModePinned,
			},
		},
		{
			name:              "workers per core expands worker count",
			coresEnv:          "5",
			workersPerCoreEnv: "3",
			workerModeEnv:     string(komaWorkerModeRuntime),
			want: komaConfig{
				enabled:        true,
				cores:          []int{5},
				numWorkers:     3,
				workersPerCore: 3,
				workerMode:     komaWorkerModeRuntime,
			},
		},
		{
			name:              "workers per core multiplies across multiple cores",
			coresEnv:          "1,3",
			workersPerCoreEnv: "2",
			want: komaConfig{
				enabled:        true,
				cores:          []int{1, 3},
				numWorkers:     4,
				workersPerCore: 2,
				workerMode:     komaWorkerModePinned,
			},
		},
		{
			name:              "workers per core conflicts with server option",
			coresEnv:          "0",
			workersPerCoreEnv: "2",
			configuredWorkers: 4,
			wantErr:           grpcKomaWorkersPerCoreEnvName + " and NumStreamWorkers cannot both be set",
		},
		{
			name:              "workers per core requires cores",
			workersPerCoreEnv: "2",
			wantErr:           grpcKomaWorkersPerCoreEnvName + " is set but " + grpcKomaCoresEnvName + " is not",
		},
		{
			name:          "worker mode requires cores",
			workerModeEnv: string(komaWorkerModeRuntime),
			wantErr:       grpcKomaWorkerModeEnvName + " is set but " + grpcKomaCoresEnvName + " is not",
		},
		{
			name:          "invalid worker mode",
			workerModeEnv: "fast",
			wantErr:       `invalid ` + grpcKomaWorkerModeEnvName + ` value "fast"`,
		},
		{
			name:              "invalid workers per core",
			coresEnv:          "0",
			workersPerCoreEnv: "0",
			wantErr:           `invalid ` + grpcKomaWorkersPerCoreEnvName + ` value "0"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseKomaConfig(test.coresEnv, test.workersPerCoreEnv, test.workerModeEnv, test.configuredWorkers)
			if test.wantErr != "" {
				if err == nil {
					t.Fatalf("parseKomaConfig() = %+v, want error containing %q", got, test.wantErr)
				}
				if !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("parseKomaConfig() error = %q, want substring %q", err.Error(), test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseKomaConfig() returned unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("parseKomaConfig() = %+v, want %+v", got, test.want)
			}
		})
	}
}
