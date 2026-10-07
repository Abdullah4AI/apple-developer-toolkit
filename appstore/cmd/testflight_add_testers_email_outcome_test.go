package cmd

import (
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

func TestRunTestFlightGroupsAddTestersUnknownEmailIsExpectedNegative(t *testing.T) {
	resetReportFlags(t)
	t.Setenv("ASC_APP_ID", "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/betaGroups/group-1/app":
			fmt.Fprint(w, `{"data":{"type":"apps","id":"app-1"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/betaTesters":
			fmt.Fprint(w, `{"data":[]}`)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
		}
	}))
	defer server.Close()

	client := newHTTPStatusTestClient(t, server.URL)
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))

	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
	var gotContext telemetry.EventContext
	emitTelemetry = func(_ string, _ string, _ time.Duration, _ int, eventContext telemetry.EventContext) {
		gotContext = eventContext
	}

	_, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{
			"testflight", "groups", "add-testers",
			"--group", "group-1",
			"--email", "new@example.com",
		}, "4.0.0"); code != ExitError {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitError)
		}
	})

	for _, want := range []string{
		`groups add-testers: tester email "new@example.com" not found for app "app-1"`,
		`asc testflight testers add --app "app-1" --email "new@example.com" --group "group-1"`,
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want %q", stderr, want)
		}
	}
	if gotContext.OutcomeKind != telemetry.OutcomeExpectedNegative ||
		gotContext.DiagnosticCode != string(shared.DiagnosticResourceNotFound) ||
		gotContext.FailureParameter != "--email" {
		t.Fatalf("telemetry context = %+v, want expected_negative resource_not_found for --email", gotContext)
	}
}
