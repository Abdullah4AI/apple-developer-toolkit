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

const (
	deprecatedBetaBuildLocalizationsListWarning   = "Warning: `asc beta-build-localizations list` is deprecated. Use `asc builds test-notes list` for build-scoped workflows. `--global` remains legacy-only during transition."
	deprecatedBetaBuildLocalizationsCreateWarning = "Warning: `asc beta-build-localizations create` is deprecated. Use `asc builds test-notes create` for build-scoped workflows."
)

func TestDeprecatedBetaBuildLocalizationsRootShowsDeprecationGuidance(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"beta-build-localizations"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if strings.Contains(stderr, "was removed") {
		t.Fatalf("expected deprecation guidance instead of removal guidance, got %q", stderr)
	}
	if !strings.Contains(stderr, "Warning: `asc beta-build-localizations` is deprecated. Use `asc builds test-notes ...` for canonical build-scoped workflows.") {
		t.Fatalf("expected root deprecation warning, got %q", stderr)
	}
}

func TestBetaBuildLocalizationsListGlobalSuccess(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaBuildLocalizations" {
			t.Fatalf("expected path /v1/betaBuildLocalizations, got %s", req.URL.Path)
		}
		body := `{"data":[{"type":"betaBuildLocalizations","id":"bbl-1","attributes":{"locale":"en-US","whatsNew":"Test"}}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"beta-build-localizations", "list", "--global"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	requireStderrContainsWarning(t, stderr, deprecatedBetaBuildLocalizationsListWarning)
	assertOnlyDeprecatedCommandWarnings(t, stderr)
	if !strings.Contains(stdout, `"id":"bbl-1"`) {
		t.Fatalf("expected localization id in output, got %q", stdout)
	}
}

func TestBetaBuildLocalizationsListScopedStillWorks(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/builds/build-1/betaBuildLocalizations" {
			t.Fatalf("expected path /v1/builds/build-1/betaBuildLocalizations, got %s", req.URL.Path)
		}
		body := `{"data":[{"type":"betaBuildLocalizations","id":"bbl-scoped","attributes":{"locale":"en-US"}}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"beta-build-localizations", "list", "--build", "build-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	requireStderrContainsWarning(t, stderr, deprecatedBetaBuildLocalizationsListWarning)
	assertOnlyDeprecatedCommandWarnings(t, stderr)
	if !strings.Contains(stdout, `"id":"bbl-scoped"`) {
		t.Fatalf("expected scoped localization in output, got %q", stdout)
	}
}

func TestBetaBuildLocalizationsCreateUpsertUpdatesExistingLocale(t *testing.T) {
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
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1/betaBuildLocalizations" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			if req.URL.Query().Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", req.URL.Query().Get("limit"))
			}
			body := `{"data":[{"type":"betaBuildLocalizations","id":"loc-1","attributes":{"locale":"en-US","whatsNew":"Old"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodPatch || req.URL.Path != "/v1/betaBuildLocalizations/loc-1" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("failed to read update payload: %v", err)
			}
			if !strings.Contains(string(payload), `"whatsNew":"Updated notes"`) {
				t.Fatalf("expected update payload to contain whatsNew, got %s", string(payload))
			}
			body := `{"data":{"type":"betaBuildLocalizations","id":"loc-1","attributes":{"locale":"en-US","whatsNew":"Updated notes"}}}`
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

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"beta-build-localizations", "create",
			"--build", "build-1",
			"--locale", "en-US",
			"--whats-new", "Updated notes",
			"--upsert",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	requireStderrContainsWarning(t, stderr, deprecatedBetaBuildLocalizationsCreateWarning)
	assertOnlyDeprecatedCommandWarnings(t, stderr)
	if !strings.Contains(stdout, `"id":"loc-1"`) {
		t.Fatalf("expected updated localization output, got %q", stdout)
	}
}
