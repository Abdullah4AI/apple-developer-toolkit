package cmdtest

import (
	"context"
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
	iapcli "github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/iap"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

type iapListQuerySurfaceRequest struct {
	calls int
	path  string
	query url.Values
}

func iapListQuerySurfaceStub(t *testing.T) *iapListQuerySurfaceRequest {
	t.Helper()

	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	captured := &iapListQuerySurfaceRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		captured.calls++
		captured.path = req.URL.Path
		captured.query = req.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Pro","productId":"com.example.pro","inAppPurchaseType":"CONSUMABLE","state":"READY_TO_SUBMIT"}}],"links":{"next":""}}`)
	}))
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.URL.Scheme + "://" + req.URL.Host; got != asc.BaseURL {
			t.Errorf("request origin = %s, want %s", got, asc.BaseURL)
		}
		routed := req.Clone(req.Context())
		routed.URL.Scheme = serverURL.Scheme
		routed.URL.Host = serverURL.Host
		routed.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(routed)
	})
	client, err := asc.NewClientWithHTTPClient(
		"TEST_KEY",
		"TEST_ISSUER",
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: transport},
	)
	if err != nil {
		t.Fatalf("create test client: %v", err)
	}
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	}))

	return captured
}

func (r *iapListQuerySurfaceRequest) assertNoRequest(t *testing.T) {
	t.Helper()
	if r.calls != 0 {
		t.Fatalf("expected validation to short-circuit before any request, got %d call(s) to %s?%s", r.calls, r.path, r.query.Encode())
	}
}

func runIAPListQuerySurface(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	return stdout, stderr, runErr
}

func TestIAPListQuerySurfaceEmitsDocumentedSelectors(t *testing.T) {
	captured := iapListQuerySurfaceStub(t)

	stdout, stderr, err := runIAPListQuerySurface(
		t,
		"iap", "list",
		"--app", "app-1",
		"--product-id", "com.example.pro,com.example.pro2",
		"--name", "Pro,Pro Plus",
		"--state", "ready_to_submit,approved",
		"--type", "consumable,non_consumable",
		"--sort", "name,-inAppPurchaseType",
		"--output", "json",
	)
	if err != nil {
		t.Fatalf("run error: %v (stderr=%q)", err, stderr)
	}

	if captured.path != "/v1/apps/app-1/inAppPurchasesV2" {
		t.Fatalf("expected v2 path, got %q", captured.path)
	}
	want := url.Values{
		"filter[productId]":         {"com.example.pro,com.example.pro2"},
		"filter[name]":              {"Pro,Pro Plus"},
		"filter[state]":             {"READY_TO_SUBMIT,APPROVED"},
		"filter[inAppPurchaseType]": {"CONSUMABLE,NON_CONSUMABLE"},
		"sort":                      {"name,-inAppPurchaseType"},
	}
	if got := captured.query; got.Encode() != want.Encode() {
		t.Fatalf("query = %s, want %s", got.Encode(), want.Encode())
	}
	if !strings.Contains(stdout, `"id":"iap-1"`) {
		t.Fatalf("expected IAP envelope, got %q", stdout)
	}
}

func TestIAPListQuerySurfaceDeduplicatesNormalizedEnumValues(t *testing.T) {
	captured := iapListQuerySurfaceStub(t)

	_, stderr, err := runIAPListQuerySurface(
		t,
		"iap", "list",
		"--app", "app-1",
		"--state", "ready_to_submit,READY_TO_SUBMIT",
		"--type", "consumable,CONSUMABLE",
		"--output", "json",
	)
	if err != nil {
		t.Fatalf("run error: %v (stderr=%q)", err, stderr)
	}

	if got := captured.query.Get("filter[state]"); got != "READY_TO_SUBMIT" {
		t.Fatalf("filter[state] = %q, want READY_TO_SUBMIT", got)
	}
	if got := captured.query.Get("filter[inAppPurchaseType]"); got != "CONSUMABLE" {
		t.Fatalf("filter[inAppPurchaseType] = %q, want CONSUMABLE", got)
	}
}

