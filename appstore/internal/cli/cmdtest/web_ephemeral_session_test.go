package cmdtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	cmd "github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

type ephemeralSessionTransport func(*http.Request) (*http.Response, error)

func (f ephemeralSessionTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func ephemeralSessionFixture(t *testing.T, status int, email string) (*[]string, string) {
	t.Helper()
	cache := isolateWebSessionCache(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	t.Setenv("ASC_WEB_SESSION", webSessionBundleFixture)
	t.Setenv("ASC_WEB_SESSION_PROVIDER", "")
	t.Setenv("ASC_WEB_SESSION_CSRF", "")
	for _, name := range []string{"ASC_PROFILE", "ASC_KEY_ID", "ASC_ISSUER_ID", "ASC_PRIVATE_KEY_PATH", "ASC_PRIVATE_KEY", "ASC_PRIVATE_KEY_B64", "ASC_STRICT_AUTH"} {
		t.Setenv(name, "")
	}
	requests := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Method != http.MethodGet {
			t.Errorf("unexpected mutation: %s", r.Method)
		}
		cookie, err := r.Cookie("myacinfo")
		if err != nil || cookie.Value != "super-secret-token" {
			t.Error("request missing supplied session cookie")
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("unexpected JWT authentication")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/olympus/v1/session":
			w.WriteHeader(status)
			_, _ = fmt.Fprintf(w, `{"provider":{"providerId":42,"publicProviderId":"team-42"},"user":{"emailAddress":%q}}`, email)
		case "/iris/v1/apps":
			if r.URL.Query().Get("filter[removed]") != "true" {
				t.Error("missing removed filter")
			}
			_, _ = fmt.Fprint(w, `{"data":[{"id":"app-1","type":"apps","attributes":{"name":"Removed app","bundleId":"com.example.app"}}]}`)
		case "/iris/v1/apiKeys/key-1":
			_, _ = fmt.Fprint(w, `{"data":{"id":"key-1","type":"apiKeys","attributes":{"nickname":"Automation","isActive":true}}}`)
		case "/iris/v1/apiKeys", "/iris/v2/apiKeys":
			_, _ = fmt.Fprint(w, `{"data":[]}`)
		default:
			t.Errorf("unexpected route: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	target, _ := url.Parse(server.URL)
	transport := http.DefaultTransport
	installDefaultTransport(t, ephemeralSessionTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Scheme != "https" || r.URL.Host != "appstoreconnect.apple.com" {
			t.Fatalf("unexpected origin: %s", r.URL.Host)
		}
		clone := r.Clone(r.Context())
		clone.URL.Scheme, clone.URL.Host = target.Scheme, target.Host
		return transport.RoundTrip(clone)
	}))
	return &requests, cache
}

func TestEnvironmentSessionReadAllowlist(t *testing.T) {
	for _, tc := range []struct {
		args, paths []string
		result      string
	}{
		{[]string{"web", "removed-apps", "list"}, []string{"GET /olympus/v1/session", "GET /iris/v1/apps"}, "app-1"},
		{[]string{"web", "api-keys", "list"}, []string{"GET /olympus/v1/session", "GET /iris/v1/apiKeys", "GET /iris/v2/apiKeys"}, "[]"},
		{[]string{"web", "api-keys", "view", "--key-id", "key-1"}, []string{"GET /olympus/v1/session", "GET /iris/v1/apiKeys/key-1"}, "key-1"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			requests, cache := ephemeralSessionFixture(t, http.StatusOK, "user@example.com")
			debugFlag := cmd.RootCommand("test").FlagSet.Lookup("api-debug").Value.(*shared.OptionalBool)
			previousDebugFlag := *debugFlag
			t.Cleanup(func() {
				*debugFlag = previousDebugFlag
				shared.ApplyRootLoggingOverrides()
			})
			// An unusable cache backend must not prevent ephemeral execution.
			t.Setenv("ASC_WEB_SESSION_CACHE_BACKEND", "off")
			args := append([]string{"--api-debug"}, tc.args...)
			args = append(args, "--session-from-env", "--output", "json")
			var code int
			stdout, stderr := captureOutput(t, func() { code = cmd.Run(args, "test") })
			if code != 0 {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			if !json.Valid([]byte(stdout)) || !strings.Contains(stdout, tc.result) {
				t.Fatalf("unexpected output %q", stdout)
			}
			if !reflect.DeepEqual(*requests, tc.paths) {
				t.Fatalf("requests=%v, want %v", *requests, tc.paths)
			}
			assertEphemeralSessionUnchanged(t, cache, stdout+stderr)
		})
	}
}

func TestEnvironmentSessionIgnoresAppleIDEnvironmentFallback(t *testing.T) {
	requests, cache := ephemeralSessionFixture(t, http.StatusOK, "user@example.com")
	t.Setenv("ASC_WEB_APPLE_ID", "different@example.com")

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"web", "removed-apps", "list", "--session-from-env", "--output", "json"}, "test")
	})
	if code != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !reflect.DeepEqual(*requests, []string{"GET /olympus/v1/session", "GET /iris/v1/apps"}) {
		t.Fatalf("requests=%v", *requests)
	}
	if strings.Contains(stdout+stderr, "different@example.com") {
		t.Fatalf("ASC_WEB_APPLE_ID affected ephemeral session output: stdout=%q stderr=%q", stdout, stderr)
	}
	assertEphemeralSessionUnchanged(t, cache, stdout+stderr)
}

