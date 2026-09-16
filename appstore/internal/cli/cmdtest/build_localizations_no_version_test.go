package cmdtest

import (
	"net/http"
	"strings"
	"sync"
	"testing"

	rootcmd "github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
)

// Apple answers GET /v1/builds/{id}/appStoreVersion with 200 and a null data
// object for any build that is not attached to an App Store version, which is
// the normal state of a TestFlight-only build.
const buildWithoutAppStoreVersionBody = `{"data":null,"links":{"self":"https://api.appstoreconnect.apple.com/v1/builds/build-1/appStoreVersion"}}`

func runBuildLocalizations(t *testing.T, args []string, handler func(req *http.Request) (*http.Response, error)) (int, string, string, []string) {
	t.Helper()
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var mu sync.Mutex
	var requests []string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		mu.Lock()
		requests = append(requests, req.Method+" "+req.URL.Path)
		mu.Unlock()
		return handler(req)
	})

	var exitCode int
	stdout, stderr := captureOutput(t, func() {
		exitCode = rootcmd.Run(append(append([]string{"build-localizations"}, args...), "--output", "json"), "1.2.3")
	})

	mu.Lock()
	gotRequests := append([]string(nil), requests...)
	mu.Unlock()
	return exitCode, stdout, stderr, gotRequests
}

func TestBuildLocalizationsListRejectsBuildWithoutAppStoreVersion(t *testing.T) {
	exitCode, stdout, stderr, requests := runBuildLocalizations(t, []string{"list", "--build-id", "build-1"}, func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1/appStoreVersion" {
			return statusJSONResponse(buildWithoutAppStoreVersionBody), nil
		}
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	if exitCode != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d; stderr=%q", exitCode, rootcmd.ExitUsage, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "the selected build is not attached to an App Store version") {
		t.Fatalf("stderr = %q, want explanation that the build has no App Store version", stderr)
	}
	if !strings.Contains(stderr, "asc builds test-notes list") {
		t.Fatalf("stderr = %q, want pointer to asc builds test-notes for TestFlight notes", stderr)
	}
	if strings.Contains(stderr, "USAGE") {
		t.Fatalf("stderr = %q, want concise guidance without the full usage page", stderr)
	}
	if len(requests) != 1 {
		t.Fatalf("requests = %v, want only the appStoreVersion lookup", requests)
	}
}

func TestBuildLocalizationsCreateRejectsBuildWithoutAppStoreVersionBeforeMutating(t *testing.T) {
	exitCode, stdout, stderr, requests := runBuildLocalizations(t, []string{"create", "--build-id", "build-1", "--locale", "en-US", "--whats-new", "Bug fixes"}, func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/builds/build-1/appStoreVersion" {
			return statusJSONResponse(buildWithoutAppStoreVersionBody), nil
		}
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	if exitCode != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d; stderr=%q", exitCode, rootcmd.ExitUsage, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "the selected build is not attached to an App Store version") {
		t.Fatalf("stderr = %q, want explanation that the build has no App Store version", stderr)
	}
	if !strings.Contains(stderr, "asc builds test-notes create") {
		t.Fatalf("stderr = %q, want pointer to asc builds test-notes create", stderr)
	}
	if !strings.Contains(stderr, `--locale "LOCALE" --whats-new "NOTES"`) {
		t.Fatalf("stderr = %q, want a complete builds test-notes create command", stderr)
	}
	if len(requests) != 1 || requests[0] != "GET /v1/builds/build-1/appStoreVersion" {
		t.Fatalf("requests = %v, want only the appStoreVersion lookup and no POST", requests)
	}
}

func TestBuildLocalizationsListSurfacesAppleBuildNotFound(t *testing.T) {
	exitCode, stdout, stderr, _ := runBuildLocalizations(t, []string{"list", "--build-id", "missing-build"}, func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/builds/missing-build/appStoreVersion" {
			resp := statusJSONResponse(`{"errors":[{"id":"e1","status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'builds' with id 'missing-build'"}]}`)
			resp.StatusCode = http.StatusNotFound
			return resp, nil
		}
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	if exitCode != rootcmd.ExitNotFound {
		t.Fatalf("exit code = %d, want %d; stderr=%q", exitCode, rootcmd.ExitNotFound, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "There is no resource of type 'builds' with id 'missing-build'") {
		t.Fatalf("stderr = %q, want Apple error detail", stderr)
	}
}
