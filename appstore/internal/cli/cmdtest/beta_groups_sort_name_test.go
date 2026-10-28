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
// app-scoped lookup through the endpoint that supports filter[name], and that
// a name typed with ASCII spaces finds Apple's default group, whose name has a
// non-breaking space, with one extra lookup and no extra request on an exact match.
func TestBetaGroupsListNameFilterUsesTopLevelEndpoint(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const appGroups = `{"data":[` +
		`{"type":"betaGroups","id":"bg-default","attributes":{"name":"App\u00a0Store Connect Users"}},` +
		`{"type":"betaGroups","id":"bg-qa","attributes":{"name":"QA"}}]}`
	const nbspGroup = `{"data":[{"type":"betaGroups","id":"bg-default","attributes":{"name":"App\u00a0Store Connect Users"}}]}`

	type request struct{ path, name, body string }
	tests := []struct {
		name       string
		nameFlag   string
		requests   []request
		wantStdout string
	}{
		{
			name:     "exact match makes no extra request",
			nameFlag: "Beta Testers",
			requests: []request{
				{"/v1/betaGroups", "Beta Testers", `{"data":[{"type":"betaGroups","id":"bg-1","attributes":{"name":"Beta Testers"}}]}`},
			},
			wantStdout: `"id":"bg-1"`,
		},
		{
			name:     "ASCII spaces resolve the NBSP-named group",
			nameFlag: "app store connect users",
			requests: []request{
				{"/v1/betaGroups", "app store connect users", `{"data":[]}`},
				{"/v1/apps/app-1/betaGroups", "", appGroups},
				{"/v1/betaGroups", "App\u00a0Store Connect Users", nbspGroup},
			},
			wantStdout: `"id":"bg-default"`,
		},
		{
			name:     "no match prints the empty envelope after one lookup",
			nameFlag: "Missing Group",
			requests: []request{
				{"/v1/betaGroups", "Missing Group", `{"data":[]}`},
				{"/v1/apps/app-1/betaGroups", "", appGroups},
			},
			wantStdout: `"data":[]`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var count int
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				count++
				if count > len(test.requests) {
					t.Errorf("unexpected request %d: %s", count, req.URL.String())
					return betaGroupsJSONResponse(`{"data":[]}`), nil
				}
				want := test.requests[count-1]
				query := req.URL.Query()
				if req.URL.Path != want.path || query.Get("filter[name]") != want.name {
					t.Errorf("request %d = %s filter[name]=%q, want %s filter[name]=%q", count, req.URL.Path, query.Get("filter[name]"), want.path, want.name)
				}
				if want.path == "/v1/betaGroups" && query.Get("filter[app]") != "app-1" {
					t.Errorf("request %d filter[app] = %q, want app-1", count, query.Get("filter[app]"))
				}
				return betaGroupsJSONResponse(want.body), nil
			}))

			stdout, stderr := runBetaGroupsList(t, "--app", "app-1", "--name", test.nameFlag, "--output", "json")

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if count != len(test.requests) {
				t.Fatalf("made %d requests, want %d", count, len(test.requests))
			}
			if !strings.Contains(stdout, test.wantStdout) {
				t.Fatalf("stdout = %q, want %s", stdout, test.wantStdout)
			}
		})
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
