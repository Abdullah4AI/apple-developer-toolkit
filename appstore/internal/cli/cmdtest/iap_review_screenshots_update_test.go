package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

func TestIAPReviewScreenshotsUpdateSendsSupportedPayload(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}

		var payload asc.InAppPurchaseAppStoreReviewScreenshotUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Data.Type != asc.ResourceTypeInAppPurchaseAppStoreReviewScreenshots {
			t.Fatalf("expected screenshot resource type, got %q", payload.Data.Type)
		}
		if payload.Data.ID != "shot-1" {
			t.Fatalf("expected screenshot ID shot-1, got %q", payload.Data.ID)
		}
		if payload.Data.Attributes == nil {
			t.Fatal("expected update attributes")
		}
		if payload.Data.Attributes.Uploaded == nil || !*payload.Data.Attributes.Uploaded {
			t.Fatalf("expected uploaded=true, got %#v", payload.Data.Attributes.Uploaded)
		}
		if payload.Data.Attributes.SourceFileChecksum == nil || *payload.Data.Attributes.SourceFileChecksum != "checksum-1" {
			t.Fatalf("expected sourceFileChecksum checksum-1, got %#v", payload.Data.Attributes.SourceFileChecksum)
		}

		return jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"uploaded":true,"sourceFileChecksum":"checksum-1"}}}`)
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "review-screenshots", "update",
			"--screenshot-id", "shot-1",
			"--checksum", "checksum-1",
			"--uploaded", "true",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr != nil {
		t.Fatalf("expected success, got %v", runErr)
	}
	if requestCount != 1 {
		t.Fatalf("expected one PATCH request, got %d", requestCount)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var output struct {
		Data struct {
			Type       string `json:"type"`
			ID         string `json:"id"`
			Attributes struct {
				SourceFileChecksum string `json:"sourceFileChecksum"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("decode output: %v\nstdout=%s", err, stdout)
	}
	if output.Data.Type != string(asc.ResourceTypeInAppPurchaseAppStoreReviewScreenshots) || output.Data.ID != "shot-1" || output.Data.Attributes.SourceFileChecksum != "checksum-1" {
		t.Fatalf("expected returned screenshot resource, got %s", stdout)
	}
}

func TestIAPReviewScreenshotsUpdateSupportsIndividualFields(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantChecksum *string
		wantUploaded *bool
	}{
		{
			name:         "checksum only",
			args:         []string{"--checksum", "checksum-1"},
			wantChecksum: stringPointer("checksum-1"),
		},
		{
			name:         "uploaded false",
			args:         []string{"--uploaded", "false"},
			wantUploaded: boolPointer(false),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPatch || req.URL.Path != "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1" {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
				}
				var payload asc.InAppPurchaseAppStoreReviewScreenshotUpdateRequest
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatalf("decode payload: %v", err)
				}
				if payload.Data.Attributes == nil {
					t.Fatal("expected update attributes")
				}
				if test.wantChecksum == nil && payload.Data.Attributes.SourceFileChecksum != nil {
					t.Fatalf("expected checksum to be omitted, got %q", *payload.Data.Attributes.SourceFileChecksum)
				}
				if test.wantChecksum != nil && (payload.Data.Attributes.SourceFileChecksum == nil || *payload.Data.Attributes.SourceFileChecksum != *test.wantChecksum) {
					t.Fatalf("expected checksum %q, got %#v", *test.wantChecksum, payload.Data.Attributes.SourceFileChecksum)
				}
				if test.wantUploaded == nil && payload.Data.Attributes.Uploaded != nil {
					t.Fatalf("expected uploaded to be omitted, got %t", *payload.Data.Attributes.Uploaded)
				}
				if test.wantUploaded != nil && (payload.Data.Attributes.Uploaded == nil || *payload.Data.Attributes.Uploaded != *test.wantUploaded) {
					t.Fatalf("expected uploaded %t, got %#v", *test.wantUploaded, payload.Data.Attributes.Uploaded)
				}
				return jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"sourceFileChecksum":"checksum-1"}}}`)
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			args := append([]string{"iap", "review-screenshots", "update", "--screenshot-id", "shot-1"}, test.args...)
			args = append(args, "--output", "json")
			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})
			if runErr != nil {
				t.Fatalf("expected success, got %v", runErr)
			}
			if stdout == "" || stderr != "" {
				t.Fatalf("unexpected output: stdout=%q stderr=%q", stdout, stderr)
			}
		})
	}
}

func TestIAPReviewScreenshotsUpdateUsesRegisteredTableRenderer(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"review.png","fileSize":42,"assetType":"SCREENSHOT"}}}`)
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "review-screenshots", "update",
			"--screenshot-id", "shot-1",
			"--uploaded", "false",
			"--output", "table",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("expected success, got %v", runErr)
	}
	if !strings.Contains(stdout, "ID") || !strings.Contains(stdout, "shot-1") {
		t.Fatalf("expected screenshot table output, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestIAPReviewScreenshotsUpdateRequiresAnUpdateFieldBeforeRequest(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "review-screenshots", "update",
			"--screenshot-id", "shot-1",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil || !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected reported usage error, got %v", runErr)
	}
	if !strings.Contains(stderr, "Error: at least one update flag is required") {
		t.Fatalf("expected missing update field diagnostic, got %q", stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if requestCount != 0 {
		t.Fatalf("expected no requests, got %d", requestCount)
	}
}

func TestIAPReviewScreenshotsUpdateTreatsBlankChecksumAsMissing(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "review-screenshots", "update",
			"--screenshot-id", "shot-1",
			"--checksum", "  ",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil || !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", runErr)
	}
	if !strings.Contains(stderr, "Error: at least one update flag is required") {
		t.Fatalf("expected missing update field diagnostic, got %q", stderr)
	}
	if requestCount != 0 {
		t.Fatalf("expected no requests, got %d", requestCount)
	}
}

func TestIAPReviewScreenshotsUpdateRejectsDeprecatedFileBeforeRequest(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "review-screenshots", "update",
			"--screenshot-id", "shot-1",
			"--file", "./review.png",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil || !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", runErr)
	}
	if !strings.Contains(stderr, "Warning: `--file` is deprecated and unsupported") || !strings.Contains(stderr, "Error: `--file` is unsupported") {
		t.Fatalf("expected deprecation and migration diagnostics, got %q", stderr)
	}
	if requestCount != 0 {
		t.Fatalf("expected no requests, got %d", requestCount)
	}
}

