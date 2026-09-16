package cmdtest

import (
	"context"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	rootcmd "github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

type addGroupsDiagnosticsFixture struct {
	t *testing.T

	external     bool
	postStatus   int
	postBody     string
	buildStatus  int
	buildBody    string
	detailStatus int
	detailBody   string

	postCount   int
	buildReads  int
	detailReads int
	requests    []string
}

func (f *addGroupsDiagnosticsFixture) transport() roundTripFunc {
	return func(req *http.Request) (*http.Response, error) {
		request := req.Method + " " + req.URL.Path
		f.requests = append(f.requests, request)
		switch request {
		case "GET /v1/builds/build-1/app":
			return jsonResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1"}}`)
		case "GET /v1/apps/app-1/betaGroups":
			isInternal := !f.external
			return jsonResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"QA","isInternalGroup":`+boolString(isInternal)+`}}]}`)
		case "POST /v1/builds/build-1/relationships/betaGroups":
			f.postCount++
			status := f.postStatus
			if status == 0 {
				status = http.StatusNoContent
			}
			return jsonResponse(status, f.postBody)
		case "GET /v1/builds/build-1":
			f.buildReads++
			status := f.buildStatus
			if status == 0 {
				status = http.StatusOK
			}
			return jsonResponse(status, f.buildBody)
		case "GET /v1/builds/build-1/buildBetaDetail":
			f.detailReads++
			status := f.detailStatus
			if status == 0 {
				status = http.StatusOK
			}
			return jsonResponse(status, f.detailBody)
		default:
			f.t.Fatalf("unexpected request %s", request)
			return nil, nil
		}
	}
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func runAddGroupsDiagnostics(t *testing.T, fixture *addGroupsDiagnosticsFixture) (string, string, error) {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = fixture.transport()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "add-groups",
			"--build-id", "build-1",
			"--group", "group-1",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	return stdout, stderr, runErr
}

func TestBuildsAddGroupsSuccessMakesNoDiagnosticReads(t *testing.T) {
	fixture := &addGroupsDiagnosticsFixture{t: t, external: true}
	stdout, stderr, runErr := runAddGroupsDiagnostics(t, fixture)
	if runErr != nil {
		t.Fatalf("unexpected error: %v (stderr=%q)", runErr, stderr)
	}
	if fixture.postCount != 1 || fixture.buildReads != 0 || fixture.detailReads != 0 {
		t.Fatalf("requests = %v, want one POST and no diagnostic reads", fixture.requests)
	}
	if !strings.Contains(stdout, `"groupIds":["group-1"]`) {
		t.Fatalf("stdout = %q, want assignment receipt", stdout)
	}
}

func TestBuildsAddGroupsDiagnoses422AfterAssignment(t *testing.T) {
	fixture := &addGroupsDiagnosticsFixture{
		t:          t,
		external:   true,
		postStatus: http.StatusUnprocessableEntity,
		postBody:   `{"errors":[{"status":"422","code":"STATE_ERROR.ENTITY_STATE_INVALID","detail":"The build is not ready.","meta":{"associatedErrors":{"betaGroups":[{"code":"BETA_GROUP_INVALID","detail":"The selected beta group is not eligible."}]}}}]}`,
		buildBody:  `{"data":{"type":"builds","id":"build-1","attributes":{"processingState":"FAILED","expired":false}}}`,
		detailBody: `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"externalBuildState":"MISSING_EXPORT_COMPLIANCE"}}}`,
	}
	_, stderr, runErr := runAddGroupsDiagnostics(t, fixture)
	if runErr == nil {
		t.Fatal("expected HTTP 422 failure")
	}
	wantRequests := []string{
		"GET /v1/builds/build-1/app",
		"GET /v1/apps/app-1/betaGroups",
		"POST /v1/builds/build-1/relationships/betaGroups",
		"GET /v1/builds/build-1",
		"GET /v1/builds/build-1/buildBetaDetail",
	}
	if !reflect.DeepEqual(fixture.requests, wantRequests) {
		t.Fatalf("requests = %v, want %v", fixture.requests, wantRequests)
	}
	if rootcmd.ExitCodeFromError(runErr) != rootcmd.HTTPStatusToExitCode(http.StatusUnprocessableEntity) {
		t.Fatalf("exit code = %d, want HTTP 422 mapping", rootcmd.ExitCodeFromError(runErr))
	}
	var apiErr *asc.APIError
	if !errors.As(runErr, &apiErr) || apiErr.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("error chain lost original API error: %v", runErr)
	}
	for _, want := range []string{
		"The build is not ready.",
		"The selected beta group is not eligible.",
		"Current build state: processingState=FAILED",
		"externalBuildState=MISSING_EXPORT_COMPLIANCE",
		`--ipa "PATH_TO_IPA"`,
		`--pkg "PATH_TO_PKG"`,
		"If the app does not use non-exempt encryption",
		`--uses-non-exempt-encryption=false`,
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want %q", stderr, want)
		}
	}
	if strings.Contains(stderr, "--file") {
		t.Fatalf("stderr contains unsupported --file guidance: %q", stderr)
	}
	if codeAt, associatedAt := strings.Index(stderr, "(STATE_ERROR.ENTITY_STATE_INVALID)"), strings.Index(stderr, "Associated errors"); codeAt < 0 || associatedAt < 0 || codeAt > associatedAt {
		t.Fatalf("top-level code must precede associated errors: %q", stderr)
	}
}

func TestBuildsAddGroupsNon422DoesNotReadDiagnostics(t *testing.T) {
	fixture := &addGroupsDiagnosticsFixture{
		t:          t,
		external:   true,
		postStatus: http.StatusConflict,
		postBody:   `{"errors":[{"status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","detail":"Conflict."}]}`,
	}
	_, _, runErr := runAddGroupsDiagnostics(t, fixture)
	if runErr == nil {
		t.Fatal("expected HTTP 409 failure")
	}
	if fixture.postCount != 1 || fixture.buildReads != 0 || fixture.detailReads != 0 {
		t.Fatalf("requests = %v, want one POST and no diagnostic reads", fixture.requests)
	}
	var apiErr *asc.APIError
	if !errors.As(runErr, &apiErr) || apiErr.StatusCode != http.StatusConflict {
		t.Fatalf("error chain lost original API error: %v", runErr)
	}
}

func TestBuildsAddGroupsDiagnosticReadFailurePreserves422(t *testing.T) {
	fixture := &addGroupsDiagnosticsFixture{
		t:           t,
		external:    false,
		postStatus:  http.StatusUnprocessableEntity,
		postBody:    `{"errors":[{"status":"422","code":"STATE_ERROR.ENTITY_STATE_INVALID","detail":"The build is not ready."}]}`,
		buildStatus: http.StatusInternalServerError,
		buildBody:   `{"errors":[{"status":"500","code":"UNEXPECTED_ERROR","detail":"Unavailable."}]}`,
	}
	_, stderr, runErr := runAddGroupsDiagnostics(t, fixture)
	if runErr == nil {
		t.Fatal("expected HTTP 422 failure")
	}
	if fixture.postCount != 1 || fixture.buildReads != 1 || fixture.detailReads != 0 {
		t.Fatalf("requests = %v, want one POST, one build read, and no external detail read", fixture.requests)
	}
	if rootcmd.ExitCodeFromError(runErr) != rootcmd.HTTPStatusToExitCode(http.StatusUnprocessableEntity) {
		t.Fatalf("exit code = %d, want original HTTP 422 mapping (stderr=%q)", rootcmd.ExitCodeFromError(runErr), stderr)
	}
	var apiErr *asc.APIError
	if !errors.As(runErr, &apiErr) || apiErr.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("error chain lost original API error: %v", runErr)
	}
}
