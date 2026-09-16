package cmdtest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// webAppleIDEnvNameForTest names the Apple ID environment fallback without
// spelling the variable out, matching the password helper above it.
func webAppleIDEnvNameForTest() string {
	return strings.Join([]string{"ASC", "WEB", "APPLE", "ID"}, "_")
}

// writeCachedWebSessionFile lays out a file-backed cache entry without a
// last-session pointer, the state left behind after logging out of the
// most recently used account while another account stays cached.
func writeCachedWebSessionFile(t *testing.T, dir, email string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	raw, err := json.Marshal(map[string]any{
		"version":    1,
		"user_email": email,
		"cookies": map[string]any{
			"https://appstoreconnect.apple.com": []map[string]any{{"name": "myacinfo", "value": "cookie-" + email}},
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "session-"+hex.EncodeToString(sum[:])+".json"), raw, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func runWebCommandForAppleIDDefault(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	return stderr, runErr
}

func TestWebCommandsRequireAppleIDWithLoginHintWhenNothingIsCached(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "review show", args: []string{"web", "review", "show", "--app", "123456789"}},
		{name: "auth capabilities", args: []string{"web", "auth", "capabilities", "--key-id", "KEY123"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ASC_WEB_SESSION_CACHE_BACKEND", "file")
			t.Setenv("ASC_WEB_SESSION_CACHE_DIR", t.TempDir())
			t.Setenv("ASC_WEB_SESSION_CACHE", "1")
			t.Setenv(webPasswordEnvNameForTest(), "")

			stderr, runErr := runWebCommandForAppleIDDefault(t, tc.args...)
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp (exit 2), got %v", runErr)
			}
			want := "Error: --apple-id is required when no cached web session is available; run 'asc web auth login --apple-id EMAIL'\n"
			if !strings.Contains(stderr, want) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr, want)
			}
			if strings.Contains(stderr, "Using cached web session") {
				t.Fatalf("stderr = %q, did not expect a cached-session notice", stderr)
			}
			if got := strings.Count(stderr, "Error: "); got != 1 {
				t.Fatalf("stderr = %q, want exactly one diagnostic, got %d", stderr, got)
			}
		})
	}
}

func TestWebCommandsRequireAppleIDWhenMultipleSessionsAreCached(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "review show", args: []string{"web", "review", "show", "--app", "123456789"}},
		{name: "auth capabilities", args: []string{"web", "auth", "capabilities", "--key-id", "KEY123"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("ASC_WEB_SESSION_CACHE_BACKEND", "file")
			t.Setenv("ASC_WEB_SESSION_CACHE_DIR", dir)
			t.Setenv("ASC_WEB_SESSION_CACHE", "1")
			t.Setenv(webPasswordEnvNameForTest(), "")
			writeCachedWebSessionFile(t, dir, "zed@example.com")
			writeCachedWebSessionFile(t, dir, "amy@example.com")

			stderr, runErr := runWebCommandForAppleIDDefault(t, tc.args...)
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp (exit 2), got %v", runErr)
			}
			want := "Error: --apple-id is required: multiple cached web sessions are available (amy@example.com, zed@example.com); pass --apple-id to choose one\n"
			if !strings.Contains(stderr, want) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr, want)
			}
			if got := strings.Count(stderr, "Error: "); got != 1 {
				t.Fatalf("stderr = %q, want exactly one diagnostic, got %d", stderr, got)
			}
		})
	}
}
