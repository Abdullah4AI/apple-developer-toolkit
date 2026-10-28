package cmdtest

import (
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
)

func TestRun_BuildsInfoInclude(t *testing.T) {
	const included = `[{"type":"buildBetaDetails","id":"bbd-1","attributes":{"autoNotifyEnabled":false,"internalBuildState":"READY_FOR_BETA_TESTING","externalBuildState":"READY_FOR_BETA_SUBMISSION"}}]`
	detailBody := `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}},"included":` + included + `}`
	listBody := `{"data":[{"type":"builds","id":"build-1","attributes":{"version":"42","uploadedDate":"2026-03-13T00:00:00Z","processingState":"VALID","expired":false}}],"links":{}}`

	tests := []struct {
		name         string
		args         []string
		wantCode     int
		wantRequests []string
		wantStderr   string
	}{
		{
			name:         "build id",
			args:         []string{"--build-id", "build-1", "--include", "buildBetaDetail,betaGroups"},
			wantCode:     cmd.ExitSuccess,
			wantRequests: []string{"/v1/builds/build-1 include=preReleaseVersion,buildBetaDetail,betaGroups"},
		},
		{
			name:     "app build number",
			args:     []string{"--app", "123456789", "--build-number", "42", "--platform", "IOS", "--include", "buildBetaDetail"},
			wantCode: cmd.ExitSuccess,
			wantRequests: []string{
				"/v1/builds",
				"/v1/builds/build-1 include=preReleaseVersion,buildBetaDetail",
			},
		},
		{
			name:       "unknown value",
			args:       []string{"--build-id", "build-1", "--include", "buildBetaDetail,bogus"},
			wantCode:   cmd.ExitUsage,
			wantStderr: `unknown value "bogus"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var requests []string
			transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					t.Fatalf("expected GET, got %s %s", req.Method, req.URL.Path)
				}
				body := emptyCollectionBody
				switch req.URL.Path {
				case "/v1/builds":
					requests = append(requests, req.URL.Path)
					body = listBody
				case "/v1/builds/build-1":
					requests = append(requests, req.URL.Path+" include="+req.URL.Query().Get("include"))
					body = detailBody
				case "/v1/preReleaseVersions":
				default:
					t.Fatalf("unexpected request %s", req.URL.String())
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			})

			code, stdout, stderr := runBuildsInfoSelector(t, transport, append(test.args, "--output", "json")...)

			if code != test.wantCode {
				t.Fatalf("exit code = %d, want %d (stderr %q)", code, test.wantCode, stderr)
			}
			if !reflect.DeepEqual(requests, test.wantRequests) {
				t.Fatalf("requests = %q, want %q", requests, test.wantRequests)
			}
			if test.wantStderr != "" {
				if !strings.Contains(stderr, test.wantStderr) {
					t.Fatalf("stderr = %q, want it to contain %q", stderr, test.wantStderr)
				}
				return
			}

			var got struct {
				Included any `json:"included"`
			}
			if err := json.Unmarshal([]byte(stdout), &got); err != nil {
				t.Fatalf("decode stdout %q: %v", stdout, err)
			}
			var wantIncluded any
			if err := json.Unmarshal([]byte(included), &wantIncluded); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Included, wantIncluded) {
				t.Fatalf("included = %#v, want %s", got.Included, included)
			}
		})
	}
}
