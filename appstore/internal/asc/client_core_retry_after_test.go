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
	"testing/synctest"
	"time"
)

func rateLimitedError(retryAfter time.Duration) error {
	return &RetryableError{
		Err:        buildRetryableError(http.StatusTooManyRequests, retryAfter, nil),
		RetryAfter: retryAfter,
	}
}

func retryableHTTPError(statusCode int, retryAfter time.Duration) error {
	return &RetryableError{
		Err:        buildRetryableError(statusCode, retryAfter, nil),
		RetryAfter: retryAfter,
	}
}

func TestParseRetryAfterHeaderHandlesNumericDateAndBoundaryValues(t *testing.T) {
	const maxRetryAfterDuration = time.Duration(1<<63 - 1)

	tests := []struct {
		name  string
		value string
		check func(t *testing.T, got time.Duration)
	}{
		{
			name:  "numeric seconds",
			value: "7",
			check: func(t *testing.T, got time.Duration) {
				if got != 7*time.Second {
					t.Fatalf("parseRetryAfterHeader() = %s, want 7s", got)
				}
			},
		},
		{
			name:  "zero falls back",
			value: "0",
			check: func(t *testing.T, got time.Duration) {
				if got != 0 {
					t.Fatalf("parseRetryAfterHeader() = %s, want 0", got)
				}
			},
		},
		{
			name:  "negative falls back",
			value: "-7",
			check: func(t *testing.T, got time.Duration) {
				if got != 0 {
					t.Fatalf("parseRetryAfterHeader() = %s, want 0", got)
				}
			},
		},
		{
			name:  "huge numeric saturates",
			value: "9223372036854775807",
			check: func(t *testing.T, got time.Duration) {
				if got != maxRetryAfterDuration {
					t.Fatalf("parseRetryAfterHeader() = %s, want %s", got, maxRetryAfterDuration)
				}
			},
		},
		{
			name:  "unsigned range numeric saturates",
			value: "9223372036854775808",
			check: func(t *testing.T, got time.Duration) {
				if got != maxRetryAfterDuration {
					t.Fatalf("parseRetryAfterHeader() = %s, want %s", got, maxRetryAfterDuration)
				}
			},
		},
		{
			name:  "unsigned range numeric with plus saturates",
			value: "+9223372036854775808",
			check: func(t *testing.T, got time.Duration) {
				if got != maxRetryAfterDuration {
					t.Fatalf("parseRetryAfterHeader() = %s, want %s", got, maxRetryAfterDuration)
				}
			},
		},
		{
			name:  "uint64 maximum numeric saturates",
			value: "18446744073709551615",
			check: func(t *testing.T, got time.Duration) {
				if got != maxRetryAfterDuration {
					t.Fatalf("parseRetryAfterHeader() = %s, want %s", got, maxRetryAfterDuration)
				}
			},
		},
		{
			name:  "overflow with trailing text is malformed",
			value: "18446744073709551616x",
			check: func(t *testing.T, got time.Duration) {
				if got != 0 {
					t.Fatalf("parseRetryAfterHeader() = %s, want 0", got)
				}
			},
		},
		{
			name:  "future http date",
			value: time.Now().UTC().Add(2 * time.Second).Format(http.TimeFormat),
			check: func(t *testing.T, got time.Duration) {
				if got < 500*time.Millisecond || got > 3*time.Second {
					t.Fatalf("parseRetryAfterHeader() = %s, want roughly 2s", got)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, parseRetryAfterHeader(tt.value))
		})
	}
}

// A Retry-After Apple can actually satisfy is waited out exactly, not replaced
// by the shorter exponential backoff.
func TestWithRetry_HonorsRetryAfterWithinCap(t *testing.T) {
	var attempts atomic.Int32

	var err error
	var elapsed time.Duration
	synctest.Test(t, func(*testing.T) {
		start := time.Now()
		_, err = WithRetry(context.Background(), func() (struct{}, error) {
			if attempts.Add(1) == 1 {
				return struct{}{}, rateLimitedError(60 * time.Millisecond)
			}
			return struct{}{}, nil
		}, RetryOptions{MaxRetries: 2, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Second})
		elapsed = time.Since(start)
	})

	if err != nil {
		t.Fatalf("WithRetry() error: %v", err)
	}
	if got := attempts.Load(); got != 2 {
		t.Fatalf("expected 2 attempts, got %d", got)
	}
	if elapsed < 60*time.Millisecond {
		t.Fatalf("expected the 60ms Retry-After to be honored, retried after %s", elapsed)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("expected the wait to track Retry-After, waited %s", elapsed)
	}
}

