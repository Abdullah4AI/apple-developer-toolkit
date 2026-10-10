package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

func TestBuildsListMatchesEquivalentMarketingVersionForPlatform(t *testing.T) {
	setupAuth(t)
	shared.ResetEquivalentVersionNotesForTest()
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
		case 1, 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/preReleaseVersions" {
				t.Fatalf("unexpected pre-release-version request: %s %s", req.Method, req.URL.String())
			}
			wantVersion := "98.76.0"
			body := `{"data":[],"links":{"next":""}}`
			if requestCount == 2 {
				wantVersion = "98.76"
				body = `{"data":[{"type":"preReleaseVersions","id":"prv-equivalent","attributes":{"version":"98.76","platform":"MAC_OS"}}],"links":{"next":""}}`
			}
			if got := req.URL.Query().Get("filter[version]"); got != wantVersion {
				t.Fatalf("filter[version] = %q, want %q", got, wantVersion)
			}
			if got := req.URL.Query().Get("filter[platform]"); got != "MAC_OS" {
				t.Fatalf("filter[platform] = %q, want MAC_OS", got)
			}
			return jsonHTTPResponse(http.StatusOK, body), nil
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds" {
				t.Fatalf("unexpected builds request: %s %s", req.Method, req.URL.String())
			}
			if got := req.URL.Query().Get("filter[preReleaseVersion]"); got != "prv-equivalent" {
				t.Fatalf("filter[preReleaseVersion] = %q, want prv-equivalent", got)
			}
			if got := req.URL.Query().Get("filter[preReleaseVersion.platform]"); got != "MAC_OS" {
				t.Fatalf("filter[preReleaseVersion.platform] = %q, want MAC_OS", got)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"builds","id":"build-equivalent"}]}`), nil
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "list", "--app", "123456789", "--version", "98.76.0", "--platform", "MAC_OS"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if requestCount != 3 {
		t.Fatalf("request count = %d, want 3", requestCount)
	}
	if !strings.Contains(stdout, `"id":"build-equivalent"`) {
		t.Fatalf("expected equivalent-version build, got %q", stdout)
	}
	if !strings.Contains(stderr, `matched version "98.76" for requested "98.76.0"`) {
		t.Fatalf("expected equivalent-version note, got %q", stderr)
	}
}

func TestBuildsCountMatchesEquivalentMarketingVersionForPlatform(t *testing.T) {
	setupAuth(t)
	shared.ResetEquivalentVersionNotesForTest()
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
		case 1, 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/preReleaseVersions" {
				t.Fatalf("unexpected pre-release-version request: %s %s", req.Method, req.URL.String())
			}
			wantVersion := "87.65.0"
			body := `{"data":[],"links":{"next":""}}`
			if requestCount == 2 {
				wantVersion = "87.65"
				body = `{"data":[{"type":"preReleaseVersions","id":"prv-count-equivalent","attributes":{"version":"87.65","platform":"MAC_OS"}}],"links":{"next":""}}`
			}
			if got := req.URL.Query().Get("filter[version]"); got != wantVersion {
				t.Fatalf("filter[version] = %q, want %q", got, wantVersion)
			}
			if got := req.URL.Query().Get("filter[platform]"); got != "MAC_OS" {
				t.Fatalf("filter[platform] = %q, want MAC_OS", got)
			}
			return jsonHTTPResponse(http.StatusOK, body), nil
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds" {
				t.Fatalf("unexpected builds request: %s %s", req.Method, req.URL.String())
			}
			if got := req.URL.Query().Get("filter[preReleaseVersion]"); got != "prv-count-equivalent" {
				t.Fatalf("filter[preReleaseVersion] = %q, want prv-count-equivalent", got)
			}
			if got := req.URL.Query().Get("filter[preReleaseVersion.platform]"); got != "MAC_OS" {
				t.Fatalf("filter[preReleaseVersion.platform] = %q, want MAC_OS", got)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"meta":{"paging":{"total":4,"limit":1}}}`), nil
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "count", "--app", "123456789", "--version", "87.65.0", "--platform", "MAC_OS"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if requestCount != 3 {
		t.Fatalf("request count = %d, want 3", requestCount)
	}
	var result struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if result.Total != 4 {
		t.Fatalf("total = %d, want 4", result.Total)
	}
	if !strings.Contains(stderr, `matched version "87.65" for requested "87.65.0"`) {
		t.Fatalf("expected equivalent-version note, got %q", stderr)
	}
}
