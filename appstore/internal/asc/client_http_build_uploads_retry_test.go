package asc

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Apple intermittently answers Build Upload API reads with 404 NOT_FOUND for
// apps that certainly exist (fastlane/fastlane#29908). These tests pin the
// bounded retry to that family of paths and to reads only.

const buildUploadsFlakyNotFoundBody = `{"errors":[{"id":"e1","status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'apps' with id '6759231657'"}]}`

func newBuildUploadsNotFoundServer(t *testing.T, failures int, attempts *atomic.Int32) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		attempt := int(attempts.Add(1))
		w.Header().Set("Content-Type", "application/json")
		if attempt <= failures {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, buildUploadsFlakyNotFoundBody)
			return
		}
		_, _ = io.WriteString(w, `{"data":[{"type":"buildUploads","id":"bu-1"}]}`)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestClientDo_RetriesIntermittentNotFoundOnAppBuildUploads(t *testing.T) {
	setFastRetryEnv(t, "3")

	var attempts atomic.Int32
	server := newBuildUploadsNotFoundServer(t, 2, &attempts)
	client := newMutationRetryTestClient(t, server.Client())

	data, err := client.do(context.Background(), http.MethodGet, server.URL+"/v1/apps/6759231657/buildUploads?limit=1", nil)
	if err != nil {
		t.Fatalf("do() error: %v", err)
	}
	if got, want := string(data), `{"data":[{"type":"buildUploads","id":"bu-1"}]}`; got != want {
		t.Fatalf("do() = %q, want %q", got, want)
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("expected 3 attempts (404, 404, 200), got %d", got)
	}
}

func TestClientDo_BuildUploadsNotFoundRetryIsBoundedAndSurfacesAppleDetail(t *testing.T) {
	setFastRetryEnv(t, "3")

	var attempts atomic.Int32
	server := newBuildUploadsNotFoundServer(t, 4, &attempts)
	client := newMutationRetryTestClient(t, server.Client())

	_, err := client.do(context.Background(), http.MethodGet, server.URL+"/v1/buildUploads/bu-1", nil)
	if err == nil {
		t.Fatal("expected error after exhausting the retry budget")
	}
	if got := attempts.Load(); got != 4 {
		t.Fatalf("expected 4 attempts (1 + 3 retries), got %d", got)
	}
	if !IsRetryBudgetExhausted(err) {
		t.Fatalf("expected retry budget marker, got %v", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError in chain, got %T: %v", err, err)
	}
	if apiErr.Code != "NOT_FOUND" || apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("expected NOT_FOUND/404, got %s/%d", apiErr.Code, apiErr.StatusCode)
	}
	if !IsNotFound(err) {
		t.Fatalf("expected IsNotFound to see through the retry wrappers: %v", err)
	}
	if !strings.Contains(err.Error(), "There is no resource of type 'apps' with id '6759231657'") {
		t.Fatalf("expected Apple's detail in error, got %v", err)
	}
}

func TestClientDo_DoesNotRetryNotFoundOutsideBuildUploads(t *testing.T) {
	setFastRetryEnv(t, "3")

	for _, path := range []string{
		"/v1/apps/6759231657/builds",
		"/v1/apps/6759231657",
		"/v1/builds/b-1",
		"/v1/buildUploadsArchive/x",
		"/v1/apps/6759231657/buildUploads/extra",
	} {
		t.Run(path, func(t *testing.T) {
			var attempts atomic.Int32
			server := newBuildUploadsNotFoundServer(t, 4, &attempts)
			client := newMutationRetryTestClient(t, server.Client())

			_, err := client.do(context.Background(), http.MethodGet, server.URL+path, nil)
			if err == nil {
				t.Fatal("expected 404 error")
			}
			if got := attempts.Load(); got != 1 {
				t.Fatalf("expected a single attempt for a non-buildUploads 404, got %d", got)
			}
			if IsRetryable(err) {
				t.Fatalf("expected a plain terminal error, got retryable %v", err)
			}
			if !IsNotFound(err) {
				t.Fatalf("expected not-found error, got %v", err)
			}
		})
	}
}

