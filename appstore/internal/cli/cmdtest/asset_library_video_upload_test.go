package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared/errfmt"
)

func TestAssetLibraryVideoUpload(t *testing.T) { runLibraryVideoUploadCase(t, false, false, false) }
func TestAssetLibraryVideoUploadKeepsProcessingPartialReceipt(t *testing.T) {
	runLibraryVideoUploadCase(t, true, false, false)
}

func TestAssetLibraryVideoUploadObjectAspectRatio(t *testing.T) {
	runLibraryVideoUploadCase(t, false, true, false)
}

func TestAssetLibraryVideoUploadRequestDeadlineKeepsRequestHint(t *testing.T) {
	runLibraryVideoUploadCase(t, false, false, true)
}

func runLibraryVideoUploadCase(t *testing.T, pending, objectSpec, requestDeadline bool) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX ffprobe fixture")
	}
	setupAuth(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "video.mp4")
	content := []byte("bounded video fixture")
	if err := os.WriteFile(file, content, 0o600); err != nil {
		t.Fatal(err)
	}
	probe := `#!/bin/sh
printf '%s' '{"streams":[{"codec_type":"video","codec_name":"h264","width":1920,"height":1280,"avg_frame_rate":"30/1"}],"format":{"duration":"5","format_name":"mov,mp4,m4a,3gp,3g2,mj2","tags":{"major_brand":"isom"}}}'
`
	if objectSpec {
		probe = strings.ReplaceAll(probe, `"width":1920,"height":1280`, `"width":3840,"height":1646`)
	}
	if err := os.WriteFile(filepath.Join(dir, "ffprobe"), []byte(probe), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ASC_UPLOAD_TIMEOUT", "3s")
	if pending {
		t.Setenv("ASC_UPLOAD_TIMEOUT", "100ms")
	}
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	calls := []string{}
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls = append(calls, req.Method+" "+req.URL.Path)
		switch req.Method + " " + req.URL.Path {
		case "GET /v1/appAssetLibraryRefData":
			if objectSpec {
				return jsonResponse(200, `{"data":[{"attributes":{"videoSpecs":[{"dimensions":{"minWidth":3840,"maxWidth":3840,"minHeight":1646,"maxHeight":1646},"aspectRatio":{"width":21,"height":9},"compatiblePlacementTypes":["PRODUCT_PAGE_HEADER_ASSET"],"frameRates":[{"minFps":30,"maxFps":30}],"duration":{"min":"PT5S","max":"PT30S"},"fileExtensions":[".mp4"],"mimeTypes":["video/mp4"],"maxFileSize":524288000}]}}]}`)
			}
			return jsonResponse(200, `{"data":[{"attributes":{"videoSpecs":[{"dimensions":{"minWidth":1920,"maxWidth":3840,"minHeight":1280,"maxHeight":2560},"aspectRatio":"3:2","compatiblePlacementTypes":["APP_STORE_SEARCH_RESULTS_ASSET"],"frameRates":[{"minFps":30,"maxFps":30}],"duration":{"min":"PT5S","max":"PT30S"},"fileExtensions":[".mp4"],"mimeTypes":["video/mp4"],"maxFileSize":524288000}]}}]}`)
		case "POST /v1/appAssetLibraryVideos":
			if requestDeadline {
				return nil, context.DeadlineExceeded
			}
			var body map[string]any
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			data := body["data"].(map[string]any)
			attrs := data["attributes"].(map[string]any)
			if data["type"] != "appAssetLibraryVideos" || attrs["category"] != "CREATIVE_ASSETS" || attrs["fileName"] != "video.mp4" {
				t.Fatalf("reservation %+v", body)
			}
			return jsonResponse(201, `{"data":{"id":"video","type":"appAssetLibraryVideos","attributes":{"state":"AWAITING_UPLOAD","uploadOperations":[{"method":"PUT","url":"https://storage.example/video","offset":0,"length":21}]}}}`)
		case "PUT /video":
			bytes, err := io.ReadAll(req.Body)
			if err != nil || string(bytes) != string(content) || req.Header.Get("Authorization") != "" {
				t.Fatal("incorrect or credentialed transfer")
			}
			return jsonResponse(200, "")
		case "PATCH /v1/appAssetLibraryVideos/video":
			bytes, _ := io.ReadAll(req.Body)
			if string(bytes) != `{"data":{"type":"appAssetLibraryVideos","id":"video","attributes":{"uploaded":true}}}` {
				t.Fatalf("commit %s", bytes)
			}
			return jsonResponse(200, `{"data":{"id":"video","type":"appAssetLibraryVideos"}}`)
		case "GET /v1/appAssetLibraryVideos/video":
			if pending {
				return jsonResponse(200, `{"data":{"id":"video","type":"appAssetLibraryVideos","attributes":{"state":"PREPARE_FOR_SUBMISSION","specId":"spec","videoAsset":null,"previewFrameImage":{"state":"COMPLETE"}}}}`)
			}
			return jsonResponse(200, `{"data":{"id":"video","type":"appAssetLibraryVideos","attributes":{"state":"PREPARE_FOR_SUBMISSION","specId":"spec","videoAsset":"https://video.example/processed.mp4"}}}`)
		default:
			t.Fatalf("unexpected route %s %s", req.Method, req.URL.Path)
			return nil, context.Canceled
		}
	})
	stdout, stderr, err := runAssetLibrary(t, "asset-library", "videos", "upload", "--library-id", "library", "--file", file, "--output", "json")
	if requestDeadline {
		if !errors.Is(err, context.DeadlineExceeded) || errfmt.Classify(err).Hint != "Increase the request timeout (e.g. set `ASC_TIMEOUT=90s`)." {
			t.Fatalf("request deadline uses wrong hint: err=%v hint=%q", err, errfmt.Classify(err).Hint)
		}
		return
	}
	if pending {
		if !errors.Is(err, context.DeadlineExceeded) || errfmt.Classify(err).Hint != "Increase the upload timeout (e.g. set `ASC_UPLOAD_TIMEOUT=600s`)." {
			t.Fatalf("processing deadline uses wrong hint: err=%v hint=%q", err, errfmt.Classify(err).Hint)
		}
		if err == nil || !strings.Contains(stdout, `"uploaded":true`) || !strings.Contains(stdout, `"ready":false`) || !strings.Contains(stdout, `"videoId":"video"`) {
			t.Fatalf("premature processing receipt: err=%v stdout=%s", err, stdout)
		}
		return
	}
	if err != nil {
		t.Fatalf("error %v stdout %s stderr %s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, `"videoId":"video"`) || !strings.Contains(stdout, `"ready":true`) {
		t.Fatalf("receipt %s", stdout)
	}
	if len(calls) != 5 {
		t.Fatalf("calls %v", calls)
	}
}

func TestAssetLibraryVideoUploadRejectsRenamedWebMBeforeHTTP(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX ffprobe fixture")
	}
	setupAuth(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "renamed.mp4")
	if err := os.WriteFile(file, []byte("WebM fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	probe := `#!/bin/sh
printf '%s' '{"streams":[{"codec_type":"video","codec_name":"h264","width":1920,"height":1280,"avg_frame_rate":"30/1"}],"format":{"duration":"5","format_name":"matroska,webm"}}'
`
	if err := os.WriteFile(filepath.Join(dir, "ffprobe"), []byte(probe), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	calls := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) { calls++; return jsonResponse(400, `{"errors":[]}`) })
	_, stderr, err := runAssetLibrary(t, "asset-library", "videos", "upload", "--library-id", "library", "--file", file)
	if err == nil || !strings.Contains(stderr, "container") || calls != 0 {
		t.Fatalf("expected pre-auth container rejection, calls=%d err=%v stderr=%s", calls, err, stderr)
	}
}