// A Retry-After beyond the configured cap cannot be honored, and sleeping the
// capped amount only collects the same rejection again. Report the numbers
// instead of stalling.
func TestWithRetry_FailsFastWhenRetryAfterExceedsCap(t *testing.T) {
	var attempts atomic.Int32

	var err error
	var elapsed time.Duration
	synctest.Test(t, func(*testing.T) {
		// Bound the test: a regression sleeps the server's hint, and this turns that
		// into a failure instead of an hour-long hang.
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		start := time.Now()
		_, err = WithRetry(ctx, func() (struct{}, error) {
			attempts.Add(1)
			return struct{}{}, rateLimitedError(time.Hour)
		}, RetryOptions{MaxRetries: 3, BaseDelay: time.Millisecond, MaxDelay: 30 * time.Second})
		elapsed = time.Since(start)
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("expected exactly 1 attempt, got %d", got)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("expected an immediate failure, took %s", elapsed)
	}

	message := err.Error()
	if !strings.Contains(message, "1h0m0s") {
		t.Fatalf("expected the requested wait in the message, got %q", message)
	}
	if !strings.Contains(message, "30s") {
		t.Fatalf("expected the retry cap in the message, got %q", message)
	}
	if !strings.Contains(message, "rate limited") {
		t.Fatalf("expected the message to name rate limiting, got %q", message)
	}
	if !strings.Contains(message, "context deadline") {
		t.Fatalf("expected the message to name the context deadline, got %q", message)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("did not expect context deadline classification before the parent context expires, got %v", err)
	}

	apiErr, ok := errors.AsType[*APIError](err)
	if !ok {
		t.Fatalf("expected *APIError in chain, got %v", err)
	}
	if apiErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, apiErr.StatusCode)
	}
}

func TestWithRetry_FailsFastWhenRetryAfterExceedsContextBudget(t *testing.T) {
	var attempts atomic.Int32

	var err error
	var elapsed time.Duration
	synctest.Test(t, func(*testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		start := time.Now()
		_, err = WithRetry(ctx, func() (struct{}, error) {
			attempts.Add(1)
			return struct{}{}, rateLimitedError(5 * time.Second)
		}, RetryOptions{MaxRetries: 3, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Second})
		elapsed = time.Since(start)
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("expected exactly 1 attempt, got %d", got)
	}
	if elapsed > 250*time.Millisecond {
		t.Fatalf("expected an immediate failure, took %s", elapsed)
	}
	if message := err.Error(); !strings.Contains(message, "context deadline") || !strings.Contains(message, "5s") {
		t.Fatalf("expected the requested wait and context deadline in the message, got %q", message)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("did not expect context deadline classification before the parent context expires, got %v", err)
	}

	apiErr, ok := errors.AsType[*APIError](err)
	if !ok {
		t.Fatalf("expected *APIError in chain, got %v", err)
	}
	if apiErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, apiErr.StatusCode)
	}
}

func TestWithRetry_FailsFastWhenOptedInFallbackExceedsContextBudget(t *testing.T) {
	var attempts atomic.Int32

	var err error
	var elapsed time.Duration
	synctest.Test(t, func(*testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		start := time.Now()
		_, err = WithRetry(ctx, func() (struct{}, error) {
			attempts.Add(1)
			return struct{}{}, &RetryableError{
				Err:                     buildRetryableError(http.StatusTooManyRequests, 0, nil),
				PreserveErrorOnDeadline: true,
			}
		}, RetryOptions{MaxRetries: 3, BaseDelay: time.Second, MaxDelay: time.Second})
		elapsed = time.Since(start)
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("expected exactly 1 attempt, got %d", got)
	}
	if elapsed >= 250*time.Millisecond {
		t.Fatalf("elapsed = %s, want an immediate fallback deadline failure", elapsed)
	}
	if !IsRetryDelayExceeded(err) {
		t.Fatalf("expected retry-delay-exceeded marker, got %v", err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("did not expect context deadline classification before the parent context expires, got %v", err)
	}
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok {
		t.Fatalf("expected *APIError in chain, got %v", err)
	}
	if apiErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, apiErr.StatusCode)
	}
}

