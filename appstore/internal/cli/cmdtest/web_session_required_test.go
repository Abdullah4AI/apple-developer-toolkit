package cmdtest

import (
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
)

// isolateEmptyWebSessionCache points the web session cache at an empty
// directory and removes every non-interactive sign-in input, so a command that
// needs a session has none to resume and no way to create one.
func isolateEmptyWebSessionCache(t *testing.T) {
	t.Helper()
	t.Setenv("ASC_WEB_SESSION_CACHE_BACKEND", "file")
	t.Setenv("ASC_WEB_SESSION_CACHE_DIR", t.TempDir())
	t.Setenv("ASC_WEB_SESSION_CACHE", "1")
	t.Setenv(webPasswordEnvNameForTest(), "")
	t.Setenv("ASC_WEB_DONT_STORE_PASSWORD", "1")
	t.Setenv("ASC_WEB_2FA_CODE_COMMAND", "")
}

func runASCForExitCode(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run(args, "1.2.3")
	})
	return code, stdout, stderr
}

const webSessionRequiredHint = "Hint: asc web commands need a signed-in Apple Account session, and signing in needs an interactive terminal for the password and two-factor code. Run 'asc web auth login --apple-id EMAIL' in a terminal, or load a session exported elsewhere with 'asc web auth import --file FILE'. Unattended sign-in needs ASC_WEB_PASSWORD and ASC_WEB_2FA_CODE_COMMAND, plus --apple-id or ASC_WEB_APPLE_ID."

func TestWebCommandsWithoutCachedSessionFailFastWithoutUsagePage(t *testing.T) {
	for _, tc := range []struct {
		name       string
		args       []string
		wantPrefix string
		wantSuffix string
	}{
		{
			name:       "privacy pull",
			args:       []string{"web", "privacy", "pull", "--app", "123456789"},
			wantPrefix: "Error: no Apple web session is cached\n",
		},
		{
			name:       "agreements status",
			args:       []string{"web", "agreements", "status"},
			wantPrefix: "Error: no Apple web session is cached\n",
		},
		{
			name:       "review list names the public API alternative",
			args:       []string{"web", "review", "list", "--app", "123456789"},
			wantPrefix: "Error: no Apple web session is cached\n",
			wantSuffix: ` Without a web session, 'asc review submissions-list --app "123456789"' lists review submissions through the App Store Connect API.`,
		},
		{
			name:       "review show names the public API alternative",
			args:       []string{"web", "review", "show", "--app", "123456789"},
			wantPrefix: "Error: no Apple web session is cached\n",
			wantSuffix: ` Without a web session, 'asc review status --app "123456789"' reports App Review state through the App Store Connect API; Resolution Center messages need a web session.`,
		},
		{
			name:       "review threads names the public API alternative",
			args:       []string{"web", "review", "threads", "--app", "123456789"},
			wantPrefix: "Error: no Apple web session is cached\n",
			wantSuffix: ` Without a web session, 'asc review status --app "123456789"' reports App Review state through the App Store Connect API; Resolution Center messages need a web session.`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateEmptyWebSessionCache(t)

			code, stdout, stderr := runASCForExitCode(t, tc.args...)
			if code != cmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (usage); stderr=%q", code, cmd.ExitUsage, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			want := tc.wantPrefix + webSessionRequiredHint + tc.wantSuffix + "\n"
			if stderr != want {
				t.Fatalf("stderr =\n%q\nwant\n%q", stderr, want)
			}
		})
	}
}

func TestWebCommandWithUnusableSessionForNamedAccountFailsFastWithAuthExitCode(t *testing.T) {
	isolateEmptyWebSessionCache(t)

	code, stdout, stderr := runASCForExitCode(t, "web", "privacy", "pull", "--app", "123456789", "--apple-id", "user@example.com")
	if code != cmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d (usage); stderr=%q", code, cmd.ExitUsage, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	want := "Error: no usable Apple web session for user@example.com\n" + webSessionRequiredHint + "\n"
	if stderr != want {
		t.Fatalf("stderr =\n%q\nwant\n%q", stderr, want)
	}
}

func TestWebAppsCreateWithoutCachedSessionOrTerminalFailsWithoutUsagePage(t *testing.T) {
	isolateEmptyWebSessionCache(t)

	code, stdout, stderr := runASCForExitCode(
		t,
		"web", "apps", "create",
		"--name", "My App",
		"--bundle-id", "com.example.app",
		"--sku", "SKU123",
	)
	if code != cmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d (usage); stderr=%q", code, cmd.ExitUsage, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	// The command echoes the app it would create before resolving a session.
	if want := "\nError: no Apple web session is cached\n" + webSessionRequiredHint + "\n"; !strings.HasSuffix(stderr, want) {
		t.Fatalf("stderr =\n%q\nwant suffix\n%q", stderr, want)
	}
	if strings.Contains(stderr, "USAGE") {
		t.Fatalf("stderr = %q, did not expect the usage page", stderr)
	}
}

// TestWebAuthLoginKeepsUsageErrorsForMissingSignInInput pins the sign-in
// command's own contract: it exists to create the session, so a missing
// Apple Account or password is still a usage error (exit 2) for it.
func TestWebAuthLoginKeepsUsageErrorsForMissingSignInInput(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{
			name: "no account",
			args: []string{"web", "auth", "login"},
			want: "Error: --apple-id is required when no cached web session is available; run 'asc web auth login --apple-id EMAIL'\n",
		},
		{
			name: "no password",
			args: []string{"web", "auth", "login", "--apple-id", "user@example.com"},
			want: "Error: password is required: run in a terminal for an interactive prompt or set ASC_WEB_PASSWORD\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateEmptyWebSessionCache(t)

			code, _, stderr := runASCForExitCode(t, tc.args...)
			if code != cmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (usage); stderr=%q", code, cmd.ExitUsage, stderr)
			}
			if !strings.HasPrefix(stderr, tc.want) {
				t.Fatalf("stderr = %q, want prefix %q", stderr, tc.want)
			}
		})
	}
}
