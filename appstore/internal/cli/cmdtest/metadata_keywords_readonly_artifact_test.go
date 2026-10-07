package cmdtest

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
)

func TestKeywordArtifactFailureKeepsBatchOutcome(t *testing.T) {
	for _, tc := range []struct {
		operation string
		readOnly  bool
	}{
		{"push", true}, {"apply", true}, {"sync", true}, {"push", false}, {"apply", false}, {"sync", false},
	} {
		operation := tc.operation
		mode := "api-error"
		if tc.readOnly {
			mode = "read-only"
		}
		t.Run(operation+"/"+mode, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_STOREKIT_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_ADS_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_READ_ONLY", "")
			if tc.readOnly {
				t.Setenv("ASC_READ_ONLY", "1")
			}
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
			workDir := t.TempDir()
			t.Chdir(workDir)
			// Deterministic ENOTDIR, independent of user privileges/platform modes.
			if err := os.WriteFile(".asc", []byte("block report directory"), 0o600); err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(workDir, "metadata")
			versionDir := filepath.Join(dir, "version", "1.2.3")
			if err := os.MkdirAll(versionDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(versionDir, "en-US.json"), []byte(`{"keywords":"alpha,beta"}`), 0o600); err != nil {
				t.Fatal(err)
			}
			input := filepath.Join(workDir, "keywords.json")
			if err := os.WriteFile(input, []byte(`{"en-US":"alpha,beta"}`), 0o600); err != nil {
				t.Fatal(err)
			}
			reads, mutations := 0, 0
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					mutations++
					if tc.readOnly || req.Method != http.MethodPatch || req.URL.Path != "/v1/appStoreVersionLocalizations/loc-en" {
						t.Errorf("unexpected mutation: %s %s", req.Method, req.URL.Path)
						return nil, io.ErrUnexpectedEOF
					}
					return jsonResponse(http.StatusBadRequest, `{"errors":[{"status":"400","code":"ENTITY_ERROR","title":"test API failure"}]}`)
				}
				reads++
				switch req.URL.Path {
				case "/v1/apps/app-1/appStoreVersions":
					return metadataKeywordsJSONResponse(`{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`)
				case "/v1/appStoreVersions/version-1":
					return metadataKeywordsJSONResponse(`{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`)
				case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
					return metadataKeywordsJSONResponse(`{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","keywords":"old,keywords"}}],"links":{"next":""}}`)
				default:
					t.Errorf("unexpected GET: %s", req.URL.Path)
					return nil, io.ErrUnexpectedEOF
				}
			}))
			args := []string{"metadata", "keywords", operation, "--output", "json"}
			if operation == "push" {
				args = append(args, "--version-id", "version-1", "--input", input)
			} else {
				args = append(args, "--app", "app-1", "--version", "1.2.3", "--dir", dir, "--confirm")
				if operation == "sync" {
					args = append(args, "--input", input, "--format", "json")
				}
			}
			code := -1
			stdout, stderr := captureOutput(t, func() { code = cmd.Run(args, "test") })
			wantMutations := 1
			if tc.readOnly {
				wantMutations = 0
			}
			if reads == 0 || mutations != wantMutations {
				t.Fatalf("reads=%d mutations=%d want %d", reads, mutations, wantMutations)
			}
			if !strings.Contains(stderr, "write failure artifact") {
				t.Fatalf("did not reach artifact failure: %q", stderr)
			}
			raw := json.RawMessage(stdout)
			if operation == "sync" {
				var composite struct {
					Plan json.RawMessage `json:"plan"`
				}
				if err := json.Unmarshal(raw, &composite); err != nil {
					t.Fatalf("decode sync receipt: %v", err)
				}
				raw = composite.Plan
			}
			var receipt struct {
				Failed  int `json:"failed"`
				Results []struct {
					Status string `json:"status"`
					Error  string `json:"error"`
				} `json:"results"`
			}
			if err := json.Unmarshal(raw, &receipt); err != nil {
				t.Fatalf("decode failure receipt: %v", err)
			}
			expectedError := "test API failure"
			if tc.readOnly {
				expectedError = "ASC_READ_ONLY is set; refusing"
			}
			if receipt.Failed != 1 || len(receipt.Results) != 1 || receipt.Results[0].Status != "failed" || !strings.Contains(receipt.Results[0].Error, expectedError) {
				t.Fatalf("missing failed row receipt: %s", raw)
			}
			if strings.Count(stderr, "Error:") != 1 {
				t.Fatalf("expected one artifact diagnostic: %q", stderr)
			}
			wantExit := cmd.ExitError
			if tc.readOnly {
				wantExit = cmd.ExitReadOnly
			}
			if code != wantExit {
				t.Fatalf("exit=%d want %d despite artifact failure", code, wantExit)
			}
		})
	}
}
