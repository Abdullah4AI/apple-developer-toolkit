package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildsTestNotesUpdateByBuildLocaleNotFound(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/builds/build-1" {
				t.Fatalf("expected path /v1/builds/build-1, got %s", req.URL.Path)
			}
			body := `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/betaBuildLocalizations" {
				t.Fatalf("expected path /v1/betaBuildLocalizations, got %s", req.URL.Path)
			}
			query := req.URL.Query()
			if query.Get("filter[build]") != "build-1" {
				t.Fatalf("expected filter[build]=build-1, got %q", query.Get("filter[build]"))
			}
			if query.Get("filter[locale]") != "en-US" {
				t.Fatalf("expected filter[locale]=en-US, got %q", query.Get("filter[locale]"))
			}
			if query.Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", query.Get("limit"))
			}
			body := `{"data":[]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "test-notes", "update",
			"--build-id", "build-1",
			"--locale", "en-US",
			"--whats-new", "Updated notes",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected localization lookup error, got nil")
	}
	if !strings.Contains(runErr.Error(), `no localization found for build "build-1" and locale "en-US"`) {
		t.Fatalf("expected not-found localization error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on lookup failure, got %q", stdout)
	}
}
