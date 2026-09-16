package cmdtest

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

// versionRelationshipTypeValues mirrors the relationship types the command
// accepts, in the order the CLI prints them.
var versionRelationshipTypeValues = []string{
	"ageRatingDeclaration",
	"alternativeDistributionPackage",
	"appClipDefaultExperience",
	"appStoreReviewDetail",
	"appStoreVersionExperiments",
	"appStoreVersionExperimentsV2",
	"appStoreVersionSubmission",
	"customerReviews",
	"gameCenterAppVersion",
	"routingAppCoverage",
}

// Captured live from GET
// /v1/appStoreVersions/999999999999/relationships/appStoreReviewDetail on
// 2026-09-15.
const appStoreVersionNotFoundBody = `{"errors":[{"id":"7e1e1ba0-2b1f-4a0a-9a3f-4f9e2d3b7c11","status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'appStoreVersions' with id '999999999999'"}]}`

func TestVersionsLinksTypeUsageErrorsEnumerateValidValues(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantPrefix string
	}{
		{
			name:       "missing type",
			args:       []string{"versions", "links", "--version-id", "version-1"},
			wantPrefix: "Error: --type is required; must be one of: ",
		},
		{
			name:       "invalid type",
			args:       []string{"versions", "links", "--version-id", "version-1", "--type", "notARelationship"},
			wantPrefix: `Error: --type "notARelationship" is not a valid relationship type; must be one of: `,
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

			if got := cmd.ExitCodeFromError(runErr); got != cmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (%v)", got, cmd.ExitUsage, runErr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.HasPrefix(stderr, test.wantPrefix) {
				t.Fatalf("stderr = %q, want prefix %q", stderr, test.wantPrefix)
			}
			for _, value := range versionRelationshipTypeValues {
				if !strings.Contains(stderr, value) {
					t.Fatalf("stderr = %q, missing relationship value %q", stderr, value)
				}
			}
		})
	}
}

func TestVersionsLinksUnknownVersionReportsVersionNotFound(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	server := newNotFoundServer(t, "/v1/appStoreVersions/999999999999/relationships/appStoreReviewDetail", appStoreVersionNotFoundBody)
	useVersionLinksServerClient(t, server)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"versions", "links", "--version-id", "999999999999", "--type", "appStoreReviewDetail", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected not-found error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if got := cmd.ExitCodeFromError(runErr); got != cmd.ExitNotFound {
		t.Fatalf("exit code = %d, want %d (%v)", got, cmd.ExitNotFound, runErr)
	}
	message := runErr.Error()
	wantMessage := `versions links: app store version "999999999999" was not found; --version-id expects an App Store version ID, not an app ID (list them with: asc versions list --app "APP_ID")`
	if message != wantMessage {
		t.Fatalf("error = %q, want %q", message, wantMessage)
	}
}

func TestVersionsLinksMissingRelationshipReportsRelationship(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	const body = `{"errors":[{"id":"1a2b3c4d-0000-4a0a-9a3f-4f9e2d3b7c11","status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'customerReviews' with id 'version-1'"}]}`
	server := newNotFoundServer(t, "/v1/appStoreVersions/version-1/relationships/customerReviews", body)
	useVersionLinksServerClient(t, server)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"versions", "links", "--version-id", "version-1", "--type", "customerReviews", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected not-found error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if got := cmd.ExitCodeFromError(runErr); got != cmd.ExitNotFound {
		t.Fatalf("exit code = %d, want %d (%v)", got, cmd.ExitNotFound, runErr)
	}
	if !strings.HasPrefix(runErr.Error(), `versions links: customerReviews relationship was not found for app store version "version-1"`) {
		t.Fatalf("error = %q, want the relationship named as missing", runErr)
	}
}

func TestVersionsLinksUnclassifiedNotFoundKeepsAPIMessage(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	const body = `{"errors":[{"id":"5f6e7d8c-0000-4a0a-9a3f-4f9e2d3b7c11","status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'apps' with id 'app-1'"}]}`
	server := newNotFoundServer(t, "/v1/appStoreVersions/version-1/relationships/customerReviews", body)
	useVersionLinksServerClient(t, server)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{"versions", "links", "--version-id", "version-1", "--type", "customerReviews", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected not-found error")
	}
	if got := cmd.ExitCodeFromError(runErr); got != cmd.ExitNotFound {
		t.Fatalf("exit code = %d, want %d (%v)", got, cmd.ExitNotFound, runErr)
	}
	want := "versions links: The specified resource does not exist: There is no resource of type 'apps' with id 'app-1'"
	if runErr.Error() != want {
		t.Fatalf("error = %q, want the unmodified API message %q", runErr, want)
	}
}

func TestVersionsLinksNextURLNotFoundDoesNotBlameVersionFlag(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	const body = `{"errors":[{"id":"9c8b7a65-0000-4a0a-9a3f-4f9e2d3b7c11","status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'appStoreVersions' with id 'other-version'"}]}`
	server := newNotFoundServer(t, "/v1/appStoreVersions/other-version/relationships/customerReviews", body)
	useVersionLinksServerClient(t, server)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, _ = captureOutput(t, func() {
		args := []string{
			"versions", "links",
			"--version-id", "version-1",
			"--type", "customerReviews",
			"--next", "https://api.appstoreconnect.apple.com/v1/appStoreVersions/other-version/relationships/customerReviews?cursor=NEXT",
			"--output", "json",
		}
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected not-found error")
	}
	if got := cmd.ExitCodeFromError(runErr); got != cmd.ExitNotFound {
		t.Fatalf("exit code = %d, want %d (%v)", got, cmd.ExitNotFound, runErr)
	}
	want := "versions links: the app store version referenced by the requested page URL was not found"
	if runErr.Error() != want {
		t.Fatalf("error = %q, want %q", runErr, want)
	}
}

// useVersionLinksServerClient points the general command client factory at the
// test server so versions links exercises real transport and error parsing.
func useVersionLinksServerClient(t *testing.T, server *httptest.Server) {
	t.Helper()

	client, err := asc.NewClientWithHTTPClient(
		"TEST_KEY",
		"TEST_ISSUER",
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: serverRoundTripper(t, server)},
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	})
	t.Cleanup(restore)
}
