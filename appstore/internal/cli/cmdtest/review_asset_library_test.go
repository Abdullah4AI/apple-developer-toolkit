package cmdtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
)

func TestAssetLibraryReviewItemsAdd(t *testing.T) {
	for _, kind := range []string{"Image", "Video"} {
		t.Run(kind, func(t *testing.T) {
			setupSubmitCreateAuth(t)
			key := "appAssetLibrary" + kind
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.Path != "/v1/reviewSubmissionItems" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
					w.WriteHeader(500)
					return
				}
				assertJSONDocument(t, r.Body, fmt.Sprintf(`{"data":{"type":"reviewSubmissionItems","relationships":{"reviewSubmission":{"data":{"type":"reviewSubmissions","id":"sub-1"}},%q:{"data":{"type":%q,"id":"asset-1"}}}}}`, key, key+"s"))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(201)
				fmt.Fprintf(w, `{"data":{"type":"reviewSubmissionItems","id":"item-1","relationships":{%q:{"data":{"type":%q,"id":"asset-1"}}}}}`, key, key+"s")
			}))
			defer server.Close()
			setReviewItemsTestServerClient(t, server)
			out, errout := captureOutput(t, func() {
				if code := cmd.Run([]string{"review", "items", "add", "--submission", "sub-1", "--item-type", key + "s", "--item-id", "asset-1", "--output", "json"}, "dev"); code != cmd.ExitSuccess {
					t.Errorf("exit=%d", code)
				}
			})
			if errout != "" || !strings.Contains(out, key) || !strings.Contains(out, `"id":"item-1"`) {
				t.Fatalf("stdout=%s stderr=%s", out, errout)
			}
		})
	}
}

func TestAssetLibraryReviewItemsListAndHistory(t *testing.T) {
	for _, mode := range []string{"json", "table", "markdown", "history"} {
		t.Run(mode, func(t *testing.T) {
			setupSubmitCreateAuth(t)
			old := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = old })
			http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != "GET" {
					t.Fatalf("unexpected method %s", r.Method)
				}
				if r.URL.Path == "/v1/apps/app-1/reviewSubmissions" {
					return jsonResponse(200, `{"data":[{"type":"reviewSubmissions","id":"sub-1","attributes":{"state":"COMPLETE","platform":"IOS","submittedDate":"2026-10-01T12:00:00Z"}}],"links":{}}`)
				}
				if r.URL.Path != "/v1/reviewSubmissions/sub-1/items" {
					t.Fatalf("unexpected request %s", r.URL)
				}
				for _, key := range []string{"appAssetLibraryImage", "appAssetLibraryVideo"} {
					if !strings.Contains(r.URL.Query().Get("include"), key) || !strings.Contains(r.URL.Query().Get("fields[reviewSubmissionItems]"), key) {
						t.Errorf("query lacks %s: %s", key, r.URL)
					}
				}
				return jsonResponse(200, `{"data":[{"type":"reviewSubmissionItems","id":"item-1","attributes":{"state":"ACCEPTED"},"relationships":{"appAssetLibraryImage":{"data":{"type":"appAssetLibraryImages","id":"image-1"}}}},{"type":"reviewSubmissionItems","id":"item-2","attributes":{"state":"ACCEPTED"},"relationships":{"appAssetLibraryVideo":{"data":{"type":"appAssetLibraryVideos","id":"video-1"}}}}],"links":{},"included":[{"type":"appAssetLibraryImages","id":"image-1","attributes":{"future":"preserved"}}]}`)
			})
			args := []string{"review", "items-list", "--submission", "sub-1", "--fields", "state,appAssetLibraryImage,appAssetLibraryVideo", "--include", "appAssetLibraryImage,appAssetLibraryVideo", "--output", mode}
			if mode == "history" {
				args = []string{"review", "history", "--app", "app-1", "--output", "json"}
			}
			out, errout := captureOutput(t, func() {
				if code := cmd.Run(args, "dev"); code != cmd.ExitSuccess {
					t.Errorf("exit=%d", code)
				}
			})
			if errout != "" || !strings.Contains(out, "image-1") || !strings.Contains(out, "video-1") {
				t.Fatalf("stdout=%s stderr=%s", out, errout)
			}
			if mode == "json" {
				var document map[string]json.RawMessage
				if err := json.Unmarshal([]byte(out), &document); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(document["included"]), "preserved") {
					t.Fatal("included lost")
				}
			}
			if mode == "history" && (!strings.Contains(out, `"type":"appAssetLibraryImage"`) || !strings.Contains(out, `"type":"appAssetLibraryVideo"`)) {
				t.Fatalf("history targets missing: %s", out)
			}
		})
	}
}

func TestAssetLibraryReviewItemsSkipRequiresValidLinkage(t *testing.T) {
	for _, kind := range []string{"Image", "Video"} {
		for _, linkage := range []string{"valid", "wrong-type", "wrong-id", "null", "removed"} {
			t.Run(kind+"/"+linkage, func(t *testing.T) {
				key := "appAssetLibrary" + kind
				out, _, seen, err := runIfExistsCommand(t, []string{"review", "items-add", "--submission", "sub-1", "--item-type", key + "s", "--item-id", "asset-1", "--if-exists", "skip", "--output", "json"}, func(r ifExistsRequest) (*http.Response, error) {
					if r.Method == "POST" {
						return jsonResponse(409, reviewItemExists409)
					}
					q, _ := url.ParseQuery(r.Query)
					if q.Get("include") != key {
						t.Errorf("include=%s", r.Query)
					}
					state := "READY_FOR_REVIEW"
					typ := key + "s"
					if linkage == "wrong-type" {
						typ = "appStoreVersions"
					}
					if linkage == "removed" {
						state = "REMOVED"
					}
					id := "asset-1"
					if linkage == "wrong-id" {
						id = "another-asset"
					}
					data := fmt.Sprintf(`{"type":%q,"id":%q}`, typ, id)
					if linkage == "null" {
						data = "null"
					}
					return jsonResponse(200, fmt.Sprintf(`{"data":[{"type":"reviewSubmissionItems","id":"item-1","attributes":{"state":%q},"relationships":{%q:{"data":%s}}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/reviewSubmissions/sub-1/items"}}`, state, key, data))
				})
				if len(seen) != 2 {
					t.Fatalf("requests=%+v", seen)
				}
				if linkage == "valid" {
					if err != nil || !strings.Contains(out, "asset-1") {
						t.Fatalf("err=%v stdout=%s", err, out)
					}
				} else if err == nil {
					t.Fatalf("invalid %s reused: %s", linkage, out)
				}
			})
		}
	}
}

func TestAssetLibraryReviewUnknownTypeBeforeAuth(t *testing.T) {
	assertUsageExit(t, []string{"review", "items-add", "--submission", "sub-1", "--item-type", "appAssetLibraryMedia", "--item-id", "asset-1"}, "--item-type must be one of")
}