func TestClientDo_DoesNotRetryNotFoundOnBuildUploadsWrites(t *testing.T) {
	setFastRetryEnv(t, "3")

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/v1/buildUploads"},
		{http.MethodPost, "/v1/buildUploadFiles"},
		{http.MethodPatch, "/v1/buildUploadFiles/f-1"},
		{http.MethodDelete, "/v1/buildUploads/bu-1"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			var attempts atomic.Int32
			server := newBuildUploadsNotFoundServer(t, 4, &attempts)
			client := newMutationRetryTestClient(t, server.Client())

			var body io.Reader
			if tc.method != http.MethodDelete {
				body = strings.NewReader(`{"data":{"type":"buildUploads"}}`)
			}
			_, err := client.do(context.Background(), tc.method, server.URL+tc.path, body)
			if err == nil {
				t.Fatal("expected 404 error")
			}
			if got := attempts.Load(); got != 1 {
				t.Fatalf("expected a single attempt for a write 404, got %d", got)
			}
			if IsRetryable(err) {
				t.Fatalf("expected a plain terminal error for a write, got retryable %v", err)
			}
			if !IsNotFound(err) {
				t.Fatalf("expected not-found error, got %v", err)
			}
		})
	}
}

func TestClientDo_DoesNotRetryBuildUploadsNotFoundWithOtherCode(t *testing.T) {
	setFastRetryEnv(t, "3")

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"errors":[{"status":"404","code":"PATH_ERROR","title":"Unknown path"}]}`)
	}))
	t.Cleanup(server.Close)
	client := newMutationRetryTestClient(t, server.Client())

	_, err := client.do(context.Background(), http.MethodGet, server.URL+"/v1/apps/6759231657/buildUploads", nil)
	if err == nil {
		t.Fatal("expected 404 error")
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("expected a single attempt for a non-NOT_FOUND 404, got %d", got)
	}
	if IsRetryable(err) {
		t.Fatalf("expected a plain terminal error, got retryable %v", err)
	}
}

// A genuinely missing upload or file id is a terminal 404, not the flake:
// the detail names the buildUploads/buildUploadFiles resource, not the app.
func TestClientDo_DoesNotRetryGenuinelyMissingBuildUploadResources(t *testing.T) {
	setFastRetryEnv(t, "3")

	for _, tc := range []struct {
		path   string
		detail string
	}{
		{"/v1/buildUploads/bu-missing", "There is no resource of type 'buildUploads' with id 'bu-missing'"},
		{"/v1/buildUploads/bu-missing/buildUploadFiles", "There is no resource of type 'buildUploads' with id 'bu-missing'"},
		{"/v1/buildUploadFiles/f-missing", "There is no resource of type 'buildUploadFiles' with id 'f-missing'"},
		{"/v1/buildUploads/bu-missing", ""},
	} {
		t.Run(tc.path+"|"+tc.detail, func(t *testing.T) {
			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				attempts.Add(1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_, _ = io.WriteString(w, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"`+tc.detail+`"}]}`)
			}))
			t.Cleanup(server.Close)
			client := newMutationRetryTestClient(t, server.Client())

			_, err := client.do(context.Background(), http.MethodGet, server.URL+tc.path, nil)
			if err == nil {
				t.Fatal("expected 404 error")
			}
			if got := attempts.Load(); got != 1 {
				t.Fatalf("expected a single attempt for a genuinely missing resource, got %d", got)
			}
			if IsRetryable(err) {
				t.Fatalf("expected a plain terminal error, got retryable %v", err)
			}
			if !IsNotFound(err) {
				t.Fatalf("expected not-found error, got %v", err)
			}
			if IsRetryBudgetExhausted(err) {
				t.Fatalf("expected no retry-budget marker, got %v", err)
			}
		})
	}
}

// The app-scoped view has only one resource that can be missing, so any
// NOT_FOUND there is the flake even when Apple omits the detail.
func TestClientDo_RetriesAppBuildUploadsNotFoundWithoutDetail(t *testing.T) {
	setFastRetryEnv(t, "3")

	for _, path := range []string{
		"/v1/apps/6759231657/buildUploads",
		"/v1/apps/6759231657/relationships/buildUploads",
	} {
		t.Run(path, func(t *testing.T) {
			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if attempts.Add(1) == 1 {
					w.WriteHeader(http.StatusNotFound)
					_, _ = io.WriteString(w, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist"}]}`)
					return
				}
				_, _ = io.WriteString(w, `{"data":[]}`)
			}))
			t.Cleanup(server.Close)
			client := newMutationRetryTestClient(t, server.Client())

			if _, err := client.do(context.Background(), http.MethodGet, server.URL+path, nil); err != nil {
				t.Fatalf("do() error: %v", err)
			}
			if got := attempts.Load(); got != 2 {
				t.Fatalf("expected 2 attempts (404, 200), got %d", got)
			}
		})
	}
}

