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

func TestRunBuildsAddGroupsUnprocessableIsExpectedNegativeOutcome(t *testing.T) {
	resetReportFlags(t)
	t.Setenv("ASC_APP_ID", "")

	postCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1/app":
			fmt.Fprint(w, `{"data":{"type":"apps","id":"app-1"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/betaGroups":
			fmt.Fprint(w, `{"data":[{"type":"betaGroups","id":"group-external","attributes":{"name":"External QA","isInternalGroup":false}}]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/builds/build-1/relationships/betaGroups":
			postCount++
			w.WriteHeader(http.StatusUnprocessableEntity)
			fmt.Fprint(w, `{"errors":[{"status":"422","code":"STATE_ERROR.ENTITY_STATE_INVALID","title":"The request cannot be fulfilled","detail":"This build cannot be added to the requested beta group."}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1":
			fmt.Fprint(w, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"PROCESSING","expired":false}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1/buildBetaDetail":
			fmt.Fprint(w, `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"externalBuildState":"PROCESSING"}}}`)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
		}
	}))
	defer server.Close()

	client := newHTTPStatusTestClient(t, server.URL)
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	}))

	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
	var gotExitCode int
	var gotContext telemetry.EventContext
	emitTelemetry = func(_ string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
		gotExitCode = exitCode
		gotContext = eventContext
	}

	wantExit := HTTPStatusToExitCode(http.StatusUnprocessableEntity)
	_, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{
			"builds", "add-groups",
			"--build-id", "build-1",
			"--group", "group-external",
		}, "4.0.0"); code != wantExit {
			t.Fatalf("Run() exit code = %d, want %d", code, wantExit)
		}
	})

	if postCount != 1 {
		t.Fatalf("expected one beta-group POST, got %d", postCount)
	}
	if !strings.Contains(stderr, "This build cannot be added to the requested beta group.") ||
		!strings.Contains(stderr, "Current build state: processingState=PROCESSING") {
		t.Fatalf("stderr = %q, want Apple detail and current state", stderr)
	}
	if gotExitCode != wantExit || gotContext.OutcomeKind != telemetry.OutcomeExpectedNegative {
		t.Fatalf("unexpected telemetry: exit=%d context=%+v", gotExitCode, gotContext)
	}
	if gotContext.HTTPStatus != http.StatusUnprocessableEntity ||
		gotContext.DiagnosticCode != string(shared.DiagnosticStateNotReady) {
		t.Fatalf("unexpected diagnostic context: %+v", gotContext)
	}
}
