package cmd

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/telemetry"
)

func TestRunIAPReviewScreenshotDeliveryFailureIsInvalidFileWithDeleteHint(t *testing.T) {
	resetReportFlags(t)
	t.Setenv("ASC_APP_ID", "")
	imagePath := writeOutcomeReviewScreenshotPNG(t)
	info, err := os.Stat(imagePath)
	if err != nil {
		t.Fatalf("stat fixture: %v", err)
	}

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots":
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintf(w, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"review.png","fileSize":%d,"uploadOperations":[{"method":"PUT","url":%q,"length":%d,"offset":0}]}}}`, info.Size(), server.URL+"/upload", info.Size())
		case req.Method == http.MethodPut && req.URL.Path == "/upload":
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1":
			fmt.Fprint(w, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"review.png"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1":
			fmt.Fprint(w, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"review.png","assetDeliveryState":{"state":"FAILED","errors":[{"code":"IMAGE_INCORRECT_DIMENSIONS","description":"The image dimensions are not supported."}]}}}}`)
		default:
			t.Errorf("unexpected request %s %s", req.Method, req.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()
	useOutcomeTestClient(t, server.URL)
	gotExit, gotContext := captureOutcomeTelemetry(t)

	_, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"iap", "review-screenshots", "create", "--iap-id", "9000000001", "--file", imagePath}, "4.0.0"); code != ExitError {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitError)
		}
	})

	for _, want := range []string{
		"IMAGE_INCORRECT_DIMENSIONS",
		"The image dimensions are not supported.",
		"asc iap review-screenshots delete --screenshot-id shot-1 --confirm",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want %q", stderr, want)
		}
	}
	assertOutcome(t, *gotExit, *gotContext, ExitError, telemetry.OutcomeExpectedNegative, shared.DiagnosticInvalidInput, "--file")
}

func TestRunSubscriptionReviewScreenshotExistingFailedDeliveryIsConflictWithDeleteHint(t *testing.T) {
	resetReportFlags(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_MAX_RETRIES", "0")
	imagePath := writeOutcomeReviewScreenshotPNG(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/8000000001/appStoreReviewScreenshot" {
			t.Errorf("unexpected request %s %s", req.Method, req.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, `{"data":{"type":"subscriptionAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"review.png","fileSize":1,"sourceFileChecksum":"abc","assetDeliveryState":{"state":"FAILED","errors":[{"code":"IMAGE_INCORRECT_DIMENSIONS","description":"The image dimensions are not supported."}]}}}}`)
	}))
	defer server.Close()
	useOutcomeTestClient(t, server.URL)
	gotExit, gotContext := captureOutcomeTelemetry(t)

	_, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"subscriptions", "review", "screenshots", "create", "--subscription-id", "8000000001", "--file", imagePath}, "4.0.0"); code != ExitConflict {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitConflict)
		}
	})

	for _, want := range []string{
		"shot-1",
		"IMAGE_INCORRECT_DIMENSIONS",
		"The image dimensions are not supported.",
		"asc subscriptions review screenshots delete --screenshot-id shot-1 --confirm",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want %q", stderr, want)
		}
	}
	assertOutcome(t, *gotExit, *gotContext, ExitConflict, telemetry.OutcomeConflict, shared.DiagnosticResourceConflict, "")
}

func TestRunReviewScreenshotCreateMissingFileIsFileNotFound(t *testing.T) {
	for _, args := range [][]string{
		{"iap", "review-screenshots", "create", "--iap-id", "9000000001"},
		{"subscriptions", "review", "screenshots", "create", "--subscription-id", "8000000001"},
	} {
		t.Run(args[0], func(t *testing.T) {
			resetReportFlags(t)
			t.Setenv("ASC_APP_ID", "")
			t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				t.Fatal("unexpected client creation for a missing file")
				return nil, nil
			}))
			gotExit, gotContext := captureOutcomeTelemetry(t)
			missing := filepath.Join(t.TempDir(), "missing.png")

			_, _ = captureCommandOutput(t, func() {
				if code := Run(append(args, "--file", missing), "4.0.0"); code != ExitError {
					t.Fatalf("Run() exit code = %d, want %d", code, ExitError)
				}
			})

			assertOutcome(t, *gotExit, *gotContext, ExitError, telemetry.OutcomeExpectedNegative, shared.DiagnosticFileNotFound, "--file")
		})
	}
}

func writeOutcomeReviewScreenshotPNG(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 1290, 2796))); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	path := filepath.Join(t.TempDir(), "review.png")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write png: %v", err)
	}
	return path
}

func useOutcomeTestClient(t *testing.T, serverURL string) {
	t.Helper()
	client := newHTTPStatusTestClient(t, serverURL)
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))
}

func captureOutcomeTelemetry(t *testing.T) (*int, *telemetry.EventContext) {
	t.Helper()
	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
	var gotExit int
	var gotContext telemetry.EventContext
	emitTelemetry = func(_ string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
		gotExit = exitCode
		gotContext = eventContext
	}
	return &gotExit, &gotContext
}

func assertOutcome(t *testing.T, gotExit int, got telemetry.EventContext, wantExit int, wantOutcome telemetry.OutcomeKind, wantCode shared.DiagnosticCode, wantParameter string) {
	t.Helper()
	if gotExit != wantExit ||
		got.OutcomeKind != wantOutcome ||
		got.DiagnosticCode != string(wantCode) ||
		got.FailureParameter != wantParameter {
		t.Fatalf("telemetry exit=%d context=%+v, want exit=%d outcome=%s diagnostic=%s parameter=%q", gotExit, got, wantExit, wantOutcome, wantCode, wantParameter)
	}
}
