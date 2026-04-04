package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

const testFlightLegacyBuildWarning = "Warning: `--build` is deprecated. Use `--build-id`."

func runTestFlightBuildIDCommand(t *testing.T, args []string) (string, string, error) {
	t.Helper()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	return stdout, stderr, runErr
}

func TestTestFlightBuildIDAliasConflictErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "review submit conflicting build values",
			args: []string{"testflight", "review", "submit", "--build-id", "BUILD_CANON", "--build", "BUILD_LEGACY", "--confirm"},
		},
		{
			name: "beta testers add-builds conflicting build values",
			args: []string{"testflight", "testers", "add-builds", "--id", "TESTER_ID", "--build-id", "BUILD_CANON", "--build", "BUILD_LEGACY"},
		},
		{
			name: "config export conflicting build values",
			args: []string{"testflight", "config", "export", "--app", "APP_ID", "--output", "./testflight.yaml", "--include-builds", "--build-id", "BUILD_CANON", "--build", "BUILD_LEGACY"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, runErr := runTestFlightBuildIDCommand(t, test.args)
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, "Error: --build conflicts with --build-id; use only --build-id") {
				t.Fatalf("expected conflicting build selector error, got %q", stderr)
			}
			if strings.Contains(stderr, testFlightLegacyBuildWarning) {
				t.Fatalf("expected conflict to fail before deprecation warning, got %q", stderr)
			}
		})
	}
}

func TestTestFlightReviewSubmissionsListBuildAliasWarnsAndMatchesCanonical(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaAppReviewSubmissions" {
			t.Fatalf("expected path /v1/betaAppReviewSubmissions, got %s", req.URL.Path)
		}
		query := req.URL.Query()
		if query.Get("filter[build]") != "build-1" {
			t.Fatalf("expected build filter build-1, got %q", query.Get("filter[build]"))
		}
		body := `{"data":[{"type":"betaAppReviewSubmissions","id":"submission-1","attributes":{"betaReviewState":"WAITING_FOR_REVIEW","submittedDate":"2024-01-01T00:00:00Z"}}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	canonicalStdout, canonicalStderr, canonicalErr := runTestFlightBuildIDCommand(t, []string{
		"testflight", "review", "submissions", "list", "--build-id", "build-1", "--output", "json",
	})
	if canonicalErr != nil {
		t.Fatalf("canonical run error: %v", canonicalErr)
	}
	if canonicalStderr != "" {
		t.Fatalf("expected empty canonical stderr, got %q", canonicalStderr)
	}

	aliasStdout, aliasStderr, aliasErr := runTestFlightBuildIDCommand(t, []string{
		"testflight", "review", "submissions", "list", "--build", "build-1", "--output", "json",
	})
	if aliasErr != nil {
		t.Fatalf("alias run error: %v", aliasErr)
	}
	if canonicalStdout != aliasStdout {
		t.Fatalf("expected identical stdout, canonical=%q alias=%q", canonicalStdout, aliasStdout)
	}
	if !strings.Contains(aliasStderr, testFlightLegacyBuildWarning) {
		t.Fatalf("expected legacy build warning, got %q", aliasStderr)
	}
	if requestCount != 2 {
		t.Fatalf("expected two requests, got %d", requestCount)
	}
}

func TestTestFlightBetaTestersAddBuildsAliasWarnsAndMatchesCanonical(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaTesters/tester-1/relationships/builds" {
			t.Fatalf("expected beta tester builds relationship path, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		bodyText := string(body)
		if !strings.Contains(bodyText, `"id":"build-1"`) || !strings.Contains(bodyText, `"id":"build-2"`) {
			t.Fatalf("expected both build IDs in body, got %q", bodyText)
		}
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	canonicalStdout, canonicalStderr, canonicalErr := runTestFlightBuildIDCommand(t, []string{
		"testflight", "testers", "add-builds", "--id", "tester-1", "--build-id", "build-1,build-2", "--output", "json",
	})
	if canonicalErr != nil {
		t.Fatalf("canonical run error: %v", canonicalErr)
	}
	if strings.Contains(canonicalStderr, testFlightLegacyBuildWarning) {
		t.Fatalf("did not expect canonical warning, got %q", canonicalStderr)
	}

	aliasStdout, aliasStderr, aliasErr := runTestFlightBuildIDCommand(t, []string{
		"testflight", "testers", "add-builds", "--id", "tester-1", "--build", "build-1,build-2", "--output", "json",
	})
	if aliasErr != nil {
		t.Fatalf("alias run error: %v", aliasErr)
	}
	if canonicalStdout != aliasStdout {
		t.Fatalf("expected identical stdout, canonical=%q alias=%q", canonicalStdout, aliasStdout)
	}
	if !strings.Contains(aliasStderr, testFlightLegacyBuildWarning) {
		t.Fatalf("expected legacy build warning, got %q", aliasStderr)
	}
	if !strings.Contains(aliasStderr, "Successfully added tester tester-1 to 2 build(s)") {
		t.Fatalf("expected success message in stderr, got %q", aliasStderr)
	}
	if requestCount != 2 {
		t.Fatalf("expected two requests, got %d", requestCount)
	}
}
