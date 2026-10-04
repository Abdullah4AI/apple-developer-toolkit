package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
)

const dsymErrorPrefix = "builds dsyms: "

func TestBuildsDSYMNoLiveVersionErrorNamesTheCommandOnce(t *testing.T) {
	setupAuth(t)
	restoreTransport(t)
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/apps/123456789/appStoreVersions" {
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
		}
		return dsymJSON(`{"data":[],"links":{}}`), nil
	})

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"builds", "dsyms", "--app", "123456789", "--version", "live", "--output-dir", t.TempDir()}, "1.2.3")
	})
	if code != rootcmd.ExitError {
		t.Fatalf("exit code = %d, want %d; stderr = %q", code, rootcmd.ExitError, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if want := "Error: builds dsyms: no live App Store version\n"; stderr != want {
		t.Fatalf("stderr = %q, want exactly %q", stderr, want)
	}
}

// TestBuildsDSYMErrorsNameTheCommandOnce covers every failure path that
// crosses the command's error wrapping, including errors raised after an inner
// step already named the command and errors from helpers that do not.
func TestBuildsDSYMErrorsNameTheCommandOnce(t *testing.T) {
	forbidden := func() (*http.Response, error) {
		return jsonResponse(http.StatusForbidden, apiErrorJSONForStatus(http.StatusForbidden))
	}
	emptyBuilds := `{"data":[],"included":[],"links":{}}`

	tests := []struct {
		name      string
		args      []string
		respond   func(req *http.Request) (*http.Response, error)
		wantExact string
	}{
		{
			name: "no builds match a range",
			args: []string{"--app", "123456789", "--min-version", "9.0"},
			respond: func(req *http.Request) (*http.Response, error) {
				if req.URL.Path == "/v1/builds" {
					return dsymJSON(emptyBuilds), nil
				}
				return nil, nil
			},
			wantExact: "builds dsyms: no builds matched",
		},
		{
			name: "live version lookup fails",
			args: []string{"--app", "123456789", "--version", "live"},
			respond: func(req *http.Request) (*http.Response, error) {
				if req.URL.Path == "/v1/apps/123456789/appStoreVersions" {
					return forbidden()
				}
				return nil, nil
			},
		},
		{
			name: "build listing fails",
			args: []string{"--app", "123456789", "--min-version", "1.0"},
			respond: func(req *http.Request) (*http.Response, error) {
				if req.URL.Path == "/v1/builds" {
					return forbidden()
				}
				return nil, nil
			},
		},
		{
			name: "build lookup fails",
			args: []string{"--build-id", "build-1"},
			respond: func(req *http.Request) (*http.Response, error) {
				if req.URL.Path == "/v1/builds/build-1" {
					return forbidden()
				}
				return nil, nil
			},
		},
		{
			name: "bundle lookup fails",
			args: []string{"--build-id", "build-1"},
			respond: func(req *http.Request) (*http.Response, error) {
				if req.URL.Path != "/v1/builds/build-1" {
					return nil, nil
				}
				if req.URL.Query().Get("include") == "buildBundles" {
					return forbidden()
				}
				return dsymJSON(`{"data":{"type":"builds","id":"build-1","attributes":{"version":"7"}}}`), nil
			},
		},
		{
			name: "bundle lookup fails while waiting",
			args: []string{"--build-id", "build-1", "--wait", "--poll-interval", "1ms", "--timeout", "5s"},
			respond: func(req *http.Request) (*http.Response, error) {
				if req.URL.Path != "/v1/builds/build-1" {
					return nil, nil
				}
				if req.URL.Query().Get("include") == "buildBundles" {
					return forbidden()
				}
				return dsymJSON(`{"data":{"type":"builds","id":"build-1","attributes":{"version":"7"}}}`), nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			restoreTransport(t)
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				resp, err := test.respond(req)
				if resp == nil && err == nil {
					t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
				}
				return resp, err
			})

			args := append([]string{"builds", "dsyms"}, test.args...)
			args = append(args, "--output-dir", filepath.Join(t.TempDir(), "dsyms"), "--output", "json")
			_, _, err := runDSYMErr(t, args...)
			if err == nil {
				t.Fatal("expected an error")
			}
			message := err.Error()
			if !strings.HasPrefix(message, dsymErrorPrefix) || strings.Count(message, dsymErrorPrefix) != 1 {
				t.Fatalf("error = %q, want exactly one %q prefix", message, dsymErrorPrefix)
			}
			if test.wantExact != "" && message != test.wantExact {
				t.Fatalf("error = %q, want %q", message, test.wantExact)
			}
		})
	}
}
