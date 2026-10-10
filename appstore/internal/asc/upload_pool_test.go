package asc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func uploadPoolServer(t testing.TB) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	connections := new(atomic.Int64)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
		}
	}
	server.StartTLS()
	previous := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	t.Cleanup(func() { http.DefaultTransport = previous; server.Close() })
	return server, connections
}

func TestClientAssetUploadsReuseConnections(t *testing.T) {
	server, connections := uploadPoolServer(t)
	file := createTempAssetFile(t, []byte("abc"))
	defer file.Close()
	client := new(Client)
	defer client.CloseUploadConnections()
	for range 10 {
		if err := client.UploadAssetFromFile(nil, file, 3, []UploadOperation{{Method: http.MethodPut, URL: server.URL, Length: 3}}); err != nil { //nolint:staticcheck // SA1012: preserve this API's explicit nil-context compatibility.
			t.Fatal(err)
		}
	}
	if got := connections.Load(); got != 1 {
		t.Fatalf("successive assets opened %d connections, want 1", got)
	}
}

func BenchmarkAssetUploadConnections(b *testing.B) {
	server, connections := uploadPoolServer(b)
	file, err := os.CreateTemp(b.TempDir(), "upload")
	if err != nil {
		b.Fatal(err)
	}
	defer file.Close()
	if _, err := file.WriteString("abc"); err != nil {
		b.Fatal(err)
	}
	ops := []UploadOperation{{Method: http.MethodPut, URL: server.URL, Length: 3}}
	b.ResetTimer()
	for range b.N {
		client := new(Client)
		for range 10 {
			if err := client.UploadAssetFromFile(context.Background(), file, 3, ops); err != nil {
				b.Fatal(err)
			}
		}
		client.CloseUploadConnections()
	}
	b.StopTimer()
	b.ReportMetric(float64(connections.Load())/float64(b.N), "connections/batch")
}

func TestClientUploadPoolConcurrentRetriesKeepHeadersAndCredentialsIsolated(t *testing.T) {
	setFastAssetUploadRetries(t, "1")
	var attempts [4]atomic.Int64
	arrived := make(chan struct{}, 4)
	release := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		index, err := strconv.Atoi(r.URL.Path[1:])
		if err != nil || index < 0 || index >= 4 {
			t.Error("invalid worker index")
			w.WriteHeader(400)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || string(body) != "abc" {
			t.Errorf("body = %q, error = %v", body, err)
		}
		if got := r.Header.Get("X-Asset"); got != strconv.Itoa(index) {
			t.Errorf("asset header = %q, want %d", got, index)
		}
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("API credentials reached upload host")
		}
		attempt := attempts[index].Add(1)
		if attempt == 1 {
			arrived <- struct{}{}
			<-release
		}
		if index == 0 && attempt == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	previous := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	defer func() { http.DefaultTransport = previous }()
	jar, _ := cookiejar.New(nil)
	target, _ := url.Parse(server.URL)
	jar.SetCookies(target, []*http.Cookie{{Name: "api-session", Value: "private"}})
	apiCalls := new(atomic.Int64)
	client := &Client{httpClient: &http.Client{Jar: jar, Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		apiCalls.Add(1)
		return nil, errors.New("API client used for upload")
	})}}
	defer client.CloseUploadConnections()
	file := createTempAssetFile(t, []byte("abc"))
	defer file.Close()
	var wg sync.WaitGroup
	for index := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ops := []UploadOperation{{Method: http.MethodPut, URL: fmt.Sprintf("%s/%d", server.URL, index), Length: 3, RequestHeaders: []HTTPHeader{{Name: "X-Asset", Value: strconv.Itoa(index)}}}}
			if err := client.UploadAssetFromFile(context.Background(), file, 3, ops); err != nil {
				t.Error(err)
			}
		}()
	}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for range 4 {
		select {
		case <-arrived:
		case <-timer.C:
			close(release)
			wg.Wait()
			t.Fatal("shared pool serialized uploads")
		}
	}
	close(release)
	wg.Wait()
	if attempts[0].Load() != 2 {
		t.Errorf("retry attempts = %d, want 2", attempts[0].Load())
	}
	if apiCalls.Load() != 0 {
		t.Error("API transport used")
	}
}

func TestClientUploadPoolCleanupAndTimeoutResolution(t *testing.T) {
	server, connections := uploadPoolServer(t)
	file := createTempAssetFile(t, []byte("abc"))
	defer file.Close()
	client := new(Client)
	// Closing an unused pool must not initialize it.
	client.CloseUploadConnections()
	if client.uploadTransport != nil {
		t.Fatal("cleanup initialized unused transport")
	}
	ops := []UploadOperation{{Method: http.MethodPut, URL: server.URL, Length: 3}}
	if err := client.UploadAssetFromFile(context.Background(), file, 3, ops); err != nil {
		t.Fatal(err)
	}
	client.CloseUploadConnections()
	if err := client.UploadAssetFromFile(context.Background(), file, 3, ops); err != nil {
		t.Fatal(err)
	}
	defer client.CloseUploadConnections()
	if connections.Load() != 2 {
		t.Errorf("cleanup did not close idle pool: %d connections", connections.Load())
	}
	t.Setenv("ASC_UPLOAD_TIMEOUT", "17ms")
	if got := client.newPooledUploadClient().Timeout; got != 17*time.Millisecond {
		t.Errorf("timeout = %v, want fresh 17ms", got)
	}
}

type ownedByCallerUploadTransport struct{ closed atomic.Bool }

func (t *ownedByCallerUploadTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("unused")
}
func (t *ownedByCallerUploadTransport) CloseIdleConnections() { t.closed.Store(true) }
func TestClientUploadPoolDoesNotCloseCallerTransport(t *testing.T) {
	transport := new(ownedByCallerUploadTransport)
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	defer func() { http.DefaultTransport = previous }()
	client := new(Client)
	if client.newPooledUploadClient().Transport != transport {
		t.Fatal("custom transport was replaced")
	}
	client.CloseUploadConnections()
	if transport.closed.Load() {
		t.Fatal("caller-owned transport closed")
	}
}

func TestClientUploadPoolRejectsRedirects(t *testing.T) {
	var redirected atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if r.URL.Path == "/target" {
			redirected.Store(true)
			return
		}
		http.Redirect(w, r, "/target", http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	file := createTempAssetFile(t, []byte("abc"))
	defer file.Close()
	client := new(Client)
	defer client.CloseUploadConnections()
	err := client.UploadAssetFromFile(context.Background(), file, 3, []UploadOperation{{Method: http.MethodPut, URL: server.URL, Length: 3}})
	if err == nil || redirected.Load() {
		t.Fatalf("redirect protection failed: error=%v redirected=%v", err, redirected.Load())
	}
}
