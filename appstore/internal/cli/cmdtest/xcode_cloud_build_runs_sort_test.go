package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestXcodeCloudBuildRunsListWithSortPassesQuery(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if requestCount != 1 {
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
		}

		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciWorkflows/wf-1/buildRuns" {
			t.Fatalf("expected path /v1/ciWorkflows/wf-1/buildRuns, got %s", req.URL.Path)
		}

		values := req.URL.Query()
		if values.Get("limit") != "2" {
			t.Fatalf("expected limit=2, got %q", values.Get("limit"))
		}
		if values.Get("sort") != "-number" {
			t.Fatalf("expected sort=-number, got %q", values.Get("sort"))
		}

		body := `{"data":[{"type":"ciBuildRuns","id":"run-1","attributes":{"number":42}}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode-cloud", "build-runs", "--workflow-id", "wf-1", "--sort", "-number", "--limit", "2"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"run-1"`) {
		t.Fatalf("expected run ID in output, got %q", stdout)
	}
	if requestCount != 1 {
		t.Fatalf("expected exactly one request, got %d", requestCount)
	}
}
