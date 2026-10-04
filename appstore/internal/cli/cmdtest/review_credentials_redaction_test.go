package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

// Unique sentinels make a leak unambiguous: no fixture, help text, or unrelated
// field can produce these strings by accident.
const (
	appStoreDemoPasswordSentinel   = "asc-red-sentinel-appstore-demo-pw-9f31c7"
	testFlightDemoPasswordSentinel = "asc-red-sentinel-testflight-demo-pw-4b82ad"
	redactedDemoPasswordText       = "(redacted)"
	includeSensitiveWarningText    = "Warning: --include-sensitive prints secrets"
)

func appStoreReviewDetailJSON() string {
	return `{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{` +
		`"contactFirstName":"Dev","contactLastName":"Support","contactEmail":"dev@example.com",` +
		`"demoAccountRequired":true,"demoAccountName":"reviewer@example.com",` +
		`"demoAccountPassword":"` + appStoreDemoPasswordSentinel + `",` +
		`"notes":"Reviewer notes"}}}`
}

func betaAppReviewDetailsJSON() string {
	return `{"data":[{"type":"betaAppReviewDetails","id":"detail-1","attributes":{` +
		`"contactFirstName":"Dev","contactLastName":"Support","contactEmail":"dev@example.com",` +
		`"demoAccountRequired":true,"demoAccountName":"reviewer@example.com",` +
		`"demoAccountPassword":"` + testFlightDemoPasswordSentinel + `",` +
		`"notes":"Reviewer notes"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/betaAppReviewDetails"}}`
}

func betaAppReviewDetailJSON() string {
	return `{"data":{"type":"betaAppReviewDetails","id":"detail-1","attributes":{` +
		`"demoAccountRequired":true,"demoAccountName":"reviewer@example.com",` +
		`"demoAccountPassword":"` + testFlightDemoPasswordSentinel + `"}}}`
}

func includedAppStoreReviewDetailJSON() string {
	return `{"type":"appStoreReviewDetails","id":"detail-1","attributes":{` +
		`"demoAccountPassword":"` + appStoreDemoPasswordSentinel + `","notes":"keep"}}`
}

func assertNoSentinel(t *testing.T, label, sentinel, content string) {
	t.Helper()
	if strings.Contains(content, sentinel) {
		t.Fatalf("%s leaked the demo account password sentinel: %q", label, content)
	}
}

