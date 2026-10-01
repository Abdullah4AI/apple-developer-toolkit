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
	"time"
)

// mutationGate is a fake transport that holds every request until released
// and records the peak number of requests in flight at once.
type mutationGate struct {
	release chan struct{}
	started chan string

	mu          sync.Mutex
	inFlight    int
	maxInFlight int
}

func newMutationGate(capacity int) *mutationGate {
	return &mutationGate{
		release: make(chan struct{}),
		started: make(chan string, capacity),
	}
}

func (g *mutationGate) roundTrip(req *http.Request) (*http.Response, error) {
	g.mu.Lock()
	g.inFlight++
	g.maxInFlight = max(g.maxInFlight, g.inFlight)
	g.mu.Unlock()

	g.started <- req.URL.Path
	<-g.release

	g.mu.Lock()
	g.inFlight--
	g.mu.Unlock()
	return jsonResponse(http.StatusOK, `{"data":{"type":"territoryAvailabilities","id":"ta-1","attributes":{"available":true}}}`), nil
}

func (g *mutationGate) peak() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.maxInFlight
}

// waitStarted waits for exactly n requests to reach the transport, then
// asserts that no further request starts while the gate is held.
func (g *mutationGate) waitStarted(t *testing.T, n int) {
	t.Helper()
	for i := range n {
		select {
		case <-g.started:
		case <-time.After(2 * time.Second):
			t.Fatalf("only %d of %d gated mutating requests reached the transport", i, n)
		}
	}
	select {
	case path := <-g.started:
		t.Fatalf("request %s started beyond the expected %d in flight", path, n)
	case <-time.After(50 * time.Millisecond):
	}
}

func launchGatedPatches(ctx context.Context, client *Client, count int) <-chan error {
	errs := make(chan error, count)
	for range count {
		go func() {
			_, err := client.do(ctx, http.MethodPatch, "/v1/territoryAvailabilities/ta-1", strings.NewReader(`{}`))
			errs <- err
		}()
	}
	return errs
}

func drainGatedPatches(t *testing.T, errs <-chan error, count int) {
	t.Helper()
	for range count {
		if err := <-errs; err != nil {
			t.Fatalf("gated PATCH error: %v", err)
		}
	}
}

func TestClientBulkMutatingRequestsUseBulkLimit(t *testing.T) {
	const requests = BulkMutatingRequestLimit + 4
	gate := newMutationGate(requests)
	client := newMutationRetryTestClient(t, &http.Client{Transport: roundTripFunc(gate.roundTrip)})

	errs := launchGatedPatches(WithBulkMutatingRequestLimit(context.Background()), client, requests)
	gate.waitStarted(t, BulkMutatingRequestLimit)
	close(gate.release)
	drainGatedPatches(t, errs, requests)

	if got := gate.peak(); got != BulkMutatingRequestLimit {
		t.Fatalf("peak concurrent bulk mutating requests = %d, want %d", got, BulkMutatingRequestLimit)
	}
}

func TestClientUnmarkedMutatingRequestsKeepDefaultLimit(t *testing.T) {
	const requests = BulkMutatingRequestLimit + 4
	gate := newMutationGate(requests)
	client := newMutationRetryTestClient(t, &http.Client{Transport: roundTripFunc(gate.roundTrip)})

	errs := launchGatedPatches(context.Background(), client, requests)
	gate.waitStarted(t, defaultMutatingRequestLimit)
	close(gate.release)
	drainGatedPatches(t, errs, requests)

	if got := gate.peak(); got != defaultMutatingRequestLimit {
		t.Fatalf("peak concurrent default mutating requests = %d, want %d", got, defaultMutatingRequestLimit)
	}
}

