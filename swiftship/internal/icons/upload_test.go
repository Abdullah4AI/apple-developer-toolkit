package icons

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func iconUploadRequest(t *testing.T) *http.Request {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	file, err := form.CreateFormFile("icon", "icon.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("icon contents")); err != nil {
		t.Fatal(err)
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	return req
}

func assertUploadError(t *testing.T, response *httptest.ResponseRecorder, want string) {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q", got)
	}
	var payload map[string]any
	body := response.Body.String()
	decoder := json.NewDecoder(response.Body)
	if err := decoder.Decode(&payload); err != nil {
		t.Fatalf("invalid JSON response: %v; body = %q", err, body)
	}
	if len(payload) != 2 || payload["ok"] != false || payload["error"] != want {
		t.Fatalf("response = %#v, want ok=false and error=%q", payload, want)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("unexpected trailing JSON: %v", err)
	}
}

func TestUploadSaveErrorJSON(t *testing.T) {
	for _, name := range []string{"quote\"dir", "newline\ndir", "backslash\\dir", "\",\"injected\":true,\"error\":\""} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), name)
			_, err := os.Create(filepath.Join(dir, "AppIcon.png"))
			if err == nil {
				t.Fatal("expected nonexistent directory to prevent saving")
			}
			done := make(chan bool, 1)
			response := httptest.NewRecorder()
			newUploadMux(dir, "watchos", done).ServeHTTP(response, iconUploadRequest(t))
			assertUploadError(t, response, "failed to save: "+err.Error())
			select {
			case <-done:
				t.Fatal("failed upload must not signal completion")
			default:
			}
		})
	}
}

func TestWriteUploadErrorEscapesJSON(t *testing.T) {
	for _, prefix := range []string{"failed to save: ", "failed to write: ", "icon saved but Contents.json update failed: "} {
		for _, message := range []string{"quote\"", "newline\n", "backslash\\", "\",\"injected\":true,\"error\":\"", "tabs	and\rcarriage returns", "Unicode: العربية"} {
			t.Run(prefix+message, func(t *testing.T) {
				response := httptest.NewRecorder()
				response.Header().Set("Content-Type", "application/json")
				writeUploadError(response, prefix+message)
				assertUploadError(t, response, prefix+message)
			})
		}
	}
}

func TestUploadContentsErrorJSON(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "quote\"newline\nbackslash\\")
	contentsPath := filepath.Join(dir, "Contents.json")
	if err := os.MkdirAll(contentsPath, 0o755); err != nil {
		t.Fatal(err)
	}
	// A directory at Contents.json deterministically fails the real metadata write.
	err := os.WriteFile(contentsPath, []byte("{}"), 0o644)
	if err == nil {
		t.Fatal("expected Contents.json directory to prevent writing")
	}
	done := make(chan bool, 1)
	response := httptest.NewRecorder()
	newUploadMux(dir, "watchos", done).ServeHTTP(response, iconUploadRequest(t))
	assertUploadError(t, response, "icon saved but Contents.json update failed: "+err.Error())
	if data, err := os.ReadFile(filepath.Join(dir, "AppIcon.png")); err != nil || string(data) != "icon contents" {
		t.Fatalf("saved icon = %q, error = %v", data, err)
	}
	select {
	case <-done:
		t.Fatal("metadata failure must not signal completion")
	default:
	}
}

func TestUploadMissingFileJSON(t *testing.T) {
	done := make(chan bool, 1)
	response := httptest.NewRecorder()
	newUploadMux(t.TempDir(), "watchos", done).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/upload", nil))
	assertUploadError(t, response, "no file received")
	select {
	case <-done:
		t.Fatal("missing file must not signal completion")
	default:
	}
}

func TestUploadMuxSuccess(t *testing.T) {
	dir := t.TempDir()
	done := make(chan bool, 1)
	mux := newUploadMux(dir, "watchos", done)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, iconUploadRequest(t))
	if response.Code != http.StatusOK || response.Body.String() != `{"ok":true}` || response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("upload response: %d %q %q", response.Code, response.Body.String(), response.Header())
	}
	select {
	case result := <-done:
		if !result {
			t.Fatal("successful upload signaled false")
		}
	default:
		t.Fatal("successful upload did not signal completion")
	}
	data, err := os.ReadFile(filepath.Join(dir, "Contents.json"))
	if err != nil || !json.Valid(data) {
		t.Fatalf("Contents.json = %q, error = %v", data, err)
	}
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Body.String() != iconUploadPage || response.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatal("upload page changed")
	}
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/icon", nil))
	if response.Code != http.StatusOK || response.Body.String() != "icon contents" {
		t.Fatalf("icon response: %d %q", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/skip", nil))
	if response.Body.String() != `{"ok":true}` {
		t.Fatalf("skip response: %q", response.Body.String())
	}
	select {
	case result := <-done:
		if result {
			t.Fatal("skip signaled true")
		}
	default:
		t.Fatal("skip did not signal completion")
	}
}

func TestUploadMethodNotAllowed(t *testing.T) {
	response := httptest.NewRecorder()
	newUploadMux(t.TempDir(), "watchos", make(chan bool, 1)).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/upload", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}
