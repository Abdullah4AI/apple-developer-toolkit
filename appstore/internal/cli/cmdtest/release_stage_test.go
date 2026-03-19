package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestReleaseStage_MissingMetadataSource(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"release", "stage",
			"--app", "APP_123",
			"--version", "1.2.3",
			"--build", "BUILD_123",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "exactly one of --metadata-dir or --copy-metadata-from is required") {
		t.Fatalf("expected metadata source error, got %q", stderr)
	}
}

func TestReleaseStage_InvalidCopyFieldsValue(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"release", "stage",
			"--app", "APP_123",
			"--version", "1.2.3",
			"--build", "BUILD_123",
			"--copy-metadata-from", "1.2.2",
			"--copy-fields", "description,notAField",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "--copy-fields must be one of") {
		t.Fatalf("expected invalid copy-fields error, got %q", stderr)
	}
}

func TestReleaseStage_DryRunCopyMetadataFromVersion(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/APP_123/appStoreVersions" {
			t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
		}
		body := `{"data":[]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"release", "stage",
			"--app", "APP_123",
			"--version", "1.2.3",
			"--build", "BUILD_123",
			"--copy-metadata-from", "1.2.2",
			"--copy-fields", "description,keywords",
			"--exclude-fields", "keywords",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"status":"dry-run"`) {
		t.Fatalf("expected dry-run status, got %q", stdout)
	}
	if !strings.Contains(stdout, `"copyMetadataFrom":"1.2.2"`) {
		t.Fatalf("expected copy metadata source in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"message":"would create app store version"`) {
		t.Fatalf("expected ensure version dry-run message, got %q", stdout)
	}
	if !strings.Contains(stdout, `"message":"metadata copy plan deferred until version exists"`) {
		t.Fatalf("expected deferred metadata copy message, got %q", stdout)
	}
	if requestCount != 1 {
		t.Fatalf("expected exactly one request, got %d", requestCount)
	}
}