// Bulk and ordinary writes share one client-wide ceiling, so a bulk fan-out
// that saturates the bulk limit leaves no extra room for ordinary writes.
func TestClientBulkLimitIsClientWideWriteCeiling(t *testing.T) {
	const requests = BulkMutatingRequestLimit + 1
	gate := newMutationGate(requests)
	client := newMutationRetryTestClient(t, &http.Client{Transport: roundTripFunc(gate.roundTrip)})

	bulkErrs := launchGatedPatches(WithBulkMutatingRequestLimit(context.Background()), client, BulkMutatingRequestLimit)
	gate.waitStarted(t, BulkMutatingRequestLimit)

	defaultErrs := launchGatedPatches(context.Background(), client, 1)
	select {
	case path := <-gate.started:
		t.Fatalf("ordinary write %s started while bulk writes held every client-wide slot", path)
	case <-time.After(50 * time.Millisecond):
	}

	close(gate.release)
	drainGatedPatches(t, bulkErrs, BulkMutatingRequestLimit)
	drainGatedPatches(t, defaultErrs, 1)

	if got := gate.peak(); got != BulkMutatingRequestLimit {
		t.Fatalf("peak concurrent mutating requests = %d, want %d", got, BulkMutatingRequestLimit)
	}
}

func TestClientBulkMutatingRequestLimitCanceledWhileWaiting(t *testing.T) {
	gate := newMutationGate(BulkMutatingRequestLimit)
	client := newMutationRetryTestClient(t, &http.Client{Transport: roundTripFunc(gate.roundTrip)})

	bulkCtx := WithBulkMutatingRequestLimit(context.Background())
	errs := launchGatedPatches(bulkCtx, client, BulkMutatingRequestLimit)
	gate.waitStarted(t, BulkMutatingRequestLimit)

	waitingCtx, cancel := context.WithCancel(bulkCtx)
	waiting := launchGatedPatches(waitingCtx, client, 1)
	cancel()
	select {
	case err := <-waiting:
		if err == nil || !strings.Contains(err.Error(), "wait for mutating request slot") {
			t.Fatalf("expected slot-wait cancellation error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("canceled bulk write did not stop waiting for a slot")
	}

	close(gate.release)
	drainGatedPatches(t, errs, BulkMutatingRequestLimit)
}

// The bulk limit changes only how many writes may be in flight; retry and
// backoff stay exactly as for ordinary writes: a 429 is replayed and honors
// Retry-After, while an ambiguous failure is never replayed.
func TestClientBulkMutatingRequestRetryPolicyUnchanged(t *testing.T) {
	t.Run("rate limit is retried with Retry-After", func(t *testing.T) {
		setFastRetryEnv(t, "3")
		t.Setenv("ASC_MAX_DELAY", "5s")

		var attempts atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if attempts.Add(1) == 1 {
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = io.WriteString(w, `{"errors":[{"status":"429","code":"RATE_LIMIT_EXCEEDED","title":"Too many requests"}]}`)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"data":{"id":"ta-1"}}`)
		}))
		t.Cleanup(server.Close)
		client := newMutationRetryTestClient(t, server.Client())

		start := time.Now()
		_, err := client.do(WithBulkMutatingRequestLimit(context.Background()), http.MethodPatch, server.URL+"/v1/territoryAvailabilities/ta-1", strings.NewReader(`{}`))
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("do() error: %v", err)
		}
		if got := attempts.Load(); got != 2 {
			t.Fatalf("attempts = %d, want 2", got)
		}
		if elapsed < 900*time.Millisecond {
			t.Fatalf("expected Retry-After: 1 to be honored, retried after %s", elapsed)
		}
	})

	t.Run("ambiguous failure is not retried", func(t *testing.T) {
		setFastRetryEnv(t, "3")

		var attempts atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attempts.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"errors":[{"code":"UNEXPECTED_ERROR","title":"Server error"}]}`)
		}))
		t.Cleanup(server.Close)
		client := newMutationRetryTestClient(t, server.Client())

		_, err := client.do(WithBulkMutatingRequestLimit(context.Background()), http.MethodPatch, server.URL+"/v1/territoryAvailabilities/ta-1", strings.NewReader(`{}`))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if got := attempts.Load(); got != 1 {
			t.Fatalf("attempts = %d, want 1", got)
		}
	})
}
