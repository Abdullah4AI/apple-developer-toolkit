//go:build !windows

package web

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The cooldown record names when an Apple Account last failed to sign in, so
// it must stay private to the user and live in the configured cache directory.
func TestRecordSigninBackoffWritesPrivateFileInSessionCacheDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	t.Setenv(webSessionCacheDirEnv, dir)
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	backoff := NewSigninBackoff(&SigninServiceError{Stage: "signin_init", Status: 503}, now)

	if recorded, err := RecordSigninBackoff("User@Example.com", backoff, now); err != nil || !recorded {
		t.Fatalf("RecordSigninBackoff = (%v, %v)", recorded, err)
	}
	info, err := os.Stat(filepath.Join(dir, "signin-backoff-"+webSessionCacheKey("user@example.com")+".json"))
	if err != nil {
		t.Fatalf("stat record: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("record permissions = %o, want 600", perm)
	}
	loaded, ok, err := LoadSigninBackoff("user@example.com")
	if err != nil || !ok || !loaded.NotBefore.Equal(now.Add(DefaultSigninBackoff)) {
		t.Fatalf("LoadSigninBackoff = (%+v, %v, %v), want the recorded cooldown", loaded, ok, err)
	}
}
