package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

func TestVideoPreviewsUploadIPhoneDuoCreatesExactPreviewSet(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	path := filepath.Join(t.TempDir(), "01-first.mov")
	size := writePreviewFile(t, path)
	oldTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = oldTransport })
	fallback := previewUploadTransport(t, map[string]int64{"01-first.mov": size}, []string{"preview-first"}, func(ids []string) { t.Fatalf("unexpected order change: %v", ids) })
	createdSet := false
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appPreviewSets":
			return statusJSONResponse(`{"data":[],"links":{}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appPreviewSets":
			var payload asc.AppPreviewSetCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if payload.Data.Type != "appPreviewSets" || payload.Data.Attributes.PreviewType != "IPHONE_DUO" {
				t.Fatalf("unexpected create payload: %+v", payload)
			}
			owner := payload.Data.Relationships.AppStoreVersionLocalization
			if owner == nil || owner.Data.Type != "appStoreVersionLocalizations" || owner.Data.ID != "LOC_123" {
				t.Fatalf("unexpected set owner: %+v", owner)
			}
			createdSet = true
			return statusJSONResponse(`{"data":{"type":"appPreviewSets","id":"set-1","attributes":{"previewType":"IPHONE_DUO"}}}`), nil
		default:
			return fallback(req)
		}
	})
	stdout, stderr, err := runRootCommand(t, []string{"video-previews", "upload", "--version-localization", "LOC_123", "--path", path, "--device-type", "IPHONE_DUO", "--output", "json"})
	if err != nil {
		t.Fatalf("upload: %v; stderr: %s", err, stderr)
	}
	if !createdSet {
		t.Fatal("Duo preview set was not created")
	}
	if !strings.Contains(stdout, `"previewType":"IPHONE_DUO"`) || !strings.Contains(stdout, `"state":"COMPLETE"`) {
		t.Fatalf("unexpected receipt: %s", stdout)
	}
}
