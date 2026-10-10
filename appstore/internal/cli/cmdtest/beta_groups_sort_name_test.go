package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

// TestBetaGroupsListSortPropagates proves --sort reaches the API.
func TestBetaGroupsListSortPropagates(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/betaGroups" {
			t.Errorf("expected path /v1/betaGroups, got %s", req.URL.Path)
		}
		query := req.URL.Query()
		if got := query.Get("filter[app]"); got != "app-1" {
			t.Errorf("filter[app] = %q, want app-1", got)
		}
		if got := query.Get("sort"); got != "-createdDate" {
			t.Errorf("sort = %q, want -createdDate", got)
		}
		return betaGroupsJSONResponse(`{"data":[]}`), nil
	}))

	stdout, stderr := runBetaGroupsList(t, "--app", "app-1", "--sort", "-createdDate")

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"data":[]`) {
		t.Fatalf("expected empty data envelope, got %q", stdout)
	}
}

// TestBetaGroupsListGlobalNameAndSortPropagate proves --name and --sort reach
// the global endpoint together.
func TestBetaGroupsListGlobalNameAndSortPropagate(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/betaGroups" {
			t.Errorf("expected path /v1/betaGroups, got %s", req.URL.Path)
		}
		query := req.URL.Query()
		if got := query.Get("filter[name]"); got != "QA Testers" {
			t.Errorf("filter[name] = %q, want QA Testers", got)
		}
		if got := query.Get("sort"); got != "name" {
			t.Errorf("sort = %q, want name", got)
		}
		if got := query.Get("filter[app]"); got != "" {
			t.Errorf("filter[app] = %q, want none for --global", got)
		}
		return betaGroupsJSONResponse(`{"data":[]}`), nil
	}))

	_, stderr := runBetaGroupsList(t, "--global", "--name", "QA Testers", "--sort", "name")

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

// TestBetaGroupsListNameFilterUsesTopLevelEndpoint proves --name routes the
// app-scoped lookup through the endpoint that supports filter[name].
func TestBetaGroupsListNameFilterUsesTopLevelEndpoint(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/betaGroups" {
			t.Errorf("expected path /v1/betaGroups, got %s", req.URL.Path)
		}
		query := req.URL.Query()
		if got := query.Get("filter[app]"); got != "app-1" {
			t.Errorf("filter[app] = %q, want app-1", got)
		}
		if got := query.Get("filter[name]"); got != "Beta Testers" {
			t.Errorf("filter[name] = %q, want Beta Testers", got)
		}
		return betaGroupsJSONResponse(`{"data":[]}`), nil
	}))

	_, stderr := runBetaGroupsList(t, "--app", "app-1", "--name", "Beta Testers")

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

// TestBetaGroupsListInvalidSortIsUsageError proves an unsupported --sort value
// fails with a usage error that lists the valid values.
func TestBetaGroupsListInvalidSortIsUsageError(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	assertUsageExit(
		t,
		[]string{"testflight", "groups", "list", "--app", "app-1", "--sort", "nope"},
		"--sort must be one of: name, -name, createdDate, -createdDate, publicLinkEnabled, -publicLinkEnabled, publicLinkLimit, -publicLinkLimit",
	)
}

// TestBetaGroupsListBlankSortAndNameAreUsageErrors proves an explicitly blank
// --name or --sort is reported instead of quietly widening the listing.
func TestBetaGroupsListBlankSortAndNameAreUsageErrors(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tests := []struct {
		args    []string
		wantErr string
	}{
		{[]string{"testflight", "groups", "list", "--app", "app-1", "--sort", ""}, "--sort cannot be empty"},
		{[]string{"testflight", "groups", "list", "--app", "app-1", "--sort", "   "}, "--sort cannot be empty"},
		{[]string{"testflight", "groups", "list", "--app", "app-1", "--name", ""}, "--name cannot be empty"},
		{[]string{"testflight", "groups", "list", "--app", "app-1", "--name", "   "}, "--name cannot be empty"},
	}
	for _, test := range tests {
		assertUsageExit(t, test.args, test.wantErr)
	}
}

// TestBetaGroupsListSortAndNameRejectedWithBuildID proves the membership lookup
// does not silently ignore query flags it cannot honor.
func TestBetaGroupsListSortAndNameRejectedWithBuildID(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tests := [][]string{
		{"testflight", "groups", "list", "--build-id", "build-1", "--sort", "name"},
		{"testflight", "groups", "list", "--build-id", "build-1", "--name", "Beta Testers"},
	}
	for _, args := range tests {
		assertUsageExit(t, args, "Error: --name and --sort cannot be used with --build-id")
	}
}
