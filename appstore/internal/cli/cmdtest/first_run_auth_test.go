package cmdtest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
)

func clearAuthEnvWithConfig(t *testing.T) string {
	t.Helper()
	home := setCmdtestHome(t)
	for _, name := range []string{"ASC_PROFILE", "ASC_KEY_ID", "ASC_ISSUER_ID", "ASC_PRIVATE_KEY_PATH", "ASC_PRIVATE_KEY", "ASC_PRIVATE_KEY_B64"} {
		t.Setenv(name, "")
	}
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	configPath := filepath.Join(home, "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	return configPath
}

func TestLocalCredentialFailuresAreMissingAuth(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T) []string
		want  func(configPath string) string
	}{
		{
			name: "malformed config",
			setup: func(t *testing.T) []string {
				configPath := clearAuthEnvWithConfig(t)
				if err := os.WriteFile(configPath, []byte("bad"), 0o600); err != nil {
					t.Fatalf("write config: %v", err)
				}
				return []string{"apps", "list"}
			},
			want: func(configPath string) string { return "failed to parse config " + configPath + ": invalid character" },
		},
		{
			name: "profile without config",
			setup: func(t *testing.T) []string {
				clearAuthEnvWithConfig(t)
				return []string{"--profile", "ghost", "apps", "list"}
			},
			want: func(configPath string) string {
				return `credentials not found for profile "ghost"; no profiles are configured (config file: ` + configPath + ")"
			},
		},
		{
			name: "unknown profile",
			setup: func(t *testing.T) []string {
				writeProfileFixtureConfig(t)
				return []string{"--profile", "ghost", "apps", "list"}
			},
			want: func(string) string {
				return `credentials not found for profile "ghost"; available profiles: client, prod, staging`
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := test.setup(t)
			var code int
			stdout, stderr := captureOutput(t, func() { code = rootcmd.Run(args, "1.2.3") })
			if code != rootcmd.ExitAuth {
				t.Fatalf("exit code = %d, want %d; stderr = %q", code, rootcmd.ExitAuth, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if want := test.want(os.Getenv("ASC_CONFIG_PATH")); !strings.Contains(stderr, want) {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
			if !strings.Contains(stderr, "Hint: Run `asc auth status`") {
				t.Fatalf("stderr = %q, want the auth status hint", stderr)
			}
		})
	}
}

func TestAuthLoginMissingPrivateKeyNamesGivenPath(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "unexpanded tilde",
			args: []string{"--private-key", "~/AuthKey.p8"},
			want: `private key file not found: "~/AuthKey.p8" (the shell did not expand ~; use an absolute path)`,
		},
		{
			name: "fix permissions",
			args: []string{"--fix-permissions", "--private-key", "/nonexistent/AuthKey.p8"},
			want: `private key file not found: "/nonexistent/AuthKey.p8"`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearAuthEnvWithConfig(t)
			args := append([]string{"auth", "login", "--bypass-keychain", "--name", "demo", "--key-id", "ABC123DEFG", "--issuer-id", "00000000-0000-0000-0000-000000000000"}, test.args...)
			var code int
			_, stderr := captureOutput(t, func() { code = rootcmd.Run(args, "1.2.3") })
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d; stderr = %q", code, rootcmd.ExitUsage, stderr)
			}
			if !strings.Contains(stderr, "Error: auth login: invalid private key: "+test.want+"\n") {
				t.Fatalf("stderr = %q, want %q", stderr, test.want)
			}
		})
	}
}

func TestAPIPathWithoutMethodSuggestsGET(t *testing.T) {
	clearAuthEnvWithConfig(t)
	var code int
	_, stderr := captureOutput(t, func() { code = rootcmd.Run([]string{"api", "/v1/apps"}, "1.2.3") })
	if code != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
	}
	if !strings.Contains(stderr, "Error: api: METHOD and PATH are required\nHint: pass the method first: asc api GET /v1/apps\n") {
		t.Fatalf("stderr = %q, want a GET suggestion", stderr)
	}
}
