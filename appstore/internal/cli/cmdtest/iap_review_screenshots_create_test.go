package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIAPReviewScreenshotsCreatePrintsVerifiedScreenshot(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	imagePath := filepath.Join(t.TempDir(), "review.png")
	writePNG(t, imagePath, 1242, 2688)
	imageInfo, err := os.Stat(imagePath)
	if err != nil {
		t.Fatalf("stat review screenshot fixture: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	getRequests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots":
			body := fmt.Sprintf(`{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"review.png","fileSize":%d,"uploadOperations":[{"method":"PUT","url":"https://upload.example.com/upload/shot-1","length":%d,"offset":0}]}}}`, imageInfo.Size(), imageInfo.Size())
			return jsonResponse(http.StatusCreated, body)
		case req.Method == http.MethodPut && req.URL.Host == "upload.example.com":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{},
			}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"review.png"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1":
			getRequests++
			return jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"review.png","assetDeliveryState":{"state":"COMPLETE"}}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "review-screenshots", "create",
			"--iap-id", "9000000001",
			"--file", imagePath,
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr != nil {
		t.Fatalf("expected success, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if getRequests != 1 {
		t.Fatalf("expected 1 GET request from polling, got %d", getRequests)
	}

	var output struct {
		Data struct {
			Attributes struct {
				AssetDeliveryState struct {
					State string `json:"state"`
				} `json:"assetDeliveryState"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if output.Data.Attributes.AssetDeliveryState.State != "COMPLETE" {
		t.Fatalf("expected output assetDeliveryState COMPLETE, got %q", output.Data.Attributes.AssetDeliveryState.State)
	}
}

func TestIAPReviewScreenshotsCreateFallsBackToNumericIDAfterLookupTimeout(t *testing.T) {
	setupStableSelectorAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_TIMEOUT", "10ms")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")

	imagePath := filepath.Join(t.TempDir(), "review.png")
	writePNG(t, imagePath, 1242, 2688)
	imageInfo, err := os.Stat(imagePath)
	if err != nil {
		t.Fatalf("stat review screenshot fixture: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := 0
	getRequests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/inAppPurchasesV2":
			<-req.Context().Done()
			return nil, req.Context().Err()
		case req.Method == http.MethodPost && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots":
			if err := req.Context().Err(); err != nil {
				t.Fatalf("expected fresh upload context after lookup timeout, got %v", err)
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read create body: %v", err)
			}
			if !strings.Contains(string(body), `"id":"2024"`) {
				t.Fatalf("expected raw numeric ID in create body, got %s", body)
			}
			resp := fmt.Sprintf(`{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"review.png","fileSize":%d,"uploadOperations":[{"method":"PUT","url":"https://upload.example.com/upload/shot-1","length":%d,"offset":0}]}}}`, imageInfo.Size(), imageInfo.Size())
			return jsonResponse(http.StatusCreated, resp)
		case req.Method == http.MethodPut && req.URL.Host == "upload.example.com":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{},
			}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"review.png"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1":
			getRequests++
			return jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"review.png","assetDeliveryState":{"state":"COMPLETE"}}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "review-screenshots", "create",
			"--app", "app-123",
			"--iap-id", "2024",
			"--file", imagePath,
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr != nil {
		t.Fatalf("expected success, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requests != 5 {
		t.Fatalf("expected lookup timeout followed by upload flow, got %d requests", requests)
	}
	if getRequests != 1 {
		t.Fatalf("expected 1 GET request from polling, got %d", getRequests)
	}

	var output struct {
		Data struct {
			Attributes struct {
				AssetDeliveryState struct {
					State string `json:"state"`
				} `json:"assetDeliveryState"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if output.Data.Attributes.AssetDeliveryState.State != "COMPLETE" {
		t.Fatalf("expected output assetDeliveryState COMPLETE, got %q", output.Data.Attributes.AssetDeliveryState.State)
	}
}