func TestWithRetry_ExplicitCancellationWinsWhenOptedInFallbackExceedsContextBudget(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	cancel()

	_, err := WithRetry(ctx, func() (struct{}, error) {
		return struct{}{}, &RetryableError{
			Err:                     errors.New("retryable failure"),
			PreserveErrorOnDeadline: true,
		}
	}, RetryOptions{MaxRetries: 1, BaseDelay: time.Second, MaxDelay: time.Second})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want explicit context cancellation", err)
	}
	if IsRetryDelayExceeded(err) {
		t.Fatalf("did not expect fallback deadline classification to hide cancellation, got %v", err)
	}
}

func TestWithRetry_FinalFallbackAttemptPreservesExplicitCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var attempts atomic.Int32
	_, err := WithRetry(ctx, func() (struct{}, error) {
		if attempts.Add(1) == 2 {
			cancel()
		}
		return struct{}{}, &RetryableError{
			Err:                     buildRetryableError(http.StatusTooManyRequests, 0, nil),
			PreserveErrorOnDeadline: true,
		}
	}, RetryOptions{MaxRetries: 1, BaseDelay: time.Millisecond, MaxDelay: time.Second})

	if got := attempts.Load(); got != 2 {
		t.Fatalf("expected the final allowed attempt, got %d attempts", got)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want explicit context cancellation", err)
	}
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok {
		t.Fatalf("expected *APIError in chain, got %v", err)
	}
	if apiErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, apiErr.StatusCode)
	}
}

func TestWithRetry_FinalFallbackAttemptRetainsBudgetWithoutCancellation(t *testing.T) {
	var attempts atomic.Int32
	_, err := WithRetry(context.Background(), func() (struct{}, error) {
		attempts.Add(1)
		return struct{}{}, &RetryableError{
			Err:                     buildRetryableError(http.StatusTooManyRequests, 0, nil),
			PreserveErrorOnDeadline: true,
		}
	}, RetryOptions{MaxRetries: 1, BaseDelay: time.Millisecond, MaxDelay: time.Second})

	if got := attempts.Load(); got != 2 {
		t.Fatalf("expected the final allowed attempt, got %d attempts", got)
	}
	if !IsRetryBudgetExhausted(err) {
		t.Fatalf("expected retry budget exhaustion, got %v", err)
	}
	if errors.Is(err, context.Canceled) {
		t.Fatalf("did not expect cancellation without a canceled context, got %v", err)
	}
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok {
		t.Fatalf("expected *APIError in chain, got %v", err)
	}
	if apiErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, apiErr.StatusCode)
	}
}

func TestWithRetry_ClassifiesOverCapHintBeforeFinalRetryBudget(t *testing.T) {
	var attempts atomic.Int32

	_, err := WithRetry(context.Background(), func() (struct{}, error) {
		if attempts.Add(1) == 1 {
			return struct{}{}, retryableHTTPError(http.StatusServiceUnavailable, 0)
		}
		return struct{}{}, rateLimitedError(time.Hour)
	}, RetryOptions{MaxRetries: 1, BaseDelay: time.Millisecond, MaxDelay: time.Second})
	if err == nil {
		t.Fatal("expected over-cap Retry-After error, got nil")
	}
	if got := attempts.Load(); got != 2 {
		t.Fatalf("expected final allowed attempt to be made, got %d attempts", got)
	}
	if !strings.Contains(err.Error(), "retry cap") {
		t.Fatalf("expected retry-cap diagnostic, got %v", err)
	}
	if IsRetryBudgetExhausted(err) {
		t.Fatalf("did not expect retry budget marker to hide over-cap diagnostic, got %v", err)
	}
	if IsTransientWaitError(context.Background(), err) {
		t.Fatalf("expected over-cap final attempt to be terminal to poll recovery, got %v", err)
	}
}

func TestWithRetry_CancellationPreservesRetryableCause(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var attempts atomic.Int32
	_, err := WithRetry(ctx, func() (struct{}, error) {
		attempts.Add(1)
		cancel()
		return struct{}{}, rateLimitedError(time.Second)
	}, RetryOptions{MaxRetries: 3, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Second})

	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("expected exactly one attempt after cancellation, got %d", got)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation in error chain, got %v", err)
	}
	if !IsRetryable(err) {
		t.Fatalf("expected the original 429 to remain retryable, got %v", err)
	}
	if IsRetryBudgetExhausted(err) {
		t.Fatalf("did not expect retry budget exhaustion before a retry, got %v", err)
	}
	if got := GetRetryAfter(err); got != time.Second {
		t.Fatalf("expected Retry-After to be preserved, got %s", got)
	}
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok {
		t.Fatalf("expected *APIError in chain, got %v", err)
	}
	if apiErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, apiErr.StatusCode)
	}
}