func assertEphemeralSessionUnchanged(t *testing.T, cache, output string) {
	t.Helper()
	if strings.Contains(output, "super-secret-token") || strings.Contains(output, "override-secret") {
		t.Fatal("session secret appeared in output")
	}
	entries, err := os.ReadDir(cache)
	if err != nil || len(entries) != 0 {
		t.Fatalf("cache changed: entries=%v err=%v", entries, err)
	}
}

func TestEnvironmentSessionRejectsBeforeRequests(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		args                   []string
		env, value, diagnostic string
	}{
		{"public", []string{"apps", "list"}, "", "", "unknown flag `--session-from-env`"},
		{"executable group", []string{"validate", "--app", "app-1", "--version", "1.0"}, "", "", "unknown flag `--session-from-env`"},
		{"group", []string{"web", "api-keys"}, "", "", "unknown flag `--session-from-env`"},
		{"mutation", []string{"web", "api-keys", "create", "--name", "test"}, "", "", "unknown flag `--session-from-env`"},
		{"import", []string{"web", "auth", "import", "--from-env"}, "", "", "unknown flag `--session-from-env`"},
		{"missing", []string{"web", "removed-apps", "list"}, "ASC_WEB_SESSION", "", "ASC_WEB_SESSION is unset or empty"},
		{"invalid", []string{"web", "removed-apps", "list"}, "ASC_WEB_SESSION", "override-secret", "invalid session bundle"},
		{"unknown credential field", []string{"web", "removed-apps", "list"}, "ASC_WEB_SESSION", strings.Replace(webSessionBundleFixture, `"version": 1,`, `"version": 1, "override-secret": true,`, 1), "invalid session bundle"},
		{"oversize", []string{"web", "removed-apps", "list"}, "ASC_WEB_SESSION", strings.Repeat("x", (1<<20)+1), "exceeds"},
		{"provider env", []string{"web", "removed-apps", "list"}, "ASC_WEB_SESSION_PROVIDER", "override-secret", "ASC_WEB_SESSION_PROVIDER is unsupported"},
		{"csrf env", []string{"web", "removed-apps", "list"}, "ASC_WEB_SESSION_CSRF", "override-secret", "ASC_WEB_SESSION_CSRF is unsupported"},
		{"provider flag", []string{"web", "removed-apps", "list", "--provider-id", "42"}, "", "", "--provider-id is unsupported"},
		{"zero provider flag", []string{"web", "removed-apps", "list", "--provider-id", "0"}, "", "", "--provider-id is unsupported"},
		{"public provider flag", []string{"web", "removed-apps", "list", "--public-provider-id", "team-42"}, "", "", "--public-provider-id is unsupported"},
		{"2fa command", []string{"web", "removed-apps", "list", "--two-factor-code-command", "echo override-secret"}, "", "", "--two-factor-code-command is unsupported"},
		{"apple id", []string{"web", "removed-apps", "list", "--apple-id", "other@example.com"}, "", "", "does not match --apple-id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests, cache := ephemeralSessionFixture(t, 200, "user@example.com")
			if tc.env != "" {
				t.Setenv(tc.env, tc.value)
			}
			var code int
			stdout, stderr := captureOutput(t, func() { code = cmd.Run(append(tc.args, "--session-from-env"), "test") })
			if code != 2 || !strings.Contains(stderr, tc.diagnostic) {
				t.Fatalf("exit=%d stderr=%q", code, stderr)
			}
			if len(*requests) != 0 {
				t.Fatalf("unexpected requests: %v", *requests)
			}
			assertEphemeralSessionUnchanged(t, cache, stdout+stderr)
		})
	}
}