func TestReviewDetailsGetRedactsDemoAccountPasswordInEveryFormat(t *testing.T) {
	formats := []struct {
		name string
		args []string
	}{
		{name: "json", args: []string{"--output", "json"}},
		{name: "pretty json", args: []string{"--output", "json", "--pretty"}},
		{name: "table", args: []string{"--output", "table"}},
		{name: "markdown", args: []string{"--output", "markdown"}},
	}

	for _, format := range formats {
		t.Run(format.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			stubTransport(t, func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.Path != "/v1/appStoreReviewDetails/detail-1" {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				return jsonResponse(http.StatusOK, appStoreReviewDetailJSON())
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			args := append([]string{"review", "details-get", "--id", "detail-1"}, format.args...)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			assertNoSentinel(t, "stdout", appStoreDemoPasswordSentinel, stdout)
			assertNoSentinel(t, "stderr", appStoreDemoPasswordSentinel, stderr)
			if !strings.Contains(stdout, "detail-1") {
				t.Fatalf("expected detail id in output, got %q", stdout)
			}
			if format.name == "json" || format.name == "pretty json" {
				if !strings.Contains(stdout, redactedDemoPasswordText) {
					t.Fatalf("expected %q placeholder in output, got %q", redactedDemoPasswordText, stdout)
				}
			}
		})
	}
}

func TestReviewDetailsGetIncludesDemoAccountPasswordWithIncludeSensitive(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	stubTransport(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, appStoreReviewDetailJSON())
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"review", "details-get",
			"--id", "detail-1",
			"--output", "json",
			"--include-sensitive",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	var payload struct {
		Data struct {
			Attributes struct {
				DemoAccountPassword string `json:"demoAccountPassword"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if payload.Data.Attributes.DemoAccountPassword != appStoreDemoPasswordSentinel {
		t.Fatalf("expected real password with --include-sensitive, got %q", payload.Data.Attributes.DemoAccountPassword)
	}
	if !strings.Contains(stderr, includeSensitiveWarningText) {
		t.Fatalf("expected plaintext-secret warning on stderr, got %q", stderr)
	}
}

func TestReviewDetailsForVersionRedactsDemoAccountPassword(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	stubTransport(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/appStoreVersions/version-1/appStoreReviewDetail" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		return jsonResponse(http.StatusOK, appStoreReviewDetailJSON())
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"review", "details-for-version", "--version-id", "version-1", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	assertNoSentinel(t, "stdout", appStoreDemoPasswordSentinel, stdout)
	assertNoSentinel(t, "stderr", appStoreDemoPasswordSentinel, stderr)
	if !strings.Contains(stdout, redactedDemoPasswordText) {
		t.Fatalf("expected redaction placeholder, got %q", stdout)
	}
}

func TestIncludedReviewDetailsRedactDemoAccountPasswordByDefault(t *testing.T) {
	tests := []struct {
		name string
		path string
		args []string
		body string
	}{
		{
			name: "versions view",
			path: "/v1/appStoreVersions/version-1",
			args: []string{
				"versions", "view", "--version-id", "version-1",
				"--include", "appStoreReviewDetail", "--output", "json",
			},
			body: `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"}},"included":[` +
				includedAppStoreReviewDetailJSON() + `]}`,
		},
		{
			name: "review attachment list",
			path: "/v1/appStoreReviewDetails/detail-1/appStoreReviewAttachments",
			args: []string{
				"review", "attachments-list", "--review-detail", "detail-1",
				"--include", "appStoreReviewDetail", "--detail-fields", "demoAccountPassword", "--output", "json",
			},
			body: `{"data":[],"included":[` + includedAppStoreReviewDetailJSON() + `]}`,
		},
		{
			name: "review attachment get",
			path: "/v1/appStoreReviewAttachments/attachment-1",
			args: []string{
				"review", "attachments-get", "--id", "attachment-1",
				"--include", "appStoreReviewDetail", "--detail-fields", "demoAccountPassword", "--output", "json",
			},
			body: `{"data":{"type":"appStoreReviewAttachments","id":"attachment-1","attributes":{"fileName":"review.pdf"}},"included":[` +
				includedAppStoreReviewDetailJSON() + `]}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			stubTransport(t, func(req *http.Request) (*http.Response, error) {
				if req.URL.Path != test.path {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				return jsonResponse(http.StatusOK, test.body)
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			assertNoSentinel(t, "stdout", appStoreDemoPasswordSentinel, stdout)
			assertNoSentinel(t, "stderr", appStoreDemoPasswordSentinel, stderr)
			if !strings.Contains(stdout, redactedDemoPasswordText) {
				t.Fatalf("expected redaction placeholder, got %q", stdout)
			}
		})
	}
}

func TestReviewDetailsCreateSendsPasswordButRedactsResponse(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	sentRealPassword := false
	stubTransport(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/appStoreReviewDetails" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		if strings.Contains(string(payload), `"demoAccountPassword":"`+appStoreDemoPasswordSentinel+`"`) {
			sentRealPassword = true
		}
		return jsonResponse(http.StatusCreated, appStoreReviewDetailJSON())
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"review", "details-create",
			"--version-id", "version-1",
			"--contact-first-name", "Dev",
			"--contact-last-name", "Support",
			"--contact-email", "dev@example.com",
			"--contact-phone", "+1 408 555 0100",
			"--output", "json",
			"--demo-account-required=true",
			"--demo-account-name", "reviewer@example.com",
			"--demo-account-password", appStoreDemoPasswordSentinel,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !sentRealPassword {
		t.Fatal("expected the create request payload to carry the explicitly supplied password")
	}
	assertNoSentinel(t, "stdout", appStoreDemoPasswordSentinel, stdout)
	assertNoSentinel(t, "stderr", appStoreDemoPasswordSentinel, stderr)
}

func TestReviewDetailsUpdateSendsPasswordButRedactsResponse(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	sentRealPassword := false
	stubTransport(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/appStoreReviewDetails/detail-1" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		if strings.Contains(string(payload), `"demoAccountPassword":"`+appStoreDemoPasswordSentinel+`"`) {
			sentRealPassword = true
		}
		return jsonResponse(http.StatusOK, appStoreReviewDetailJSON())
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"review", "details-update",
			"--id", "detail-1",
			"--output", "json",
			"--demo-account-password", appStoreDemoPasswordSentinel,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !sentRealPassword {
		t.Fatal("expected the update request payload to carry the explicitly supplied password")
	}
	assertNoSentinel(t, "stdout", appStoreDemoPasswordSentinel, stdout)
	assertNoSentinel(t, "stderr", appStoreDemoPasswordSentinel, stderr)
}

func TestTestFlightReviewViewRedactsDemoAccountPassword(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	stubTransport(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, betaAppReviewDetailsJSON())
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "review", "view", "--app", "app-1", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	assertNoSentinel(t, "stdout", testFlightDemoPasswordSentinel, stdout)
	assertNoSentinel(t, "stderr", testFlightDemoPasswordSentinel, stderr)
	if !strings.Contains(stdout, redactedDemoPasswordText) {
		t.Fatalf("expected redaction placeholder, got %q", stdout)
	}
}

func TestTestFlightReviewEditSendsPasswordButRedactsResponse(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	sentRealPassword := false
	stubTransport(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		if strings.Contains(string(payload), `"demoAccountPassword":"`+testFlightDemoPasswordSentinel+`"`) {
			sentRealPassword = true
		}
		return jsonResponse(http.StatusOK, betaAppReviewDetailJSON())
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"testflight", "review", "edit",
			"--id", "detail-1",
			"--output", "json",
			"--demo-account-password", testFlightDemoPasswordSentinel,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !sentRealPassword {
		t.Fatal("expected the update request payload to carry the explicitly supplied password")
	}
	assertNoSentinel(t, "stdout", testFlightDemoPasswordSentinel, stdout)
	assertNoSentinel(t, "stderr", testFlightDemoPasswordSentinel, stderr)
}

func TestTestFlightReviewViewIncludesDemoAccountPasswordWithIncludeSensitive(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	stubTransport(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, betaAppReviewDetailsJSON())
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"testflight", "review", "view",
			"--app", "app-1",
			"--output", "json",
			"--include-sensitive",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stdout, testFlightDemoPasswordSentinel) {
		t.Fatalf("expected real password with --include-sensitive, got %q", stdout)
	}
	if !strings.Contains(stderr, includeSensitiveWarningText) {
		t.Fatalf("expected plaintext-secret warning on stderr, got %q", stderr)
	}
}

func stubTransport(t *testing.T, fn roundTripFunc) {
	t.Helper()
	original := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = original
	})
	http.DefaultTransport = fn
}