func TestIAPListQueryFlagsAreRegistered(t *testing.T) {
	command := iapcli.IAPListCommand()
	for _, name := range []string{"product-id", "name", "state", "type", "sort"} {
		flagValue := command.FlagSet.Lookup(name)
		if flagValue == nil {
			t.Fatalf("--%s is not registered", name)
		}
	}
}

func TestIAPListQuerySurfaceRejectsInvalidSelectorsBeforeRequest(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "state",
			args: []string{"iap", "list", "--app", "app-1", "--state", "NOT_A_STATE"},
			want: "--state must be one of:",
		},
		{
			name: "type",
			args: []string{"iap", "list", "--app", "app-1", "--type", "NOT_A_TYPE"},
			want: "--type must be one of:",
		},
		{
			name: "sort",
			args: []string{"iap", "list", "--app", "app-1", "--sort", "createdDate"},
			want: "--sort must be one of:",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			captured := iapListQuerySurfaceStub(t)
			_, stderr, err := runIAPListQuerySurface(t, test.args...)
			if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, err)
			}
			if !strings.Contains(stderr, test.want) {
				t.Fatalf("expected stderr to contain %q, got %q", test.want, stderr)
			}
			captured.assertNoRequest(t)
		})
	}
}

func TestIAPListQuerySurfaceRejectsExplicitlyEmptySelectors(t *testing.T) {
	for _, name := range []string{"product-id", "name", "state", "type", "sort"} {
		t.Run(name, func(t *testing.T) {
			captured := iapListQuerySurfaceStub(t)
			_, stderr, err := runIAPListQuerySurface(t, "iap", "list", "--app", "app-1", "--"+name, "")
			if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, err)
			}
			want := "--" + name + " must not be empty"
			if !strings.Contains(stderr, want) {
				t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
			}
			captured.assertNoRequest(t)
		})
	}
}

func TestIAPListQuerySurfaceRejectsNextWithSelectors(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/apps/app-1/inAppPurchasesV2?cursor=next"
	flags := []string{"product-id", "name", "state", "type", "sort"}
	for _, name := range flags {
		t.Run(name, func(t *testing.T) {
			captured := iapListQuerySurfaceStub(t)
			_, stderr, err := runIAPListQuerySurface(t, "iap", "list", "--next", next, "--"+name, "value")
			if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, err)
			}
			want := "--next cannot be combined with --" + name
			if !strings.Contains(stderr, want) {
				t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
			}
			captured.assertNoRequest(t)
		})
	}
}

func TestIAPListQuerySurfaceRejectsV2SelectorsOnLegacy(t *testing.T) {
	flags := []string{"product-id", "name", "state", "type", "sort"}
	for _, name := range flags {
		t.Run(name, func(t *testing.T) {
			captured := iapListQuerySurfaceStub(t)
			_, stderr, err := runIAPListQuerySurface(t, "iap", "list", "--app", "app-1", "--legacy", "--"+name, "value")
			if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, err)
			}
			want := "--" + name + " requires the v2 endpoint"
			if !strings.Contains(stderr, want) {
				t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
			}
			captured.assertNoRequest(t)
		})
	}
}

func TestIAPListQuerySurfaceKeepsLegacyModeWithoutSelectors(t *testing.T) {
	captured := iapListQuerySurfaceStub(t)

	_, stderr, err := runIAPListQuerySurface(t, "iap", "list", "--app", "app-1", "--legacy", "--output", "json")
	if err != nil {
		t.Fatalf("run error: %v (stderr=%q)", err, stderr)
	}
	if captured.path != "/v1/apps/app-1/inAppPurchases" {
		t.Fatalf("expected legacy path, got %q", captured.path)
	}
	if captured.query.Encode() != "" {
		t.Fatalf("expected no legacy query, got %s", captured.query.Encode())
	}
}
