package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestXcodeCloudProductsTableIncludesRelatedAppBundleID(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/ciProducts" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("limit") != "1" {
				t.Fatalf("expected limit=1, got %q", req.URL.Query().Get("limit"))
			}
			body := `{"data":[{"type":"ciProducts","id":"prod-1","attributes":{"name":"FoundationLab","createdDate":"2025-06-25T08:55:49.429Z","productType":"APP"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/ciProducts/prod-1/app" {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":{"type":"apps","id":"app-1","attributes":{"name":"FoundationLab","bundleId":"com.rudrankriyam.foundationlab","sku":"FoundationLab123","primaryLocale":"en-US"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", callCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode-cloud", "products", "--limit", "1", "--output", "table"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Bundle ID") {
		t.Fatalf("expected Bundle ID header, got %q", stdout)
	}
	if !strings.Contains(stdout, "com.rudrankriyam.foundationlab") {
		t.Fatalf("expected bundle ID value in output, got %q", stdout)
	}
	if callCount != 2 {
		t.Fatalf("expected exactly 2 requests, got %d", callCount)
	}
}
