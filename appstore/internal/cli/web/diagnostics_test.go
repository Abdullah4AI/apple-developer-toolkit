package web

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	webcore "github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/web"
)

func TestWebPrivacyPullMissingAppExposesStructuredDiagnostic(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	var err error
	stderr := captureWebDiagnosticStderr(t, func() {
		err = WebPrivacyPullCommand().ParseAndRun(context.Background(), nil)
	})

	if err == nil {
		t.Fatal("expected error")
	}
	if want := "Error: --app is required (or set ASC_APP_ID)\n"; !strings.HasPrefix(stderr, want) {
		t.Fatalf("stderr = %q, want prefix %q", stderr, want)
	}
	if got, want := err.Error(), "--app is required (or set ASC_APP_ID)"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want flag.ErrHelp usage contract", err)
	}
	if kind := shared.ClassifyUsageError(err); kind != shared.UsageErrorMissingRequired {
		t.Fatalf("usage kind = %q, want %q", kind, shared.UsageErrorMissingRequired)
	}

	diagnostic, ok := shared.DiagnosticFromError(err)
	if !ok {
		t.Fatalf("DiagnosticFromError(%v) found no metadata", err)
	}
	if diagnostic.Code != shared.DiagnosticRequiredInputMissing || diagnostic.Parameter != "--app" {
		t.Fatalf("diagnostic = %+v, want required_input_missing for --app", diagnostic)
	}
}

func TestWebAppsCreateMissingRequiredInputExposesStructuredDiagnostics(t *testing.T) {
	originalCanPrompt := appCreateCanPromptInteractivelyFn
	t.Cleanup(func() { appCreateCanPromptInteractivelyFn = originalCanPrompt })
	appCreateCanPromptInteractivelyFn = func() bool { return false }

	tests := []struct {
		name       string
		run        func() error
		wantError  string
		wantStderr string
		wantParam  string
	}{
		{
			name: "every create flag missing",
			run: func() error {
				return RunAppsCreate(context.Background(), AppsCreateRunOptions{})
			},
			wantError:  "missing required flags: --name, --bundle-id, --sku",
			wantStderr: "Error: missing required flags: --name, --bundle-id, --sku\n",
			wantParam:  "",
		},
		{
			name: "only sku missing",
			run: func() error {
				return RunAppsCreate(context.Background(), AppsCreateRunOptions{
					Name:     "My App",
					BundleID: "com.example.app",
				})
			},
			wantError:  "missing required flags: --sku",
			wantStderr: "Error: missing required flags: --sku\n",
			wantParam:  "--sku",
		},
		{
			name: "only name missing",
			run: func() error {
				return RunAppsCreate(context.Background(), AppsCreateRunOptions{
					BundleID: "com.example.app",
					SKU:      "MYAPP123",
				})
			},
			wantError:  "missing required flags: --name",
			wantStderr: "Error: missing required flags: --name\n",
			wantParam:  "--name",
		},
		{
			name: "apple id required without cached session",
			run: func() error {
				appleID := ""
				return promptAppsCreateSessionAppleID(&appleID)
			},
			wantError:  "--apple-id is required when no cached web session is available; run 'asc web auth login --apple-id EMAIL'",
			wantStderr: "Error: --apple-id is required when no cached web session is available; run 'asc web auth login --apple-id EMAIL'\n",
			wantParam:  "--apple-id",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var err error
			stderr := captureWebDiagnosticStderr(t, func() {
				err = test.run()
			})
			if err == nil {
				t.Fatal("expected error")
			}
			if err.Error() != test.wantError {
				t.Fatalf("error = %q, want %q", err, test.wantError)
			}
			if stderr != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want flag.ErrHelp usage contract", err)
			}
			if kind := shared.ClassifyUsageError(err); kind != shared.UsageErrorMissingRequired {
				t.Fatalf("usage kind = %q, want %q", kind, shared.UsageErrorMissingRequired)
			}

			diagnostic, ok := shared.DiagnosticFromError(err)
			if !ok {
				t.Fatalf("DiagnosticFromError(%v) found no metadata", err)
			}
			if diagnostic.Code != shared.DiagnosticRequiredInputMissing || diagnostic.Parameter != test.wantParam {
				t.Fatalf("diagnostic = %+v, want required_input_missing for %q", diagnostic, test.wantParam)
			}
		})
	}
}

func TestWebReviewShowInvalidInputExposesStructuredDiagnostics(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantError  string
		wantStderr string
		wantCode   shared.DiagnosticCode
		wantParam  string
	}{
		{
			name:       "missing app",
			args:       nil,
			wantError:  "--app is required",
			wantStderr: "Error: --app is required\n",
			wantCode:   shared.DiagnosticRequiredInputMissing,
			wantParam:  "--app",
		},
		{
			name:       "malformed pattern",
			args:       []string{"--app", "123456789", "--pattern", "[a-"},
			wantError:  "--pattern is invalid: syntax error in pattern",
			wantStderr: "Error: --pattern is invalid: syntax error in pattern\n",
			wantCode:   shared.DiagnosticInvalidInput,
			wantParam:  "--pattern",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := WebReviewShowCommand()
			if err := command.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("parse flags: %v", err)
			}

			var err error
			stderr := captureWebDiagnosticStderr(t, func() {
				err = command.Exec(context.Background(), nil)
			})
			if err == nil {
				t.Fatal("expected error")
			}
			if err.Error() != test.wantError {
				t.Fatalf("error = %q, want %q", err, test.wantError)
			}
			if stderr != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want flag.ErrHelp usage contract", err)
			}

			diagnostic, ok := shared.DiagnosticFromError(err)
			if !ok {
				t.Fatalf("DiagnosticFromError(%v) found no metadata", err)
			}
			if diagnostic.Code != test.wantCode || diagnostic.Parameter != test.wantParam {
				t.Fatalf("diagnostic = %+v, want code %q parameter %q", diagnostic, test.wantCode, test.wantParam)
			}
		})
	}
}

func TestChooseSubmissionForShowUnknownSubmissionExposesStructuredDiagnostic(t *testing.T) {
	submissions := []webcore.ReviewSubmission{{ID: "submission-1", State: "COMPLETE"}}

	_, _, err := chooseSubmissionForShow(submissions, "submission-404")
	if err == nil {
		t.Fatal("expected error")
	}
	if want := "submission \"submission-404\" was not found for this app"; err.Error() != want {
		t.Fatalf("error = %q, want %q", err, want)
	}
	if errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want non-usage failure contract", err)
	}

	diagnostic, ok := shared.DiagnosticFromError(err)
	if !ok {
		t.Fatalf("DiagnosticFromError(%v) found no metadata", err)
	}
	if diagnostic.Code != shared.DiagnosticResourceNotFound || diagnostic.Parameter != "--submission" {
		t.Fatalf("diagnostic = %+v, want resource_not_found for --submission", diagnostic)
	}
}

func captureWebDiagnosticStderr(t *testing.T, fn func()) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	readResult := make(chan []byte, 1)
	readError := make(chan error, 1)
	go func() {
		data, readErr := io.ReadAll(reader)
		readResult <- data
		readError <- readErr
	}()

	original := os.Stderr
	os.Stderr = writer
	defer func() { os.Stderr = original }()
	fn()
	if err := writer.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}
	os.Stderr = original
	data := <-readResult
	if err := <-readError; err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close stderr reader: %v", err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}
