package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/telemetry"
)

func TestRunBuildsWaitProcessingOutcome(t *testing.T) {
	tests := []struct {
		name          string
		state         string
		failOnInvalid bool
		wantExit      int
	}{
		{name: "failed", state: "FAILED", wantExit: ExitError},
		{name: "invalid fails", state: "INVALID", failOnInvalid: true, wantExit: ExitError},
		{name: "invalid tolerated", state: "INVALID", wantExit: ExitSuccess},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetReportFlags(t)
			t.Setenv("ASC_APP_ID", "")

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1" {
					fmt.Fprintf(w, `{"data":{"type":"builds","id":"build-1","attributes":{"processingState":%q}}}`, test.state)
					return
				}
				t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()

			client := newHTTPStatusTestClient(t, server.URL)
			t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))

			originalEmitTelemetry := emitTelemetry
			t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
			var gotExitCode int
			var gotContext telemetry.EventContext
			emitTelemetry = func(_ string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
				gotExitCode = exitCode
				gotContext = eventContext
			}

			args := []string{"builds", "wait", "--build-id", "build-1", "--poll-interval", "1ms", "--output", "json"}
			if test.failOnInvalid {
				args = append(args, "--fail-on-invalid")
			}
			stdout, stderr := captureCommandOutput(t, func() {
				if code := Run(args, "4.0.0"); code != test.wantExit {
					t.Fatalf("Run() exit code = %d, want %d", code, test.wantExit)
				}
			})

			if test.wantExit == ExitSuccess {
				var result asc.BuildWaitResult
				if err := json.Unmarshal([]byte(stdout), &result); err != nil {
					t.Fatalf("stdout = %q, want JSON wait result: %v", stdout, err)
				}
				if strings.Contains(stderr, "Error:") || result.BuildID != "build-1" || result.ProcessingState != "INVALID" {
					t.Fatalf("unexpected tolerated INVALID output: stderr=%q result=%+v", stderr, result)
				}
				if gotExitCode != ExitSuccess || gotContext.DiagnosticCode != "" || gotContext.OutcomeKind == telemetry.OutcomeExpectedNegative {
					t.Fatalf("unexpected success telemetry: exit=%d context=%+v", gotExitCode, gotContext)
				}
				return
			}

			if stdout != "" || !strings.Contains(stderr, "build processing failed with state "+test.state) {
				t.Fatalf("stdout=%q stderr=%q, want empty stdout and processing failure", stdout, stderr)
			}
			if gotExitCode != ExitError ||
				gotContext.OutcomeKind != telemetry.OutcomeExpectedNegative ||
				gotContext.DiagnosticCode != string(shared.DiagnosticStateNotReady) {
				t.Fatalf("unexpected telemetry: exit=%d context=%+v", gotExitCode, gotContext)
			}
		})
	}
}
