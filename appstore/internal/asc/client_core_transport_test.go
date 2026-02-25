package asc

import (
	"net/http"
	"testing"
	"time"
)

type testRoundTripper func(*http.Request) (*http.Response, error)

func (fn testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestNewDefaultHTTPClient_UsesTunedTransport(t *testing.T) {
	client := newDefaultHTTPClient(42 * time.Second)

	if client.Timeout != 42*time.Second {
		t.Fatalf("Timeout = %s, want %s", client.Timeout, 42*time.Second)
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport type = %T, want *http.Transport", client.Transport)
	}
	if transport.MaxIdleConns != defaultMaxIdleConns {
		t.Fatalf("MaxIdleConns = %d, want %d", transport.MaxIdleConns, defaultMaxIdleConns)
	}
	if transport.MaxIdleConnsPerHost != defaultMaxIdleConnsPerHost {
		t.Fatalf("MaxIdleConnsPerHost = %d, want %d", transport.MaxIdleConnsPerHost, defaultMaxIdleConnsPerHost)
	}
}

func TestNewDefaultHTTPClient_RespectsCustomDefaultTransport(t *testing.T) {
	originalDefaultTransport := http.DefaultTransport
	customTransport := testRoundTripper(func(req *http.Request) (*http.Response, error) {
		return nil, nil
	})
	http.DefaultTransport = customTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalDefaultTransport
	})

	client := newDefaultHTTPClient(5 * time.Second)
	if _, ok := client.Transport.(testRoundTripper); !ok {
		t.Fatalf("Transport = %T, want custom transport type %T", client.Transport, customTransport)
	}
}
