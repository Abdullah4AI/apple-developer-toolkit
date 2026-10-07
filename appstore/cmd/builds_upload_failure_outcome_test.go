package cmd

import (
	"archive/zip"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/telemetry"
)

type buildFailureOutcome struct {
	exitCode int
	context  telemetry.EventContext
}

func setupBuildFailureOutcomeTest(t *testing.T, transport metadataUsageRoundTripFunc) *buildFailureOutcome {
	t.Helper()
	resetReportFlags(t)
	resetSelectedProfile(t)
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeRunTestECDSAPEM(t, keyPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "missing.json"))
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "ENVKEY")
	t.Setenv("ASC_ISSUER_ID", "ENVISS")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_STRICT_AUTH", "")
	t.Setenv("ASC_MAX_RETRIES", "0")
	t.Setenv("ASC_APP_ID", "")
	t.Cleanup(shared.SetBuildUploadFailureDiagnosticsForTesting(func(context.Context, *asc.Client, string, *asc.BuildUploadResponse) (string, error) {
		return "", nil
	}))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = transport

	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
	outcome := &buildFailureOutcome{}
	emitTelemetry = func(_ string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
		outcome.exitCode = exitCode
		outcome.context = eventContext
	}
	return outcome
}

func buildFailureJSONResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func writeBuildFailureOutcomeIPA(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.ipa")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create IPA: %v", err)
	}
	zipWriter := zip.NewWriter(file)
	entry, err := zipWriter.Create("Payload/Demo.app/Info.plist")
	if err != nil {
		t.Fatalf("create Info.plist entry: %v", err)
	}
	if _, err := io.WriteString(entry, `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>com.example.demo</string>
<key>CFBundleShortVersionString</key><string>1.0.0</string>
<key>CFBundleVersion</key><string>42</string>
</dict></plist>`); err != nil {
		t.Fatalf("write Info.plist entry: %v", err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close IPA: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close IPA file: %v", err)
	}
	return path
}

func TestRunBuildsUploadAppleSideFailureOutcomes(t *testing.T) {
	tests := []struct {
		name        string
		uploadState string
		putStatus   int
		wantStderr  string
		wantOutcome telemetry.OutcomeKind
		wantCode    shared.DiagnosticCode
		wantStatus  int
	}{
		{
			name:        "upload failed",
			uploadState: `"state":{"state":"FAILED","errors":[{"code":"90062"},{"code":"90186"}]}`,
			putStatus:   http.StatusOK,
			wantStderr:  "recovery: increase the marketing version",
			wantOutcome: telemetry.OutcomeExpectedNegative,
			wantCode:    shared.DiagnosticStateNotReady,
		},
		{
			name:        "processing invalid",
			uploadState: `"state":{"state":"COMPLETE"}},"relationships":{"build":{"data":{"type":"builds","id":"build-1"}}`,
			putStatus:   http.StatusOK,
			wantStderr:  "build processing failed: INVALID",
			wantOutcome: telemetry.OutcomeExpectedNegative,
			wantCode:    shared.DiagnosticStateNotReady,
		},
		{
			name:        "presigned upload forbidden",
			putStatus:   http.StatusForbidden,
			wantStderr:  "status Forbidden; the upload URL may have expired, rerun the upload",
			wantOutcome: telemetry.OutcomeAPIClientError,
			wantStatus:  http.StatusForbidden,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outcome := setupBuildFailureOutcomeTest(t, func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789":
					return buildFailureJSONResponse(http.StatusOK, `{"data":{"type":"apps","id":"123456789","attributes":{"name":"Demo","bundleId":"com.example.demo"}}}`)
				case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploads":
					return buildFailureJSONResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.0.0","cfBundleVersion":"42","platform":"IOS"}}}`)
				case req.Method == http.MethodPost && req.URL.Path == "/v1/buildUploadFiles":
					return buildFailureJSONResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"fileName":"app.ipa","fileSize":4,"uti":"com.apple.itunes.ipa","assetType":"ASSET","uploadOperations":[{"method":"PUT","url":"https://upload.example.com/part-1","length":4,"offset":0}]}}}`)
				case req.Method == http.MethodPut && req.URL.Host == "upload.example.com":
					return buildFailureJSONResponse(test.putStatus, "")
				case req.Method == http.MethodPatch && req.URL.Path == "/v1/buildUploadFiles/file-1":
					return buildFailureJSONResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"uploaded":true}}}`)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/buildUploads/upload-1":
					return buildFailureJSONResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.0.0","cfBundleVersion":"42","platform":"IOS",`+test.uploadState+`}}}`)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1":
					return buildFailureJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"INVALID"}}}`)
				default:
					return buildFailureJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"not found"}]}`)
				}
			})

			_, stderr := captureCommandOutput(t, func() {
				Run([]string{
					"builds", "upload",
					"--app", "123456789",
					"--ipa", writeBuildFailureOutcomeIPA(t),
					"--wait",
					"--poll-interval", "1ms",
				}, "1.0.0")
			})

			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			if outcome.exitCode != ExitError ||
				outcome.context.OutcomeKind != test.wantOutcome ||
				outcome.context.DiagnosticCode != string(test.wantCode) ||
				outcome.context.HTTPStatus != test.wantStatus {
				t.Fatalf("unexpected telemetry: exit=%d context=%+v", outcome.exitCode, outcome.context)
			}
		})
	}
}

