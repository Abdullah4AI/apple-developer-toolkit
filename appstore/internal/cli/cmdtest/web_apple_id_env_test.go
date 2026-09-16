package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

// TestWebCommandsSelectAppleIDFromEnvironmentOverAmbiguousCache pins the
// ASC_WEB_APPLE_ID fallback at the command level. Two cached accounts make the
// cached-session default ambiguous, so without the variable this invocation
// ends in the "pass --apple-id to choose one" usage error. The environment
// account has no cached session and no password source, so resolution stops at
// the password usage error instead of attempting a live login.
func TestWebCommandsSelectAppleIDFromEnvironmentOverAmbiguousCache(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_WEB_SESSION_CACHE_BACKEND", "file")
	t.Setenv("ASC_WEB_SESSION_CACHE_DIR", dir)
	t.Setenv("ASC_WEB_SESSION_CACHE", "1")
	t.Setenv(webPasswordEnvNameForTest(), "")
	t.Setenv("ASC_WEB_DONT_STORE_PASSWORD", "1")
	t.Setenv(webAppleIDEnvNameForTest(), "  env@example.com  ")
	writeCachedWebSessionFile(t, dir, "zed@example.com")
	writeCachedWebSessionFile(t, dir, "amy@example.com")

	stderr, runErr := runWebCommandForAppleIDDefault(t, "web", "review", "show", "--app", "123456789")
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected usage error (exit 2), got %v", runErr)
	}
	// The stderr notice naming the source is written through the session
	// diagnostics writer bound at package init, which this harness cannot
	// capture; the resolver unit tests assert its text. Reaching the password
	// usage error for an uncached account is what proves the variable chose it.
	if !strings.Contains(stderr, "password is required") {
		t.Fatalf("stderr = %q, want the password usage error for the environment account", stderr)
	}
	if strings.Contains(stderr, "multiple cached web sessions are available") {
		t.Fatalf("stderr = %q, did not expect the ambiguous-cache usage error", stderr)
	}
	if strings.Contains(stderr, "--apple-id is required") {
		t.Fatalf("stderr = %q, did not expect a missing --apple-id usage error", stderr)
	}
}

// TestWebAuthStatusIgnoresAppleIDEnvironmentFallback pins the documented
// exclusion: the cache-management commands resolve the account from the flag
// and the cache alone, so the environment fallback must not select an account
// for them (asc web auth export defaults the same way, and --apple-id on
// asc web auth import asserts an account instead of selecting one).
func TestWebAuthStatusIgnoresAppleIDEnvironmentFallback(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_WEB_SESSION_CACHE_BACKEND", "file")
	t.Setenv("ASC_WEB_SESSION_CACHE_DIR", t.TempDir())
	t.Setenv("ASC_WEB_SESSION_CACHE", "1")
	t.Setenv(webPasswordEnvNameForTest(), "")
	t.Setenv(webAppleIDEnvNameForTest(), "env@example.com")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"web", "auth", "status", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, `"authenticated":false`) {
		t.Fatalf("stdout = %q, want authenticated=false", stdout)
	}
	if strings.Contains(stdout, "env@example.com") {
		t.Fatalf("stdout = %q, did not expect the environment Apple Account", stdout)
	}
}
