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

func TestRunReviewSubmitMissingVersionIsNotFound(t *testing.T) {
	resetReportFlags(t)
	t.Setenv("ASC_APP_ID", "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/appStoreVersions" {
			t.Fatalf("request = %s %s, want GET /v1/apps/app-1/appStoreVersions", req.Method, req.URL.Path)
		}
		if got := req.URL.Query().Get("filter[platform]"); got != "IOS" {
			t.Fatalf("filter[platform] = %q, want IOS", got)
		}
		w.Header().Set("Content-Type", "application/json")
		if !req.URL.Query().Has("filter[versionString]") {
			// The not-found diagnostic lists the platform's existing versions.
			fmt.Fprint(w, `{"data":[{"type":"appStoreVersions","id":"ver-2","attributes":{"versionString":"2.0.0","platform":"IOS","appVersionState":"PREPARE_FOR_SUBMISSION","createdDate":"2026-08-01T00:00:00Z"}}]}`)
			return
		}
		if got := req.URL.Query().Get("filter[versionString]"); got != "9.9.9" {
			t.Fatalf("filter[versionString] = %q, want 9.9.9", got)
		}
		fmt.Fprint(w, `{"data":[]}`)
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

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{
			"review", "submit",
			"--app", "app-1",
			"--version", "9.9.9",
			"--build-id", "build-1",
			"--dry-run",
		}, "4.0.0"); code != ExitNotFound {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitNotFound)
		}
	})

	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, `app store version not found for version "9.9.9" and platform "IOS"`) {
		t.Fatalf("stderr = %q, want missing-version diagnostic", stderr)
	}
	if !strings.Contains(stderr, "\n  2.0.0  IOS  PREPARE_FOR_SUBMISSION  ver-2\n") ||
		!strings.Contains(stderr, `asc versions update --version-id "ver-2" --version "9.9.9"`) {
		t.Fatalf("stderr = %q, want existing versions and the rename command", stderr)
	}
	if gotExitCode != ExitNotFound || gotContext.OutcomeKind != telemetry.OutcomeNotFound {
		t.Fatalf("unexpected telemetry: exit=%d context=%+v", gotExitCode, gotContext)
	}
}
