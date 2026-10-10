package shared

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

func TestRetryReadWithFreshTimeoutRetriesChildDeadline(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "100ms")
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")

	requests := 0
	var value string
	var err error
	synctest.Test(t, func(*testing.T) {
		value, err = RetryReadWithFreshTimeout(context.Background(), func(ctx context.Context) (string, error) {
			requests++
			if requests == 1 {
				<-ctx.Done()
				return "", ctx.Err()
			}
			if err := ctx.Err(); err != nil {
				t.Errorf("expected a fresh request context, got %v", err)
				return "", err
			}
			return "ok", nil
		})
	})
	if err != nil {
		t.Fatalf("RetryReadWithFreshTimeout() error: %v", err)
	}
	if value != "ok" || requests != 2 {
		t.Fatalf("unexpected retry result: value=%q requests=%d", value, requests)
	}
}

func TestRetryReadWithFreshTimeoutDoesNotRetryInFlightParentCancellation(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "3")

	ctx, cancel := context.WithCancel(context.Background())
	requests := 0
	_, err := RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (string, error) {
		requests++
		cancel()
		<-requestCtx.Done()
		return "", requestCtx.Err()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	if requests != 1 {
		t.Fatalf("expected one in-flight request, got %d", requests)
	}
}

func TestRetryReadWithFreshTimeoutDoesNotRetryParentCancellation(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "3")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	requests := 0
	_, err := RetryReadWithFreshTimeout(ctx, func(context.Context) (string, error) {
		requests++
		return "", context.Canceled
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	if requests != 0 {
		t.Fatalf("expected no request after parent cancellation, got %d", requests)
	}
}

func TestRetryReadWithFreshTimeoutDoesNotAmplifyRetryableErrors(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "3")

	requests := 0
	wantErr := errors.New("server unavailable")
	_, err := RetryReadWithFreshTimeout(context.Background(), func(context.Context) (string, error) {
		requests++
		return "", wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected server error, got %v", err)
	}
	if requests != 1 {
		t.Fatalf("expected one workflow read attempt, got %d", requests)
	}
}

func TestRetryReadWithFreshTimeoutDoesNotRetryBoundedRetryAfterError(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "30s")
	t.Setenv("ASC_MAX_RETRIES", "3")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1s")

	requests := 0
	_, err := RetryReadWithFreshTimeout(context.Background(), func(requestCtx context.Context) (string, error) {
		requests++
		_, retryErr := asc.WithRetry(requestCtx, func() (struct{}, error) {
			return struct{}{}, &asc.RetryableError{
				Err:        errors.New("rate limited"),
				RetryAfter: time.Hour,
			}
		}, asc.RetryOptions{MaxRetries: 1, BaseDelay: time.Millisecond, MaxDelay: time.Second})
		return "", retryErr
	})
	if err == nil {
		t.Fatal("expected bounded retry-after error, got nil")
	}
	if requests != 1 {
		t.Fatalf("expected one workflow read attempt, got %d", requests)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected bounded retry-after error not to masquerade as a deadline, got %v", err)
	}
}
