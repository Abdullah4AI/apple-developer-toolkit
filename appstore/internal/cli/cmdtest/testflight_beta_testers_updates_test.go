package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestTestFlightBetaTestersAddBuildsOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaTesters/tester-1/relationships/builds" {
			t.Fatalf("expected path /v1/betaTesters/tester-1/relationships/builds, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		if !strings.Contains(string(body), `"id":"build-1"`) {
			t.Fatalf("expected build-1 in body, got %s", string(body))
		}
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "testers", "add-builds", "--id", "tester-1", "--build-id", "build-1,build-2"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "Successfully added tester tester-1") {
		t.Fatalf("expected success message, got %q", stderr)
	}
	if !strings.Contains(stdout, `"testerId":"tester-1"`) {
		t.Fatalf("expected tester id in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"buildIds"`) {
		t.Fatalf("expected build ids in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"action":"added"`) {
		t.Fatalf("expected action added in output, got %q", stdout)
	}
}

func TestTestFlightBetaTestersRemoveBuildsOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaTesters/tester-2/relationships/builds" {
			t.Fatalf("expected path /v1/betaTesters/tester-2/relationships/builds, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		if !strings.Contains(string(body), `"id":"build-3"`) {
			t.Fatalf("expected build-3 in body, got %s", string(body))
		}
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "testers", "remove-builds", "--id", "tester-2", "--build-id", "build-3,build-4", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "Successfully removed tester tester-2") {
		t.Fatalf("expected success message, got %q", stderr)
	}
	if !strings.Contains(stdout, `"testerId":"tester-2"`) {
		t.Fatalf("expected tester id in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"buildIds"`) {
		t.Fatalf("expected build ids in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"action":"removed"`) {
		t.Fatalf("expected action removed in output, got %q", stdout)
	}
}

func TestTestFlightBetaTestersRemoveAppsOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaTesters/tester-3/relationships/apps" {
			t.Fatalf("expected path /v1/betaTesters/tester-3/relationships/apps, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		if !strings.Contains(string(body), `"id":"app-1"`) {
			t.Fatalf("expected app-1 in body, got %s", string(body))
		}
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "testers", "remove-apps", "--id", "tester-3", "--app", "app-1,app-2", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "Successfully removed tester tester-3") {
		t.Fatalf("expected success message, got %q", stderr)
	}
	if !strings.Contains(stdout, `"testerId":"tester-3"`) {
		t.Fatalf("expected tester id in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"appIds"`) {
		t.Fatalf("expected app ids in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"action":"removed"`) {
		t.Fatalf("expected action removed in output, got %q", stdout)
	}
}
