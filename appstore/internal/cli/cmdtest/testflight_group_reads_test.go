package cmdtest

import (
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
)

func TestTestFlightGroupReadsSendRequest(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantPath  string
		wantQuery url.Values
		body      string
	}{
		{
			name:     "group builds with fields",
			args:     []string{"testflight", "groups", "builds", "list", "--group-id", "group-1", "--fields", "version,uploadedDate,processingState", "--limit", "5"},
			wantPath: "/v1/betaGroups/group-1/builds",
			wantQuery: url.Values{
				"fields[builds]": {"version,uploadedDate,processingState"},
				"limit":          {"5"},
			},
		},
		{
			name:      "group testers paginate keep the page size",
			args:      []string{"testflight", "groups", "testers", "list", "--group-id", "group-1", "--paginate", "--limit", "3"},
			wantPath:  "/v1/betaGroups/group-1/betaTesters",
			wantQuery: url.Values{"limit": {"3"}},
		},
		{
			name:      "group builds print Apple's sparse envelope unmodified",
			args:      []string{"testflight", "groups", "builds", "list", "--group-id", "group-1", "--fields", "version", "--limit", "1"},
			wantPath:  "/v1/betaGroups/group-1/builds",
			wantQuery: url.Values{"fields[builds]": {"version"}, "limit": {"1"}},
			body:      `{"data":[{"type":"builds","id":"build-1","attributes":{"version":"11"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/betaGroups/group-1/builds"}}`,
		},
		{
			name:     "group testers with fields",
			args:     []string{"testflight", "groups", "testers", "list", "--group-id", "group-1", "--fields", "email,state"},
			wantPath: "/v1/betaGroups/group-1/betaTesters",
			wantQuery: url.Values{
				"fields[betaTesters]": {"email,state"},
			},
		},
		{
			name:     "app testers with fields keep included relationships",
			args:     []string{"testflight", "testers", "list", "--app", "app-1", "--fields", "email", "--include", "betaGroups"},
			wantPath: "/v1/betaTesters",
			wantQuery: url.Values{
				"fields[betaTesters]": {"email,betaGroups"},
				"filter[apps]":        {"app-1"},
				"include":             {"betaGroups"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			body := test.body
			if body == "" {
				body = `{"data":[],"links":{}}`
			}
			var captured *http.Request
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				captured = req
				return okJSONResponse(body), nil
			}))

			stdout, stderr := captureOutput(t, func() {
				if code := cmd.Run(append(test.args, "--output", "json"), "1.2.3"); code != cmd.ExitSuccess {
					t.Fatalf("expected exit code %d, got %d", cmd.ExitSuccess, code)
				}
			})
			if captured == nil {
				t.Fatalf("expected an outbound request, stderr %q", stderr)
			}
			if captured.Method != http.MethodGet || captured.URL.Path != test.wantPath {
				t.Fatalf("request = %s %s, want GET %s", captured.Method, captured.URL.Path, test.wantPath)
			}
			if got := captured.URL.Query().Encode(); got != test.wantQuery.Encode() {
				t.Fatalf("query = %q, want %q", got, test.wantQuery.Encode())
			}
			if got := strings.TrimSpace(stdout); got != body {
				t.Fatalf("stdout = %q, want Apple's body %q", got, body)
			}
		})
	}
}

func TestTestFlightGroupReadsRejectUsage(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/betaGroups/group-1/builds?cursor=AQ"

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "group builds without group id",
			args:    []string{"testflight", "groups", "builds", "list"},
			wantErr: "--group-id is required",
		},
		{
			name:    "group builds fields with next",
			args:    []string{"testflight", "groups", "builds", "list", "--next", nextURL, "--fields", "version"},
			wantErr: "testflight groups builds list: --next cannot be combined with --fields",
		},
		{
			name:    "group builds limit with next",
			args:    []string{"testflight", "groups", "builds", "list", "--next", nextURL, "--limit", "5"},
			wantErr: "--next cannot be combined with --limit",
		},
		{
			name:    "group builds unknown field",
			args:    []string{"testflight", "groups", "builds", "list", "--group-id", "group-1", "--fields", "name"},
			wantErr: "--fields must be one of: version, uploadedDate",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
				return nil, nil
			}))

			stdout, stderr := captureOutput(t, func() {
				if code := cmd.Run(test.args, "1.2.3"); code != cmd.ExitUsage {
					t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}
