package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestTestFlightGroupsDestructiveCommandsPrintReceipts(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		method     string
		path       string
		wantStdout []string
		wantStderr string
	}{
		{
			name:       "delete json",
			args:       []string{"testflight", "groups", "delete", "--id", "group-1", "--confirm", "--output", "json"},
			method:     http.MethodDelete,
			path:       "/v1/betaGroups/group-1",
			wantStdout: []string{`{"id":"group-1","deleted":true}`},
			wantStderr: "Successfully deleted group group-1",
		},
		{
			name:       "delete table",
			args:       []string{"testflight", "groups", "delete", "--id", "group-1", "--confirm", "--output", "table"},
			method:     http.MethodDelete,
			path:       "/v1/betaGroups/group-1",
			wantStdout: []string{"ID", "Deleted", "group-1", "true"},
			wantStderr: "Successfully deleted group group-1",
		},
		{
			name:       "remove-testers json",
			args:       []string{"testflight", "groups", "remove-testers", "--group", "group-1", "--tester", "tester-1,tester-2", "--confirm", "--output", "json"},
			method:     http.MethodDelete,
			path:       "/v1/betaGroups/group-1/relationships/betaTesters",
			wantStdout: []string{`{"groupId":"group-1","testerIds":["tester-1","tester-2"],"action":"removed"}`},
			wantStderr: "Successfully removed 2 tester(s) from group group-1",
		},
		{
			name:       "remove-testers table",
			args:       []string{"testflight", "groups", "remove-testers", "--group", "group-1", "--tester", "tester-1,tester-2", "--confirm", "--output", "table"},
			method:     http.MethodDelete,
			path:       "/v1/betaGroups/group-1/relationships/betaTesters",
			wantStdout: []string{"Group ID", "Tester IDs", "Action", "group-1", "tester-1,tester-2", "removed"},
			wantStderr: "Successfully removed 2 tester(s) from group group-1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			originalTransport := http.DefaultTransport
			t.Cleanup(func() {
				http.DefaultTransport = originalTransport
			})
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != test.method || req.URL.Path != test.path {
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
				}
				return &http.Response{
					StatusCode: http.StatusNoContent,
					Body:       io.NopCloser(strings.NewReader("")),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
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

			for _, want := range test.wantStdout {
				if !strings.Contains(stdout, want) {
					t.Fatalf("expected stdout to contain %q, got %q", want, stdout)
				}
			}
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantStderr, stderr)
			}
		})
	}
}
