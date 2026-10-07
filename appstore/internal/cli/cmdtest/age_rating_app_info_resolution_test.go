package cmdtest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAgeRatingViewResolvesAppInfoWhileNewVersionIsPrepared(t *testing.T) {
	tests := []struct {
		name         string
		appInfos     string
		wantStdout   string
		wantStderr   string
		wantErrParts []string
	}{
		{
			name:       "selects the editable app info",
			appInfos:   `{"data":[{"type":"appInfos","id":"info-live","attributes":{"state":"READY_FOR_DISTRIBUTION"}},{"type":"appInfos","id":"info-edit","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}],"links":{}}`,
			wantStdout: `"id":"decl-info-edit"`,
			wantStderr: "auto-selected info-edit (PREPARE_FOR_SUBMISSION)",
		},
		{
			name:         "names --app-info-id when no editable app info exists",
			appInfos:     `{"data":[{"type":"appInfos","id":"info-live","attributes":{"state":"READY_FOR_DISTRIBUTION"}},{"type":"appInfos","id":"info-review","attributes":{"state":"WAITING_FOR_REVIEW"}}],"links":{}}`,
			wantErrParts: []string{"pass --app-info-id with one of:", "info-live", "info-review"},
		},
		{
			name:       "ignores historical app infos",
			appInfos:   `{"data":[{"type":"appInfos","id":"info-old","attributes":{"state":"REPLACED_WITH_NEW_INFO"}},{"type":"appInfos","id":"info-live","attributes":{"state":"READY_FOR_DISTRIBUTION"}}],"links":{}}`,
			wantStdout: `"id":"decl-info-live"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				appInfoID := strings.TrimSuffix(strings.TrimPrefix(req.URL.Path, "/v1/appInfos/"), "/ageRatingDeclaration")
				switch req.URL.Path {
				case "/v1/apps/app-1/appInfos":
					return jsonHTTPResponse(http.StatusOK, test.appInfos), nil
				case "/v1/appInfos/" + appInfoID + "/ageRatingDeclaration":
					return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"decl-`+appInfoID+`","attributes":{}}}`), nil
				default:
					return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
				}
			}))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{"age-rating", "view", "--app", "app-1", "--output", "json"}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if len(test.wantErrParts) > 0 {
				if runErr == nil {
					t.Fatalf("expected error, got stdout %q", stdout)
				}
				for _, part := range test.wantErrParts {
					if !strings.Contains(runErr.Error(), part) {
						t.Fatalf("error %q missing %q", runErr.Error(), part)
					}
				}
				return
			}
			if runErr != nil {
				t.Fatalf("run error: %v", runErr)
			}
			if !strings.Contains(stdout, test.wantStdout) {
				t.Fatalf("stdout %q missing %q", stdout, test.wantStdout)
			}
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("stderr %q missing %q", stderr, test.wantStderr)
			}
		})
	}
}
