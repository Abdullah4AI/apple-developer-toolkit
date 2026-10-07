package cmdtest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	rootcmd "github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
)

func TestCustomPagePreviewsUploadReportsReceiptsWhenALaterFileFails(t *testing.T) {
	for _, tc := range []struct {
		name, command, output                                             string
		newSet, firstFailure, rollbackFailure, setCleanupFailure, success bool
	}{
		{name: "upload", command: "upload", output: "json"},
		{name: "sync", command: "sync", output: "json"},
		{name: "table", command: "upload", output: "table"},
		{name: "markdown", command: "upload", output: "markdown"},
		{name: "first failure existing set", command: "upload", output: "json", firstFailure: true},
		{name: "first failure new set", command: "upload", output: "json", newSet: true, firstFailure: true},
		{name: "new set cleanup", command: "upload", output: "json", newSet: true},
		{name: "orphan reservations", command: "upload", output: "json", newSet: true, rollbackFailure: true},
		{name: "orphan set", command: "upload", output: "json", newSet: true, setCleanupFailure: true},
		{name: "success", command: "upload", output: "json", success: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			t.Setenv("ASC_APP_ID", "")
			dir := t.TempDir()
			sizes := map[string]int64{
				"preview-first":  writePreviewFile(t, filepath.Join(dir, "01-first.mov")),
				"preview-second": writePreviewFile(t, filepath.Join(dir, "02-second.mov")),
			}
			var deletions []string
			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			conflict := func(detail string) *http.Response {
				return &http.Response{StatusCode: http.StatusConflict, Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"errors":[{"status":"409","code":"ENTITY_ERROR.CONFLICT","detail":%q}]}`, detail))), Header: http.Header{}}
			}
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/appCustomProductPageLocalizations/LOC_123/appPreviewSets":
					if tc.newSet {
						return statusJSONResponse(`{"data":[],"links":{}}`), nil
					}
					return statusJSONResponse(`{"data":[{"type":"appPreviewSets","id":"set-1","attributes":{"previewType":"IPHONE_65"}}],"links":{}}`), nil
				case req.Method == http.MethodPost && req.URL.Path == "/v1/appPreviewSets":
					if !tc.newSet {
						t.Fatal("must preserve pre-existing set")
					}
					return statusJSONResponse(`{"data":{"type":"appPreviewSets","id":"set-1","attributes":{"previewType":"IPHONE_65"}}}`), nil
				case req.Method == http.MethodGet && req.URL.Path == "/v1/appPreviewSets/set-1/appPreviews":
					return statusJSONResponse(`{"data":[{"type":"appPreviews","id":"old-1","attributes":{"fileName":"old.mov"}}],"links":{}}`), nil
				case req.Method == http.MethodPost && req.URL.Path == "/v1/appPreviews":
					if tc.firstFailure {
						return conflict("preview reservation conflict"), nil
					}
					body, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatal(err)
					}
					id := "preview-first"
					if strings.Contains(string(body), "02-second.mov") {
						id = "preview-second"
					}
					return statusJSONResponse(fmt.Sprintf(`{"data":{"type":"appPreviews","id":%q,"attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/%s","length":%d,"offset":0}]}}}`, id, id, sizes[id])), nil
				case req.Method == http.MethodPut && req.URL.Host == "upload.example":
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
				case req.Method == http.MethodPatch && req.URL.Path == "/v1/appPreviews/preview-second" && !tc.success:
					return conflict("preview commit conflict"), nil
				case (req.Method == http.MethodPatch || req.Method == http.MethodGet) && strings.HasPrefix(req.URL.Path, "/v1/appPreviews/preview-"):
					id := strings.TrimPrefix(req.URL.Path, "/v1/appPreviews/")
					return statusJSONResponse(fmt.Sprintf(`{"data":{"type":"appPreviews","id":%q,"attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`, id)), nil
				case req.Method == http.MethodDelete:
					deletions = append(deletions, req.URL.Path)
					switch req.URL.Path {
					case "/v1/appPreviews/old-1":
						if tc.command != "sync" {
							t.Fatal("must not delete pre-existing preview on upload")
						}
					case "/v1/appPreviewSets/set-1":
						if !tc.newSet {
							t.Fatal("must not delete pre-existing set")
						}
						if tc.setCleanupFailure {
							return conflict("set cleanup conflict"), nil
						}
					case "/v1/appPreviews/preview-first", "/v1/appPreviews/preview-second":
						if tc.rollbackFailure {
							return conflict("preview cleanup conflict"), nil
						}
					default:
						t.Fatalf("unexpected deletion %s", req.URL.Path)
					}
					return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			})
			args := []string{"product-pages", "custom-pages", "localizations", "preview-sets", tc.command, "--localization-id", "LOC_123", "--path", dir, "--device-type", "IPHONE_65", "--output", tc.output}
			if tc.command == "sync" {
				args = append(args, "--confirm")
			}
			var exitCode int
			stdout, stderr := captureOutput(t, func() { exitCode = rootcmd.Run(args, "1.2.3") })
			wantDeletions := []string(nil)
			if tc.command == "sync" {
				wantDeletions = append(wantDeletions, "/v1/appPreviews/old-1")
			}
			if !tc.success {
				cause := "preview commit conflict"
				if tc.firstFailure {
					cause = "preview reservation conflict"
				}
				if exitCode == rootcmd.ExitSuccess || strings.Count(stderr, cause) != 1 {
					t.Fatalf("failure must exit nonzero and print cause once: code=%d stderr=%q", exitCode, stderr)
				}
				if !tc.firstFailure {
					wantDeletions = append(wantDeletions, "/v1/appPreviews/preview-second", "/v1/appPreviews/preview-first")
				}
				if tc.newSet && !tc.rollbackFailure {
					wantDeletions = append(wantDeletions, "/v1/appPreviewSets/set-1")
				}
			} else if exitCode != rootcmd.ExitSuccess || stderr != "" {
				t.Fatalf("success: code=%d stderr=%q", exitCode, stderr)
			}
			if !reflect.DeepEqual(deletions, wantDeletions) {
				t.Fatalf("deletions=%v want=%v", deletions, wantDeletions)
			}
			if tc.output != "json" {
				for _, text := range []string{"preview-first", "preview-second", "rolled-back", "Error", "preview commit conflict"} {
					if !strings.Contains(stdout, text) {
						t.Fatalf("%s missing %q: %s", tc.output, text, stdout)
					}
				}
				if tc.output == "markdown" && !strings.Contains(stdout, "|:") {
					t.Fatalf("missing failure table: %s", stdout)
				}
				return
			}
			var payload struct {
				LocalizationID string                                       `json:"customProductPageLocalizationId"`
				SetID          string                                       `json:"setId"`
				Results        []struct{ FileName, AssetID, State string }  `json:"results"`
				Failures       []struct{ FileName, FilePath, Error string } `json:"failures"`
			}
			if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
				t.Fatalf("decode partial receipt: %v stdout=%s", err, stdout)
			}
			if payload.LocalizationID != "LOC_123" || payload.SetID != "set-1" {
				t.Fatalf("receipt=%s", stdout)
			}
			if tc.success {
				var keys map[string]json.RawMessage
				if err := json.Unmarshal([]byte(stdout), &keys); err != nil {
					t.Fatal(err)
				}
				if len(keys) != 4 || keys["failures"] != nil || len(payload.Results) != 2 {
					t.Fatalf("success JSON changed: %s", stdout)
				}
				return
			}
			failureName := "02-second.mov"
			wantState := "rolled-back"
			wantResults := 2
			if tc.firstFailure {
				failureName = "01-first.mov"
				wantState = "failed"
				wantResults = 1
			}
			if tc.rollbackFailure {
				wantState = "rollback-failed"
			}
			if len(payload.Results) != wantResults || len(payload.Failures) != 1 || payload.Failures[0].FileName != failureName || !strings.HasSuffix(payload.Failures[0].FilePath, failureName) {
				t.Fatalf("receipt=%s", stdout)
			}
			for i, item := range payload.Results {
				if item.State != wantState {
					t.Fatalf("receipt state=%s", stdout)
				}
				if !tc.firstFailure && item.AssetID != []string{"preview-first", "preview-second"}[i] {
					t.Fatalf("orphan ID missing: %s", stdout)
				}
			}
			if tc.rollbackFailure && !strings.Contains(payload.Failures[0].Error, "preview cleanup conflict") {
				t.Fatalf("cleanup cause missing: %s", stdout)
			}
			if tc.setCleanupFailure && !strings.Contains(payload.Failures[0].Error, "set cleanup conflict") {
				t.Fatalf("set cleanup cause missing: %s", stdout)
			}
		})
	}
}
