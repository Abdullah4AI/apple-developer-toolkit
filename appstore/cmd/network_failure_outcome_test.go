package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/telemetry"
)

func TestRunNetworkFailureIsTransportErrorWithHint(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "apps list", args: []string{"apps", "list"}},
		{name: "app-events localization lookup", args: []string{"app-events", "screenshots", "list", "--event-id", "event-1"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetReportFlags(t)
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_MAX_RETRIES", "0")

			server := httptest.NewServer(http.NotFoundHandler())
			server.Close()
			client := newHTTPStatusTestClient(t, server.URL)
			t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))

			originalEmitTelemetry := emitTelemetry
			t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
			var gotContext telemetry.EventContext
			emitTelemetry = func(_ string, _ string, _ time.Duration, _ int, eventContext telemetry.EventContext) {
				gotContext = eventContext
			}

			_, stderr := captureCommandOutput(t, func() {
				if code := Run(test.args, "4.0.0"); code != ExitError {
					t.Fatalf("Run() exit code = %d, want %d", code, ExitError)
				}
			})

			if !strings.Contains(stderr, "connection refused") || !strings.Contains(stderr, "Hint: Check your network connection") {
				t.Fatalf("stderr = %q, want the dial error and a network hint", stderr)
			}
			if gotContext.OutcomeKind != telemetry.OutcomeTransportError || gotContext.FailureStage != telemetry.FailureStageRequest {
				t.Fatalf("telemetry context = %+v, want transport_error at request stage", gotContext)
			}
		})
	}
}
