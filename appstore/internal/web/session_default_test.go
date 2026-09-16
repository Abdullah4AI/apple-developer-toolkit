package web

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/99designs/keyring"
)

func writeDefaultTestSessionFile(t *testing.T, dir, email string, version int) {
	t.Helper()
	sess := persistedSession{
		Version:   version,
		UpdatedAt: time.Now(),
		UserEmail: email,
		Cookies: map[string][]pCookie{
			"https://appstoreconnect.apple.com": {{Name: "myacinfo", Value: "cookie-" + email}},
		},
	}
	raw, err := json.Marshal(sess)
	if err != nil {
		t.Fatalf("marshal session: %v", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "session-"+webSessionCacheKey(email)+".json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write session: %v", err)
	}
}

func TestDefaultCachedAppleIDFileBackend(t *testing.T) {
	t.Run("no sessions", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv(webSessionBackendEnv, "file")
		t.Setenv(webSessionCacheDirEnv, dir)

		appleID, err := DefaultCachedAppleID()
		if !errors.Is(err, ErrNoCachedSession) {
			t.Fatalf("err = %v, want ErrNoCachedSession", err)
		}
		if appleID != "" {
			t.Fatalf("appleID = %q, want empty", appleID)
		}
	})

	t.Run("missing cache dir", func(t *testing.T) {
		t.Setenv(webSessionBackendEnv, "file")
		t.Setenv(webSessionCacheDirEnv, filepath.Join(t.TempDir(), "missing"))

		if _, err := DefaultCachedAppleID(); !errors.Is(err, ErrNoCachedSession) {
			t.Fatalf("err = %v, want ErrNoCachedSession", err)
		}
	})

	t.Run("exactly one session", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv(webSessionBackendEnv, "file")
		t.Setenv(webSessionCacheDirEnv, dir)
		writeDefaultTestSessionFile(t, dir, "Only@Example.com", webSessionCacheVersion)
		// Malformed, wrong-version, and anonymous entries never count.
		if err := os.WriteFile(filepath.Join(dir, "session-broken.json"), []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		writeDefaultTestSessionFile(t, dir, "old@example.com", webSessionCacheVersion+1)
		writeDefaultTestSessionFile(t, dir, "", webSessionCacheVersion)

		appleID, err := DefaultCachedAppleID()
		if err != nil {
			t.Fatalf("DefaultCachedAppleID() error = %v", err)
		}
		if appleID != "Only@Example.com" {
			t.Fatalf("appleID = %q, want Only@Example.com", appleID)
		}
	})

	t.Run("two sessions", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv(webSessionBackendEnv, "file")
		t.Setenv(webSessionCacheDirEnv, dir)
		writeDefaultTestSessionFile(t, dir, "zed@example.com", webSessionCacheVersion)
		writeDefaultTestSessionFile(t, dir, "amy@example.com", webSessionCacheVersion)

		appleID, err := DefaultCachedAppleID()
		var ambiguous *AmbiguousCachedSessionError
		if !errors.As(err, &ambiguous) {
			t.Fatalf("err = %v, want *AmbiguousCachedSessionError", err)
		}
		if appleID != "" {
			t.Fatalf("appleID = %q, want empty", appleID)
		}
		if want := []string{"amy@example.com", "zed@example.com"}; !reflect.DeepEqual(ambiguous.AppleIDs, want) {
			t.Fatalf("AppleIDs = %v, want %v", ambiguous.AppleIDs, want)
		}
		if got, want := err.Error(), "multiple cached web sessions are available: amy@example.com, zed@example.com"; got != want {
			t.Fatalf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("unreadable session prevents an unsafe default", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv(webSessionBackendEnv, "file")
		t.Setenv(webSessionCacheDirEnv, dir)
		writeDefaultTestSessionFile(t, dir, "only@example.com", webSessionCacheVersion)
		if err := os.WriteFile(filepath.Join(dir, "session-unreadable.json"), []byte("unreadable"), 0o600); err != nil {
			t.Fatal(err)
		}
		originalRead := readDefaultSessionFromFileFn
		readDefaultSessionFromFileFn = func(key string) (persistedSession, bool, error) {
			if key == "unreadable" {
				return persistedSession{}, false, errors.New("permission denied")
			}
			return originalRead(key)
		}
		t.Cleanup(func() { readDefaultSessionFromFileFn = originalRead })

		appleID, err := DefaultCachedAppleID()
		if err == nil {
			t.Fatalf("DefaultCachedAppleID() = %q, nil; want unreadable-cache error", appleID)
		}
		if appleID != "" {
			t.Fatalf("appleID = %q, want empty", appleID)
		}
		if !strings.Contains(err.Error(), "session-unreadable.json") {
			t.Fatalf("error = %q, want unreadable entry name", err)
		}
	})

	t.Run("cache disabled", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv(webSessionCacheEnabledEnv, "0")
		t.Setenv(webSessionCacheDirEnv, dir)
		writeDefaultTestSessionFile(t, dir, "only@example.com", webSessionCacheVersion)

		if _, err := DefaultCachedAppleID(); !errors.Is(err, ErrNoCachedSession) {
			t.Fatalf("err = %v, want ErrNoCachedSession", err)
		}
	})
}

func TestDefaultCachedAppleIDKeychainBackend(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(webSessionBackendEnv, "keychain")
	t.Setenv(webSessionCacheDirEnv, dir)
	kr := withArraySessionKeyring(t)

	store := newPersistedSessionStore()
	store.Sessions[webSessionCacheKey("kc@example.com")] = persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now(),
		UserEmail: "kc@example.com",
	}
	raw, err := json.Marshal(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := kr.Set(keyring.Item{Key: webSessionStoreItem, Data: raw}); err != nil {
		t.Fatal(err)
	}

	appleID, err := DefaultCachedAppleID()
	if err != nil {
		t.Fatalf("DefaultCachedAppleID() error = %v", err)
	}
	if appleID != "kc@example.com" {
		t.Fatalf("appleID = %q, want kc@example.com", appleID)
	}
}

func TestDefaultCachedAppleIDAutoBackendFallsBackToKeychainWhenFileCacheEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(webSessionBackendEnv, "auto")
	t.Setenv(webSessionCacheDirEnv, dir)
	kr := withArraySessionKeyring(t)

	store := newPersistedSessionStore()
	store.Sessions[webSessionCacheKey("kc@example.com")] = persistedSession{
		Version:   webSessionCacheVersion,
		UpdatedAt: time.Now(),
		UserEmail: "kc@example.com",
	}
	raw, err := json.Marshal(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := kr.Set(keyring.Item{Key: webSessionStoreItem, Data: raw}); err != nil {
		t.Fatal(err)
	}

	appleID, err := DefaultCachedAppleID()
	if err != nil {
		t.Fatalf("DefaultCachedAppleID() error = %v", err)
	}
	if appleID != "kc@example.com" {
		t.Fatalf("appleID = %q, want kc@example.com", appleID)
	}

	// A populated file cache wins without consulting the keychain again.
	writeDefaultTestSessionFile(t, dir, "file@example.com", webSessionCacheVersion)
	kr.ResetCounts()
	appleID, err = DefaultCachedAppleID()
	if err != nil {
		t.Fatalf("DefaultCachedAppleID() error = %v", err)
	}
	if appleID != "file@example.com" {
		t.Fatalf("appleID = %q, want file@example.com", appleID)
	}
	if kr.GetCount(webSessionStoreItem) != 0 {
		t.Fatalf("keychain store read %d times, want 0", kr.GetCount(webSessionStoreItem))
	}
}
