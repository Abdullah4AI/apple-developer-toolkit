package asc

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

// setFastAssetUploadRetries keeps retry-backed asset upload tests quick while
// still exercising the real retry decision path.
func setFastAssetUploadRetries(t *testing.T, maxRetries string) {
	t.Helper()

	t.Setenv("ASC_MAX_RETRIES", maxRetries)
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")
	resetConfigCacheForTest()
	t.Cleanup(resetConfigCacheForTest)
}

func TestUploadAssetFromFileRetriesTransientPartFailure(t *testing.T) {
	setFastAssetUploadRetries(t, "2")

	file := createTempAssetFile(t, []byte("abcdef"))
	defer file.Close()

	var (
		mu       sync.Mutex
		attempts = map[string]int{}
		bodies   = map[string][]string{}
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read upload body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		mu.Lock()
		attempts[r.URL.Path]++
		attempt := attempts[r.URL.Path]
		bodies[r.URL.Path] = append(bodies[r.URL.Path], string(body))
		mu.Unlock()

		if r.URL.Path == "/part2" && attempt == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ops := []UploadOperation{
		{Method: http.MethodPut, URL: server.URL + "/part1", Length: 3, Offset: 0},
		{Method: http.MethodPut, URL: server.URL + "/part2", Length: 3, Offset: 3},
	}

	if err := UploadAssetFromFile(context.Background(), file, 6, ops); err != nil {
		t.Fatalf("UploadAssetFromFile() error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if attempts["/part1"] != 1 {
		t.Fatalf("expected 1 request for part1, got %d", attempts["/part1"])
	}
	if attempts["/part2"] != 2 {
		t.Fatalf("expected transient part2 failure to be retried once, got %d requests", attempts["/part2"])
	}
	for path, want := range map[string]string{"/part1": "abc", "/part2": "def"} {
		for i, got := range bodies[path] {
			if got != want {
				t.Fatalf("%s attempt %d uploaded %q, want %q", path, i+1, got, want)
			}
		}
	}
}

func TestUploadAssetFromFileDoesNotRetryClientErrorStatus(t *testing.T) {
	setFastAssetUploadRetries(t, "3")

	file := createTempAssetFile(t, []byte("abc"))
	defer file.Close()

	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	ops := []UploadOperation{
		{Method: http.MethodPut, URL: server.URL + "/part1", Length: 3, Offset: 0},
	}

	err := UploadAssetFromFile(context.Background(), file, 3, ops)
	if err == nil {
		t.Fatal("expected upload to fail")
	}
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("expected a 400 response not to be retried, got %d requests", got)
	}
	if !strings.Contains(err.Error(), "400") {
		t.Fatalf("expected error to name the status, got %v", err)
	}
	if !strings.Contains(err.Error(), "upload operation 0") {
		t.Fatalf("expected error to name the operation index, got %v", err)
	}
}

func TestUploadAssetFromFileHonorsRetryAfterHeader(t *testing.T) {
	setFastAssetUploadRetries(t, "5")

	file := createTempAssetFile(t, []byte("abc"))
	defer file.Close()

	var requests int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	originalTransport := http.DefaultTransport
	http.DefaultTransport = inProcessTransport(handler)
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	ops := []UploadOperation{
		{Method: http.MethodPut, URL: "https://upload.example.test/part1", Length: 3, Offset: 0},
	}

	var err error
	synctest.Test(t, func(*testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()
		err = UploadAssetFromFile(ctx, file, 3, ops)
	})
	if err == nil {
		t.Fatal("expected upload to fail")
	}
	// A 60s Retry-After exceeds both the 1ms retry cap and this request's
	// remaining context budget, so the client fails fast before a second
	// attempt is made and names both recovery constraints.
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("expected over-cap Retry-After to fail before a retry, got %d requests", got)
	}
	if message := err.Error(); !strings.Contains(message, "retry cap") || !strings.Contains(message, "context deadline") {
		t.Fatalf("expected the cap and context budget in the error, got %v", err)
	}
	if strings.Contains(err.Error(), "App Store Connect") {
		t.Fatalf("expected external upload failure to avoid App Store Connect attribution, got %v", err)
	}
	if !strings.Contains(err.Error(), "upload server") {
		t.Fatalf("expected external upload source in the error, got %v", err)
	}
}

func TestUploadAssetFromFileDoesNotReplayNonPUTOperations(t *testing.T) {
	setFastAssetUploadRetries(t, "3")

	file := createTempAssetFile(t, []byte("abc"))
	defer file.Close()

	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	ops := []UploadOperation{
		{Method: http.MethodPost, URL: server.URL + "/part1", Length: 3, Offset: 0},
	}

	if err := UploadAssetFromFile(context.Background(), file, 3, ops); err == nil {
		t.Fatal("expected upload to fail")
	}
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("expected non-PUT operation not to be replayed, got %d requests", got)
	}
}