func TestRunPublishTestFlightUnknownGroupIsResourceNotFound(t *testing.T) {
	outcome := setupBuildFailureOutcomeTest(t, func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789/betaGroups" {
			return buildFailureJSONResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"External QA"}}],"links":{}}`)
		}
		t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		return buildFailureJSONResponse(http.StatusInternalServerError, "")
	})

	_, stderr := captureCommandOutput(t, func() {
		Run([]string{"publish", "testflight", "--app", "123456789", "--build-id", "build-1", "--group", "Missing"}, "1.0.0")
	})

	if !strings.Contains(stderr, `beta group "Missing" not found`) {
		t.Fatalf("stderr = %q, want group-not-found message", stderr)
	}
	if outcome.exitCode != ExitError ||
		outcome.context.OutcomeKind != telemetry.OutcomeExpectedNegative ||
		outcome.context.DiagnosticCode != string(shared.DiagnosticResourceNotFound) ||
		outcome.context.FailureParameter != "--group" {
		t.Fatalf("unexpected telemetry: exit=%d context=%+v", outcome.exitCode, outcome.context)
	}
}

func TestRunBuildsUploadLocalInputFailureOutcomes(t *testing.T) {
	notZip := filepath.Join(t.TempDir(), "app.ipa")
	if err := os.WriteFile(notZip, []byte("not a zip"), 0o600); err != nil {
		t.Fatalf("write IPA: %v", err)
	}
	tests := []struct {
		name        string
		args        []string
		wantStderr  string
		wantExit    int
		wantOutcome telemetry.OutcomeKind
		wantCode    shared.DiagnosticCode
	}{
		{
			name:        "dry run with wait",
			args:        []string{"--ipa", writeBuildFailureOutcomeIPA(t), "--dry-run", "--wait"},
			wantStderr:  "builds upload: --wait is not supported with --dry-run",
			wantExit:    ExitUsage,
			wantOutcome: telemetry.OutcomeUsageError,
		},
		{
			name:        "IPA is not a zip",
			args:        []string{"--ipa", notZip},
			wantStderr:  "builds upload: inspect IPA metadata: open IPA",
			wantExit:    ExitError,
			wantOutcome: telemetry.OutcomeExpectedNegative,
			wantCode:    shared.DiagnosticFileInvalidFormat,
		},
		{
			name:        "IPA is a directory",
			args:        []string{"--ipa", t.TempDir()},
			wantStderr:  "--ipa must be a file",
			wantExit:    ExitError,
			wantOutcome: telemetry.OutcomeExpectedNegative,
			wantCode:    shared.DiagnosticInvalidInput,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outcome := setupBuildFailureOutcomeTest(t, func(req *http.Request) (*http.Response, error) {
				t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
				return buildFailureJSONResponse(http.StatusInternalServerError, "")
			})

			_, stderr := captureCommandOutput(t, func() {
				Run(append([]string{"builds", "upload", "--app", "123456789"}, test.args...), "1.0.0")
			})

			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			if outcome.exitCode != test.wantExit ||
				outcome.context.OutcomeKind != test.wantOutcome ||
				outcome.context.DiagnosticCode != string(test.wantCode) {
				t.Fatalf("unexpected telemetry: exit=%d context=%+v", outcome.exitCode, outcome.context)
			}
		})
	}
}
