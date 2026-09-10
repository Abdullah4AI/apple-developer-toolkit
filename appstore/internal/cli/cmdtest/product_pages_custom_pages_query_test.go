package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

func installCustomPagesQueryTestClient(t *testing.T, handler http.HandlerFunc) {
	t.Helper()

	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	})
	client, err := asc.NewClientWithHTTPClient(
		os.Getenv("ASC_KEY_ID"),
		os.Getenv("ASC_ISSUER_ID"),
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: transport},
	)
	if err != nil {
		t.Fatalf("create custom pages query test client: %v", err)
	}
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	}))
}

func TestCustomPagesListEmitsQuerySurface(t *testing.T) {
	installCustomPagesQueryTestClient(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/appCustomProductPages" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		want := url.Values{
			"filter[visible]":                      {"false,true"},
			"fields[appCustomProductPages]":        {"name,visible,app,appCustomProductPageVersions"},
			"fields[apps]":                         {"name,bundleId"},
			"fields[appCustomProductPageVersions]": {"version,state"},
			"include":                              {"app,appCustomProductPageVersions"},
			"limit[appCustomProductPageVersions]":  {"25"},
			"limit":                                {"10"},
		}
		if got := req.URL.Query(); got.Encode() != want.Encode() {
			t.Errorf("query = %q, want %q", got.Encode(), want.Encode())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"type":"appCustomProductPages","id":"page-1"}],"links":{"next":""}}`)
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		args := []string{
			"product-pages", "custom-pages", "list", "--app", "app-1",
			"--visible", "false,true",
			"--fields", "name,visible,app",
			"--app-fields", "name,bundleId",
			"--version-fields", "version,state",
			"--include", "app,appCustomProductPageVersions",
			"--versions-limit", "25",
			"--limit", "10",
			"--output", "json",
		}
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, `"id":"page-1"`) {
		t.Fatalf("stdout = %q, want page-1", stdout)
	}
}

func TestCustomPagesListQueryFlagsAreRegistered(t *testing.T) {
	cmd := findCommandByPath(t, "product-pages", "custom-pages", "list")
	for _, name := range []string{"visible", "fields", "app-fields", "version-fields", "include", "versions-limit"} {
		flagValue := cmd.FlagSet.Lookup(name)
		if flagValue == nil {
			t.Fatalf("missing --%s flag", name)
		}
	}
}

func TestCustomPagesListRejectsNextQueryFlagsBeforeAuth(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/appCustomProductPages?cursor=next"
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "visible", args: []string{"--visible", "true"}, want: "--next cannot be combined with --visible"},
		{name: "fields", args: []string{"--fields", "name"}, want: "--next cannot be combined with --fields"},
		{name: "app fields", args: []string{"--app-fields", "name"}, want: "--next cannot be combined with --app-fields"},
		{name: "version fields", args: []string{"--version-fields", "version"}, want: "--next cannot be combined with --version-fields"},
		{name: "include", args: []string{"--include", "app"}, want: "--next cannot be combined with --include"},
		{name: "versions limit", args: []string{"--versions-limit", "10"}, want: "--next cannot be combined with --versions-limit"},
		{name: "limit", args: []string{"--limit", "10"}, want: "--next cannot be combined with --limit"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled := false
			restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("client factory must not run during validation")
			})
			defer restore()

			args := append([]string{"product-pages", "custom-pages", "list", "--next", nextURL}, test.args...)
			assertUsageExit(t, args, "custom-pages list: "+test.want)
			if clientFactoryCalled {
				t.Fatal("client factory ran before --next conflict validation")
			}
		})
	}
}

func TestCustomPagesListRejectsInvalidQueryValuesBeforeAuth(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "visible", args: []string{"--visible", "maybe"}, want: "--visible must be true or false"},
		{name: "page fields", args: []string{"--fields", "name,invalid"}, want: "--fields must be one of"},
		{name: "app fields", args: []string{"--app-fields", "invalid"}, want: "--app-fields must be one of"},
		{name: "version fields", args: []string{"--version-fields", "invalid"}, want: "--version-fields must be one of"},
		{name: "include", args: []string{"--include", "invalid"}, want: "--include must be one of"},
		{name: "versions limit explicitly zero", args: []string{"--versions-limit", "0"}, want: "--versions-limit must be between 1 and 50"},
		{name: "versions limit", args: []string{"--versions-limit", "51"}, want: "--versions-limit must be between 1 and 50"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled := false
			restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("client factory must not run during validation")
			})
			defer restore()

			args := append([]string{"product-pages", "custom-pages", "list", "--app", "app-1"}, test.args...)
			assertUsageExit(t, args, "custom-pages list: "+test.want)
			if clientFactoryCalled {
				t.Fatal("client factory ran before query validation")
			}
		})
	}
}

func TestCustomPagesListRejectsExplicitEmptyQueryFlagsBeforeAuth(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "visible empty", args: []string{"--visible", ""}, want: "--visible must not be empty"},
		{name: "visible delimiters", args: []string{"--visible", ","}, want: "--visible must not be empty"},
		{name: "fields empty", args: []string{"--fields", ""}, want: "--fields must not be empty"},
		{name: "fields delimiters", args: []string{"--fields", ","}, want: "--fields must not be empty"},
		{name: "app fields whitespace", args: []string{"--app-fields", " \t"}, want: "--app-fields must not be empty"},
		{name: "app fields delimiters", args: []string{"--app-fields", ","}, want: "--app-fields must not be empty"},
		{name: "version fields empty", args: []string{"--version-fields", ""}, want: "--version-fields must not be empty"},
		{name: "version fields delimiters", args: []string{"--version-fields", ","}, want: "--version-fields must not be empty"},
		{name: "include whitespace", args: []string{"--include", " \t"}, want: "--include must not be empty"},
		{name: "include delimiters", args: []string{"--include", ","}, want: "--include must not be empty"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled := false
			restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("client factory must not run during empty-value validation")
			})
			defer restore()

			args := append([]string{"product-pages", "custom-pages", "list", "--app", "app-1"}, test.args...)
			assertUsageExit(t, args, "custom-pages list: "+test.want)
			if clientFactoryCalled {
				t.Fatal("client factory ran before empty-value validation")
			}
		})
	}
}

func TestCustomPagesListRelationshipFlagsRequireIncludes(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		parameter string
		want      string
	}{
		{
			name:      "app fields",
			args:      []string{"--app-fields", "name"},
			parameter: "--app-fields",
			want:      "custom-pages list: --app-fields requires --include app",
		},
		{
			name:      "version fields",
			args:      []string{"--include", "app", "--version-fields", "version"},
			parameter: "--version-fields",
			want:      "custom-pages list: --version-fields requires --include appCustomProductPageVersions",
		},
		{
			name:      "versions limit",
			args:      []string{"--include", "app", "--versions-limit", "10"},
			parameter: "--versions-limit",
			want:      "custom-pages list: --versions-limit requires --include appCustomProductPageVersions",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled := false
			restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("client factory must not run during relationship validation")
			})
			defer restore()

			args := append([]string{"product-pages", "custom-pages", "list", "--app", "app-1"}, test.args...)
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})
			if runErr == nil || !shared.IsReportedUsageError(runErr) || errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("run error = %v, want reported usage error without flag.ErrHelp", runErr)
			}
			if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d", got, rootcmd.ExitUsage)
			}
			diagnostic, ok := shared.DiagnosticFromError(runErr)
			if !ok {
				t.Fatal("run error is missing validation diagnostic")
			}
			if diagnostic.Code != shared.DiagnosticInvalidInput || diagnostic.Parameter != test.parameter {
				t.Fatalf("diagnostic = %+v, want code %q parameter %q", diagnostic, shared.DiagnosticInvalidInput, test.parameter)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			wantStderr := "Error: " + test.want + "\n"
			if stderr != wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, wantStderr)
			}
			if clientFactoryCalled {
				t.Fatal("client factory ran before relationship validation")
			}
		})
	}
}

func TestCustomPagesListAllowsNextWithoutApp(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/appCustomProductPages?cursor=next"
	installCustomPagesQueryTestClient(t, func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/v1/apps/app-1/appCustomProductPages" || req.URL.Query().Get("cursor") != "next" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"type":"appCustomProductPages","id":"page-next"}],"links":{"next":""}}`)
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "custom-pages", "list", "--next", nextURL, "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, `"id":"page-next"`) {
		t.Fatalf("stdout = %q, want page-next", stdout)
	}
}

func TestCustomPagesListAllowsNextWithApp(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/appCustomProductPages?cursor=opaque%2Btoken&limit=17"
	installCustomPagesQueryTestClient(t, func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/v1/apps/app-1/appCustomProductPages" || req.URL.RawQuery != "cursor=opaque%2Btoken&limit=17" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"type":"appCustomProductPages","id":"page-opaque"}],"links":{"next":""}}`)
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "custom-pages", "list", "--app", "app-compat", "--next", nextURL, "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, `"id":"page-opaque"`) {
		t.Fatalf("stdout = %q, want page-opaque", stdout)
	}
}
