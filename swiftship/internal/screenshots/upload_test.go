package screenshots

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestScreenshotHTTPRejectsSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.png")
	secret := []byte("outside-root-secret")
	if err := os.WriteFile(outside, secret, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "escape.png")); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	server := httptest.NewServer(newUploadHandler(root, ScreenshotRequirements{}, make(chan bool, 1)))
	defer server.Close()
	resp, err := http.Get(server.URL + "/screenshots/escape.png")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode == http.StatusOK || bytes.Contains(body, secret) {
		t.Fatalf("symlink escape served: status=%d body=%q", resp.StatusCode, body)
	}
}

func uploadTestServer(t *testing.T, dir string) *httptest.Server {
	t.Helper()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(newUploadHandler(root, RequirementsForFamily("iphone"), make(chan bool, 1)))
	t.Cleanup(func() {
		server.Close()
		root.Close()
	})
	return server
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 1320, 2868))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func postScreenshot(t *testing.T, server *httptest.Server, name string, content []byte) int {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("screenshots", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(server.URL+"/upload", writer.FormDataContentType(), &body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var result struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result.Count
}

func TestScreenshotHTTPValidUploadAndServe(t *testing.T) {
	dir := t.TempDir()
	server := uploadTestServer(t, dir)
	content := testPNG(t)
	// Spaces and non-ASCII names remain supported.
	name := "screen shot-日本.PNG"
	if count := postScreenshot(t, server, name, content); count != 1 {
		t.Fatalf("uploaded count=%d, want 1", count)
	}
	resp, err := http.Get(server.URL + "/screenshots/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || !bytes.Equal(body, content) || resp.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("valid image response: status=%d type=%s bytes=%d", resp.StatusCode, resp.Header.Get("Content-Type"), len(body))
	}
	resp, err = http.Get(server.URL + "/screenshots")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var items []map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0]["deviceType"] != "IPHONE_69" {
		t.Fatalf("screenshots=%v", items)
	}
}

func TestScreenshotHTTPTraversalAndDirectoriesBlocked(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "valid.png"), testPNG(t), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "folder.png"), 0700); err != nil {
		t.Fatal(err)
	}
	server := uploadTestServer(t, dir)
	client := server.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	for _, path := range []string{
		"/screenshots/", "/screenshots/folder.png", "/screenshots/folder.png/",
		"/screenshots/../valid.png", "/screenshots/%2e%2e%2fvalid.png",
		"/screenshots/sub%2fvalid.png", "/screenshots/..%5cvalid.png",
		"/screenshots/%2fvalid.png", "/screenshots/%00valid.png",
	} {
		t.Run(path, func(t *testing.T) {
			resp, err := client.Get(server.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				t.Fatalf("unsafe path returned 200: %s", path)
			}
		})
	}
}

func TestScreenshotHTTPUploadSymlinkEscape(t *testing.T) {
	for _, relative := range []bool{false, true} {
		t.Run(map[bool]string{false: "absolute", true: "relative"}[relative], func(t *testing.T) {
			parent := t.TempDir()
			dir := filepath.Join(parent, "screenshots")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			outside := filepath.Join(parent, "outside.png")
			original := []byte("do not overwrite")
			if err := os.WriteFile(outside, original, 0600); err != nil {
				t.Fatal(err)
			}
			target := outside
			if relative {
				target = "../outside.png"
			}
			if err := os.Symlink(target, filepath.Join(dir, "escape.png")); err != nil {
				t.Fatal(err)
			}
			server := uploadTestServer(t, dir)
			if count := postScreenshot(t, server, "escape.png", testPNG(t)); count != 0 {
				t.Fatalf("escape upload count=%d", count)
			}
			actual, err := os.ReadFile(outside)
			if err != nil || !bytes.Equal(actual, original) {
				t.Fatalf("outside file modified: %q, error=%v", actual, err)
			}
			resp, err := http.Get(server.URL + "/screenshots/escape.png")
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil || resp.StatusCode != http.StatusNotFound || bytes.Contains(body, original) {
				t.Fatalf("escape served: status=%d body=%q error=%v", resp.StatusCode, body, err)
			}
			resp, err = http.Get(server.URL + "/screenshots")
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			var items []map[string]string
			if err := json.NewDecoder(resp.Body).Decode(&items); err != nil || len(items) != 0 {
				t.Fatalf("escape included in list: %v, error=%v", items, err)
			}
		})
	}
}

func TestScreenshotHTTPUploadRejectsUnsafeFilenames(t *testing.T) {
	for _, name := range []string{"../escape.png", "sub/escape.png", `..\escape.png`, "/escape.png", "C:escape.png", ".hidden.png", "index.html"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			server := uploadTestServer(t, dir)
			if count := postScreenshot(t, server, name, testPNG(t)); count != 0 {
				t.Fatalf("unsafe filename saved: %q count=%d", name, count)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("unexpected files: %v error=%v", entries, err)
			}
		})
	}
}
