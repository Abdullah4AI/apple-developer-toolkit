package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

// Apple's 409 on POST /v1/appStoreVersions when the platform already has an
// unreleased version, with the detail Apple returned live on 2026-09-15.
const versionsCreateBlocked409 = `{"errors":[{"id":"7c2d9e1f-4b3a-4c6d-8e5f-1a2b3c4d5e6f","status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","title":"The provided entity includes a relationship with an invalid value","detail":"You cannot create a new version of the App in the current state.","source":{"pointer":"/data/relationships/app"}}]}`

func TestRunVersionsCreateConflictNamesUnreleasedVersion(t *testing.T) {
	resetReportFlags(t)
	t.Setenv("ASC_APP_ID", "")

	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests = append(requests, req.Method+" "+req.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appStoreVersions":
			w.WriteHeader(http.StatusConflict)
			fmt.Fprint(w, versionsCreateBlocked409)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			if got := req.URL.Query().Get("filter[platform]"); got != "IOS" {
				t.Fatalf("filter[platform] = %q, want IOS", got)
			}
			if req.URL.Query().Has("filter[versionString]") {
				t.Fatalf("diagnostic query = %q, want every IOS version", req.URL.RawQuery)
			}
			fmt.Fprint(w, `{"data":[`+
				`{"type":"appStoreVersions","id":"ver-live","attributes":{"versionString":"1.0.0","platform":"IOS","appStoreState":"READY_FOR_SALE","createdDate":"2026-01-01T00:00:00Z"}},`+
				`{"type":"appStoreVersions","id":"ver-review","attributes":{"versionString":"1.1.0","platform":"IOS","appVersionState":"WAITING_FOR_REVIEW","createdDate":"2026-08-01T00:00:00Z"}}`+
				`]}`)
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
		}
	}))
	defer server.Close()

	client := newHTTPStatusTestClient(t, server.URL)
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	}))

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{
			"versions", "create",
			"--app", "app-1",
			"--version", "2.0.0",
			"--output", "json",
		}, "4.0.0"); code != ExitConflict {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitConflict)
		}
	})

	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %v, want the POST then one diagnostic listing", requests)
	}
	for _, want := range []string{
		"Error: versions create: ",
		"You cannot create a new version of the App in the current state.",
		"IOS versions that are not live yet (an unreleased version usually blocks creating another):\n  1.1.0  IOS  WAITING_FOR_REVIEW  ver-review\n",
		`Version 1.1.0 is WAITING_FOR_REVIEW; wait until review finishes or it is released, or inspect it: asc versions view --version-id "ver-review"`,
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want %q", stderr, want)
		}
	}
	if strings.Contains(stderr, "ver-live") {
		t.Fatalf("stderr = %q, want the live version left out of the unreleased list", stderr)
	}
}