func TestEnvironmentSessionValidationFailure(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		email  string
	}{
		{"expired", 401, "user@example.com"}, {"mismatch", 200, "other@example.com"}, {"identity missing", 200, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests, cache := ephemeralSessionFixture(t, tc.status, tc.email)
			var code int
			stdout, stderr := captureOutput(t, func() { code = cmd.Run([]string{"web", "removed-apps", "list", "--session-from-env"}, "test") })
			if code == 0 || !strings.Contains(stderr, "session validation failed") {
				t.Fatalf("exit=%d stderr=%q", code, stderr)
			}
			if !reflect.DeepEqual(*requests, []string{"GET /olympus/v1/session"}) {
				t.Fatalf("requests=%v", *requests)
			}
			assertEphemeralSessionUnchanged(t, cache, stdout+stderr)
		})
	}
}

func TestEnvironmentSessionRequiresExplicitOptIn(t *testing.T) {
	for _, suffix := range [][]string{nil, {"--session-from-env=false"}, {"--session-from-env", "false"}} {
		t.Run(strings.Join(suffix, " "), func(t *testing.T) {
			requests, cache := ephemeralSessionFixture(t, 200, "user@example.com")
			t.Setenv("ASC_WEB_SESSION_CACHE_BACKEND", "off")
			var code int
			stdout, stderr := captureOutput(t, func() {
				code = cmd.Run(append([]string{"web", "removed-apps", "list"}, suffix...), "test")
			})
			if code == 0 {
				t.Fatalf("implicit session mode: exit=%d stderr=%q", code, stderr)
			}
			if len(*requests) != 0 {
				t.Fatalf("implicit session requests: %v", *requests)
			}
			assertEphemeralSessionUnchanged(t, cache, stdout+stderr)
		})
	}
}

func TestEnvironmentSessionHelpDoesNotReadCredentials(t *testing.T) {
	for _, args := range [][]string{
		{"web", "removed-apps", "list"}, {"web", "api-keys", "list"}, {"web", "api-keys", "view"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			requests, cache := ephemeralSessionFixture(t, 200, "user@example.com")
			t.Setenv("ASC_WEB_SESSION", "override-secret")
			var code int
			stdout, stderr := captureOutput(t, func() {
				code = cmd.Run(append(args, "--session-from-env", "--help"), "test")
			})
			if code != 0 || stdout == "" {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			if !strings.Contains(stdout, "--session-from-env") || !strings.Contains(stdout, "ASC_WEB_SESSION") || !strings.Contains(stdout, "without persistence") {
				t.Fatalf("help does not explain environment session and persistence: %q", stdout)
			}
			if len(*requests) != 0 {
				t.Fatalf("help requests: %v", *requests)
			}
			assertEphemeralSessionUnchanged(t, cache, stdout+stderr)
		})
	}
}

func TestEnvironmentSessionFlagScope(t *testing.T) {
	var commands []string
	var visit func(*ffcli.Command, string)
	visit = func(command *ffcli.Command, path string) {
		if command.FlagSet != nil && command.FlagSet.Lookup("session-from-env") != nil {
			commands = append(commands, path)
		}
		for _, child := range command.Subcommands {
			visit(child, path+" "+child.Name)
		}
	}
	root := cmd.RootCommand("test")
	visit(root, root.Name)
	sort.Strings(commands)
	want := []string{"asc web api-keys list", "asc web api-keys view", "asc web removed-apps list"}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("--session-from-env commands=%v, want %v", commands, want)
	}
}
