package web

import (
	"sync"
	"testing"
	"time"
)

// An implausible record must never pause automatic sign-in for longer than
// MaxSigninBackoff after the failure it records.
func TestSigninBackoffActiveUntilCannotExceedCap(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name       string
		backoff    SigninBackoff
		wantActive bool
		wantUntil  time.Time
	}{
		{
			name:       "far-future deadline is capped from the failure",
			backoff:    SigninBackoff{FailedAt: now.Add(-30 * time.Minute), NotBefore: now.Add(1000 * time.Hour)},
			wantActive: true,
			wantUntil:  now.Add(30 * time.Minute),
		},
		{
			name:    "capped deadline already passed",
			backoff: SigninBackoff{FailedAt: now.Add(-2 * time.Hour), NotBefore: now.Add(1000 * time.Hour)},
		},
		{
			name:    "failure recorded in the future",
			backoff: SigninBackoff{FailedAt: now.Add(48 * time.Hour), NotBefore: now.Add(49 * time.Hour)},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			until, active := tc.backoff.ActiveUntil(now)
			if active != tc.wantActive || (tc.wantActive && !until.Equal(tc.wantUntil)) {
				t.Fatalf("ActiveUntil = (%s, %v), want (%s, %v)", until, active, tc.wantUntil, tc.wantActive)
			}
		})
	}
}

// Concurrent asc processes finish sign-in attempts in any order. Whatever the
// order, the record must reflect the newest outcome Apple gave: an older 5xx
// written late must not reinstate a cooldown a newer success cleared, and an
// older success must not clear a newer 5xx.
func TestSigninBackoffKeepsNewestOutcomeRegardlessOfWriteOrder(t *testing.T) {
	start := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	failure := &SigninServiceError{Stage: "signin_init", Status: 503}
	for _, newestIsFailure := range []bool{false, true} {
		t.Run(map[bool]string{false: "success newest", true: "failure newest"}[newestIsFailure], func(t *testing.T) {
			t.Setenv(webSessionCacheDirEnv, t.TempDir())
			const outcomes = 16
			// Every write lands after all outcomes were observed, on one clock.
			writtenAt := start.Add(outcomes * time.Second)
			var wg sync.WaitGroup
			for i := range outcomes {
				at := start.Add(time.Duration(i) * time.Second)
				// Alternate outcomes; the newest (odd) index is the requested kind.
				isFailure := (i%2 == 1) == newestIsFailure
				wg.Go(func() {
					var err error
					if isFailure {
						_, err = RecordSigninBackoff("user@example.com", NewSigninBackoff(failure, at), writtenAt)
					} else {
						err = ClearSigninBackoff("user@example.com", at, writtenAt)
					}
					if err != nil {
						t.Errorf("outcome at %s: %v", at, err)
					}
				})
			}
			wg.Wait()

			backoff, _, err := LoadSigninBackoff("user@example.com")
			if err != nil {
				t.Fatal(err)
			}
			newest := start.Add((outcomes - 1) * time.Second)
			if _, active := backoff.ActiveUntil(newest); active != newestIsFailure {
				t.Fatalf("cooldown active = %v after newest outcome (failure=%v); record %+v", active, newestIsFailure, backoff)
			}
		})
	}
}

// A failure stamped by a clock that later moved backward must not outrank a
// success observed afterwards, or the stale cooldown would reactivate once the
// clock catches up.
func TestClearSigninBackoffOverridesFutureDatedFailure(t *testing.T) {
	t.Setenv(webSessionCacheDirEnv, t.TempDir())
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	future := now.Add(30 * time.Minute)
	if _, err := RecordSigninBackoff("user@example.com", NewSigninBackoff(&SigninServiceError{Stage: "signin_init", Status: 503}, future), future); err != nil {
		t.Fatal(err)
	}
	if err := ClearSigninBackoff("user@example.com", now, now); err != nil {
		t.Fatal(err)
	}
	backoff, _, err := LoadSigninBackoff("user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, active := backoff.ActiveUntil(future.Add(time.Minute)); active {
		t.Fatalf("future-dated failure survived a later success: %+v", backoff)
	}
}