func TestWithRetry_OverCapNonRateLimitUsesStatusNeutralMessage(t *testing.T) {
	var attempts atomic.Int32

	_, err := WithRetry(context.Background(), func() (struct{}, error) {
		attempts.Add(1)
		return struct{}{}, retryableHTTPError(http.StatusServiceUnavailable, time.Hour)
	}, RetryOptions{MaxRetries: 3, BaseDelay: time.Millisecond, MaxDelay: 30 * time.Second})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("expected exactly 1 attempt, got %d", got)
	}
	message := err.Error()
	if strings.Contains(message, "rate limited") {
		t.Fatalf("expected status-neutral retry-cap wording, got %q", message)
	}
	if !strings.Contains(message, "retry cap") || !strings.Contains(message, "service unavailable") {
		t.Fatalf("expected the cap and wrapped service error in the message, got %q", message)
	}
}

func TestWithRetry_RetryAfterAtCapIsHonored(t *testing.T) {
	var attempts atomic.Int32

	_, err := WithRetry(context.Background(), func() (struct{}, error) {
		if attempts.Add(1) == 1 {
			return struct{}{}, rateLimitedError(20 * time.Millisecond)
		}
		return struct{}{}, nil
	}, RetryOptions{MaxRetries: 2, BaseDelay: time.Millisecond, MaxDelay: 20 * time.Millisecond})
	if err != nil {
		t.Fatalf("WithRetry() error: %v", err)
	}
	if got := attempts.Load(); got != 2 {
		t.Fatalf("expected 2 attempts, got %d", got)
	}
}

// Without a cap check the client sleeps the server's full hint and the command
// ends on its context deadline, never telling the operator it was rate limited.
func TestClientDo_RateLimitBeyondCapFailsWithoutWaiting(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "3")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "2s")
	resetConfigCacheForTest()
	t.Cleanup(resetConfigCacheForTest)

	var attempts atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"errors":[{"status":"429","code":"RATE_LIMIT_EXCEEDED","title":"Too many requests"}]}`)
	})

	client := &Client{
		httpClient: &http.Client{Transport: inProcessTransport(handler)},
		keyID:      "KEY123",
		issuerID:   "ISS456",
		privateKey: testJWTPrivateKey(t),
	}

	var err error
	var elapsed time.Duration
	synctest.Test(t, func(*testing.T) {
		// Bound the test the same way: without the cap check this sleeps the full
		// hour the server asked for.
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		start := time.Now()
		_, err = client.do(ctx, http.MethodGet, "https://api.appstoreconnect.apple.com/v1/apps", nil)
		elapsed = time.Since(start)
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("expected exactly 1 request, got %d", got)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("expected an immediate failure, took %s", elapsed)
	}
	if message := err.Error(); !strings.Contains(message, "1h0m0s") || !strings.Contains(message, "2s") {
		t.Fatalf("expected the requested wait and the cap in the message, got %q", message)
	}
}

func TestClientDo_RateLimitWithHugeNumericRetryAfterFailsWithoutRetry(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "3")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "2s")
	resetConfigCacheForTest()
	t.Cleanup(resetConfigCacheForTest)

	const hugeRetryAfter = "9223372036854775807"
	var attempts atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.Header().Set("Retry-After", hugeRetryAfter)
		w.WriteHeader(http.StatusTooManyRequests)
	})

	client := &Client{
		httpClient: &http.Client{Transport: inProcessTransport(handler)},
		keyID:      "KEY123",
		issuerID:   "ISS456",
		privateKey: testJWTPrivateKey(t),
	}

	var err error
	var elapsed time.Duration
	synctest.Test(t, func(*testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		start := time.Now()
		_, err = client.do(ctx, http.MethodGet, "https://api.appstoreconnect.apple.com/v1/apps", nil)
		elapsed = time.Since(start)
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("expected exactly 1 request, got %d (Retry-After overflow was ignored)", got)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("expected an immediate failure, took %s", elapsed)
	}
	if !strings.Contains(err.Error(), "retry cap") {
		t.Fatalf("expected retry-cap error, got %q", err)
	}
}

func inProcessTransport(handler http.Handler) http.RoundTripper {
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		if err := req.Context().Err(); err != nil {
			return nil, err
		}
		return recorder.Result(), nil
	})
}