func TestIAPReviewScreenshotsUpdateRejectsInvalidUploadedFlag(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=TestIAPReviewScreenshotsUpdateInvalidUploadedFlagHelper", "--", "iap", "review-screenshots", "update", "--screenshot-id", "shot-1", "--uploaded", "maybe")
	command.Env = append(os.Environ(), "ASC_IAP_REVIEW_SCREENSHOT_FLAG_HELPER=1", "ASC_BYPASS_KEYCHAIN=1")
	stdout, err := command.Output()
	if err == nil {
		t.Fatal("expected invalid --uploaded to fail")
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected process exit error, got %v", err)
	}
	if exitErr.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %d with stderr %q", exitErr.ExitCode(), exitErr.Stderr)
	}
	if len(stdout) != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(string(exitErr.Stderr), `invalid value "maybe" for flag -uploaded`) {
		t.Fatalf("expected invalid bool diagnostic, got %q", exitErr.Stderr)
	}
}

func TestIAPReviewScreenshotsUpdateInvalidUploadedFlagHelper(t *testing.T) {
	if os.Getenv("ASC_IAP_REVIEW_SCREENSHOT_FLAG_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Exit(rootcmd.Run(os.Args[i+1:], "1.2.3"))
		}
	}
	os.Exit(2)
}

func TestIAPReviewScreenshotsUpdatePreservesAPIValidationError(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		return jsonResponse(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"INVALID_SOURCE_FILE_CHECKSUM","title":"Invalid Attribute","detail":"checksum does not match uploaded asset"}]}`)
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "review-screenshots", "update",
			"--screenshot-id", "shot-1",
			"--checksum", "bad-checksum",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected API validation error")
	}
	var apiErr *asc.APIError
	if !errors.As(runErr, &apiErr) {
		t.Fatalf("expected APIError in chain, got %v", runErr)
	}
	if apiErr.Code != "INVALID_SOURCE_FILE_CHECKSUM" || apiErr.Detail != "checksum does not match uploaded asset" {
		t.Fatalf("expected API code/detail to be preserved, got %#v", apiErr)
	}
	if !strings.Contains(runErr.Error(), "checksum does not match uploaded asset") {
		t.Fatalf("expected API detail in error, got %v", runErr)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected no command output before top-level formatting, stdout=%q stderr=%q", stdout, stderr)
	}
}

func stringPointer(value string) *string { return &value }

func boolPointer(value bool) *bool { return &value }
