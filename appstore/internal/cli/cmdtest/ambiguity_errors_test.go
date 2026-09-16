package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
)

// These tests pin the ambiguity error contract end to end: when a selector
// matches several resources the command exits with its usual code and stderr
// lists every candidate with its ID and the flag that accepts one of them.

func assertStderrLines(t *testing.T, stderr string, wantLines ...string) {
	t.Helper()
	for _, want := range wantLines {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr missing %q:\n%s", want, stderr)
		}
	}
}

func TestAmbiguousAppNameListsCandidatesAndAppFlag(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		if req.URL.Query().Get("filter[bundleId]") != "" {
			return jsonHTTPResponse(http.StatusOK, `{"data":[]}`), nil
		}
		return jsonHTTPResponse(http.StatusOK, `{"data":[
			{"type":"apps","id":"app-2","attributes":{"name":"Ambiguous App","bundleId":"com.example.two"}},
			{"type":"apps","id":"app-1","attributes":{"name":"Ambiguous App","bundleId":"com.example.one"}}
		]}`), nil
	}))

	stdout, stderr := captureOutput(t, func() {
		if code := cmd.Run([]string{"builds", "list", "--app", "Ambiguous App"}, "1.2.3"); code != cmd.ExitError {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitError)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	assertStderrLines(
		t, stderr,
		`2 apps match "Ambiguous App"; pass --app with one of:`,
		"\n  app-1  Ambiguous App  com.example.one\n  app-2  Ambiguous App  com.example.two\n",
	)
}

func TestAmbiguousBetaGroupNameListsCandidatesAndGroupFlag(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/betaGroups" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return jsonHTTPResponse(http.StatusOK, `{"data":[
			{"type":"betaGroups","id":"group-internal","attributes":{"name":"Beta","isInternalGroup":true}},
			{"type":"betaGroups","id":"group-external","attributes":{"name":"Beta","isInternalGroup":false}}
		],"links":{}}`), nil
	}))

	stdout, stderr := captureOutput(t, func() {
		if code := cmd.Run([]string{"testflight", "testers", "add", "--app", "app-1", "--email", "tester@example.com", "--group", "Beta"}, "1.2.3"); code == cmd.ExitSuccess {
			t.Fatal("expected ambiguity failure")
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	assertStderrLines(
		t, stderr,
		`2 groups match "Beta"; pass --group with one of:`,
		"\n  group-internal  Beta  internal\n  group-external  Beta  external\n",
	)
}

func TestAmbiguousSandboxTesterEmailListsCandidatesAndIDFlag(t *testing.T) {
	setupAuth(t)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v2/sandboxTesters" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return jsonHTTPResponse(http.StatusOK, `{"data":[
			{"type":"sandboxTesters","id":"tester-1","attributes":{"email":"dup@example.com","firstName":"Jane","lastName":"Doe"}},
			{"type":"sandboxTesters","id":"tester-2","attributes":{"email":"dup@example.com","firstName":"John","lastName":"Doe"}}
		],"links":{}}`), nil
	}))

	stdout, stderr := captureOutput(t, func() {
		if code := cmd.Run([]string{"sandbox", "view", "--email", "dup@example.com"}, "1.2.3"); code != cmd.ExitError {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitError)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	assertStderrLines(
		t, stderr,
		`2 sandbox testers match email "dup@example.com"; pass --id with one of:`,
		"\n  tester-1\n  tester-2\n",
	)
	for _, privateValue := range []string{"Jane Doe", "John Doe"} {
		if strings.Contains(stderr, privateValue) {
			t.Fatalf("stderr must not expose tester name %q: %s", privateValue, stderr)
		}
	}
}

func TestBuildsTestNotesUpdateByBuildLocaleAmbiguousListsCandidates(t *testing.T) {
	setupAuth(t)
	requests := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		switch requests {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1" {
				t.Fatalf("unexpected build request: %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`), nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/betaBuildLocalizations" {
				t.Fatalf("unexpected localization request: %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[
				{"type":"betaBuildLocalizations","id":"loc-1","attributes":{"locale":"en-US"}},
				{"type":"betaBuildLocalizations","id":"loc-2","attributes":{"locale":"en-US"}}
			]}`), nil
		default:
			t.Fatalf("unexpected request after ambiguity: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"builds", "test-notes", "update",
			"--build-id", "build-1",
			"--locale", "en-US",
			"--whats-new", "Updated notes",
		}, "1.2.3")
		if code != cmd.ExitError {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitError)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	assertStderrLines(
		t, stderr,
		`2 build localizations match build "build-1" and locale "en-US"; pass --localization-id with one of:`,
		"\n  loc-1  en-US\n  loc-2  en-US\n",
	)
	if requests != 2 {
		t.Fatalf("requests = %d, want 2 (no PATCH)", requests)
	}
}
