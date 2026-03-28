package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildsReadCommandsResolveLatestSelector(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	tests := []struct {
		name         string
		args         []string
		expectedPath string
		responseBody string
		expectStdout string
	}{
		{
			name:         "app view",
			args:         []string{"builds", "app", "view", "--app", "123456789", "--latest", "--output", "json"},
			expectedPath: "/v1/builds/BUILD_123/app",
			responseBody: `{"data":{"type":"apps","id":"app-1"}}`,
			expectStdout: `"id":"app-1"`,
		},
		{
			name:         "pre-release-version view",
			args:         []string{"builds", "pre-release-version", "view", "--app", "123456789", "--latest", "--output", "json"},
			expectedPath: "/v1/builds/BUILD_123/preReleaseVersion",
			responseBody: `{"data":{"type":"preReleaseVersions","id":"prv-1"}}`,
			expectStdout: `"id":"prv-1"`,
		},
		{
			name:         "icons list",
			args:         []string{"builds", "icons", "list", "--app", "123456789", "--latest", "--output", "json"},
			expectedPath: "/v1/builds/BUILD_123/icons",
			responseBody: `{"data":[{"type":"buildIcons","id":"icon-1"}]}`,
			expectStdout: `"id":"icon-1"`,
		},
		{
			name:         "beta-app-review-submission view",
			args:         []string{"builds", "beta-app-review-submission", "view", "--app", "123456789", "--latest", "--output", "json"},
			expectedPath: "/v1/builds/BUILD_123/betaAppReviewSubmission",
			responseBody: `{"data":{"type":"betaAppReviewSubmissions","id":"review-1"}}`,
			expectStdout: `"id":"review-1"`,
		},
		{
			name:         "build-beta-detail view",
			args:         []string{"builds", "build-beta-detail", "view", "--app", "123456789", "--latest", "--output", "json"},
			expectedPath: "/v1/builds/BUILD_123/buildBetaDetail",
			responseBody: `{"data":{"type":"buildBetaDetails","id":"detail-1"}}`,
			expectStdout: `"id":"detail-1"`,
		},
		{
			name:         "links view",
			args:         []string{"builds", "links", "view", "--app", "123456789", "--latest", "--type", "app", "--output", "json"},
			expectedPath: "/v1/builds/BUILD_123/relationships/app",
			responseBody: `{"data":{"type":"apps","id":"app-1"}}`,
			expectStdout: `"id":"app-1"`,
		},
		{
			name:         "individual-testers list",
			args:         []string{"builds", "individual-testers", "list", "--app", "123456789", "--latest", "--output", "json"},
			expectedPath: "/v1/builds/BUILD_123/individualTesters",
			responseBody: `{"data":[{"type":"betaTesters","id":"tester-1"}]}`,
			expectStdout: `"id":"tester-1"`,
		},
		{
			name:         "metrics beta-usages",
			args:         []string{"builds", "metrics", "beta-usages", "--app", "123456789", "--latest", "--output", "json"},
			expectedPath: "/v1/builds/BUILD_123/metrics/betaBuildUsages",
			responseBody: `{"data":[{"type":"betaBuildUsages","id":"usage-1"}]}`,
			expectStdout: `"id":"usage-1"`,
		},
		{
			name:         "app-encryption-declaration view",
			args:         []string{"builds", "app-encryption-declaration", "view", "--app", "123456789", "--latest", "--output", "json"},
			expectedPath: "/v1/builds/BUILD_123/appEncryptionDeclaration",
			responseBody: `{"data":{"type":"appEncryptionDeclarations","id":"enc-1"}}`,
			expectStdout: `"id":"enc-1"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requestCount := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requestCount++
				switch requestCount {
				case 1:
					if req.Method != http.MethodGet {
						t.Fatalf("expected GET, got %s", req.Method)
					}
					if req.URL.Path != "/v1/builds" {
						t.Fatalf("expected build lookup path /v1/builds, got %s", req.URL.Path)
					}
					query := req.URL.Query()
					if query.Get("filter[app]") != "123456789" {
						t.Fatalf("expected filter[app]=123456789, got %q", query.Get("filter[app]"))
					}
					if query.Get("sort") != "-uploadedDate" {
						t.Fatalf("expected sort=-uploadedDate, got %q", query.Get("sort"))
					}
					if query.Get("limit") != "200" {
						t.Fatalf("expected limit=200 for latest selector lookup, got %q", query.Get("limit"))
					}
					return jsonHTTPResponse(http.StatusOK, `{
						"data":[{"type":"builds","id":"BUILD_123","attributes":{"uploadedDate":"2026-03-13T00:00:00Z","processingState":"VALID","version":"42"},"relationships":{}}]
					}`), nil
				case 2:
					if req.Method != http.MethodGet {
						t.Fatalf("expected GET, got %s", req.Method)
					}
					if req.URL.Path != test.expectedPath {
						t.Fatalf("expected path %s, got %s", test.expectedPath, req.URL.Path)
					}
					return jsonHTTPResponse(http.StatusOK, test.responseBody), nil
				default:
					t.Fatalf("unexpected request count %d", requestCount)
					return nil, nil
				}
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if !strings.Contains(stdout, test.expectStdout) {
				t.Fatalf("expected stdout to contain %q, got %q", test.expectStdout, stdout)
			}
		})
	}
}

func TestBuildsMutatingCommandsAcceptUnifiedSelectorsForValidation(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "expire latest requires confirm",
			args:    []string{"builds", "expire", "--app", "123456789", "--latest"},
			wantErr: "Error: --confirm is required to expire build",
		},
		{
			name:    "update build-number requires update flag",
			args:    []string{"builds", "update", "--app", "123456789", "--build-number", "42"},
			wantErr: "Error: at least one update flag is required",
		},
		{
			name:    "add-groups latest requires group",
			args:    []string{"builds", "add-groups", "--app", "123456789", "--latest"},
			wantErr: "Error: --group is required",
		},
		{
			name:    "remove-groups build-number requires group",
			args:    []string{"builds", "remove-groups", "--app", "123456789", "--build-number", "42", "--confirm"},
			wantErr: "Error: --group is required",
		},
		{
			name:    "individual-testers add latest requires tester",
			args:    []string{"builds", "individual-testers", "add", "--app", "123456789", "--latest"},
			wantErr: "Error: --tester is required",
		},
		{
			name:    "individual-testers remove build-number requires tester",
			args:    []string{"builds", "individual-testers", "remove", "--app", "123456789", "--build-number", "42", "--confirm"},
			wantErr: "Error: --tester is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected ErrHelp, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
			if strings.Contains(stderr, "Error: --build-id is required") {
				t.Fatalf("expected unified selectors to be accepted, got %q", stderr)
			}
		})
	}
}