func TestNotFoundNamesAppResource(t *testing.T) {
	for _, tc := range []struct {
		detail string
		want   bool
	}{
		{"There is no resource of type 'apps' with id '6759231657'", true},
		{"There is no resource of type \u2018apps\u2019 with id \u20186759231657\u2019", true},
		{"there is no resource of type apps with id 1", true},
		{"There is no resource of type 'buildUploads' with id 'bu-1'", false},
		{"There is no resource of type 'buildUploadFiles' with id 'f-1'", false},
		{"There is no resource of type 'appStoreVersions' with id 'v-1'", false},
		{"", false},
	} {
		t.Run(tc.detail, func(t *testing.T) {
			if got := notFoundNamesAppResource(&APIError{Detail: tc.detail}); got != tc.want {
				t.Fatalf("notFoundNamesAppResource(%q) = %v, want %v", tc.detail, got, tc.want)
			}
		})
	}
}

func TestClientDo_BuildUploadsNotFoundRetryHonorsContextCancellation(t *testing.T) {
	setFastRetryEnv(t, "3")
	t.Setenv("ASC_BASE_DELAY", "5s")
	t.Setenv("ASC_MAX_DELAY", "5s")
	resetConfigCacheForTest()

	var attempts atomic.Int32
	server := newBuildUploadsNotFoundServer(t, 4, &attempts)
	client := newMutationRetryTestClient(t, server.Client())

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := client.do(ctx, http.MethodGet, server.URL+"/v1/apps/6759231657/buildUploads", nil)
	if err == nil {
		t.Fatal("expected error after cancellation")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("expected cancellation to interrupt the backoff, waited %s", elapsed)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled in chain, got %v", err)
	}
	if !IsNotFound(err) {
		t.Fatalf("expected the last 404 to be preserved, got %v", err)
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("expected a single attempt before cancellation, got %d", got)
	}
}

func TestClientDo_RespectsMaxRetriesZeroForBuildUploadsNotFound(t *testing.T) {
	setFastRetryEnv(t, "0")

	var attempts atomic.Int32
	server := newBuildUploadsNotFoundServer(t, 4, &attempts)
	client := newMutationRetryTestClient(t, server.Client())

	_, err := client.do(context.Background(), http.MethodGet, server.URL+"/v1/apps/6759231657/buildUploads", nil)
	if err == nil {
		t.Fatal("expected 404 error")
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("expected a single attempt with ASC_MAX_RETRIES=0, got %d", got)
	}
	if !IsNotFound(err) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestIsBuildUploadsPath(t *testing.T) {
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"/v1/buildUploads", true},
		{"/v1/buildUploads/bu-1", true},
		{"/v1/buildUploads/bu-1/buildUploadFiles?limit=5", true},
		{"/v1/buildUploads/bu-1/relationships/buildUploadFiles", true},
		{"/v1/buildUploadFiles", true},
		{"/v1/buildUploadFiles/f-1", true},
		{"/v1/apps/6759231657/buildUploads", true},
		{"/v1/apps/6759231657/buildUploads?filter[cfBundleVersion]=22", true},
		{"/v1/apps/6759231657/relationships/buildUploads", true},
		{"https://api.appstoreconnect.apple.com/v1/apps/6759231657/buildUploads?cursor=abc", true},
		{"https://api.appstoreconnect.apple.com/v1/buildUploads/bu-1", true},
		{"", false},
		{"/v1/apps/6759231657", false},
		{"/v1/apps/6759231657/builds", false},
		{"/v1/apps/6759231657/buildUploads/extra", false},
		{"/v1/buildUploadsArchive", false},
		{"/v1/buildUploadFilesX/f-1", false},
		{"/v1/builds/b-1", false},
		{"/v2/buildUploads", false},
		{"https://api.appstoreconnect.apple.com/v1/builds?filter[app]=1", false},
	} {
		t.Run(tc.path, func(t *testing.T) {
			if got := isBuildUploadsPath(tc.path); got != tc.want {
				t.Fatalf("isBuildUploadsPath(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}
