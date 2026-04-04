package shared

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

type buildUploadsRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn buildUploadsRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func newBuildUploadsTestClient(t *testing.T, transport buildUploadsRoundTripFunc) *asc.Client {
	t.Helper()

	keyPath := filepath.Join(t.TempDir(), "key.p8")
	writeECDSAPEM(t, keyPath)

	httpClient := &http.Client{Transport: transport}
	client, err := asc.NewClientWithHTTPClient("KEY123", "ISS456", keyPath, httpClient)
	if err != nil {
		t.Fatalf("NewClientWithHTTPClient() error: %v", err)
	}
	return client
}

func buildUploadsJSONStatusResponse(statusCode int, body string) (*http.Response, error) {
	return &http.Response{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func TestPrepareBuildUploadCreatesUploadAndFileReservation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.ipa")
	if err := os.WriteFile(path, []byte("payload"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}

	requestCount := 0
	client := newBuildUploadsTestClient(t, func(req *http.Request) (*http.Response, error) {
		requestCount++
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("ReadAll() error: %v", err)
		}

		switch requestCount {
		case 1:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/buildUploads" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := string(bodyBytes)
			if !strings.Contains(body, `"cfBundleShortVersionString":"1.2.3"`) || !strings.Contains(body, `"cfBundleVersion":"42"`) || !strings.Contains(body, `"platform":"IOS"`) {
				t.Fatalf("unexpected upload request body: %s", body)
			}
			if !strings.Contains(body, `"id":"app-123"`) {
				t.Fatalf("expected app relationship in upload request body: %s", body)
			}
			return buildUploadsJSONStatusResponse(http.StatusCreated, `{"data":{"type":"buildUploads","id":"upload-123"}}`)
		case 2:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/buildUploadFiles" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			body := string(bodyBytes)
			if !strings.Contains(body, `"fileName":"app.ipa"`) || !strings.Contains(body, `"uti":"com.apple.ipa"`) {
				t.Fatalf("unexpected file request body: %s", body)
			}
			if !strings.Contains(body, `"id":"upload-123"`) {
				t.Fatalf("expected build upload relationship in file request body: %s", body)
			}
			return buildUploadsJSONStatusResponse(http.StatusCreated, `{"data":{"type":"buildUploadFiles","id":"file-456","attributes":{"fileName":"app.ipa","fileSize":7}}}`)
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	uploadResp, fileResp, err := PrepareBuildUpload(context.Background(), client, "app-123", fileInfo, "1.2.3", "42", asc.PlatformIOS, asc.UTIIPA)
	if err != nil {
		t.Fatalf("PrepareBuildUpload() error: %v", err)
	}
	if uploadResp.Data.ID != "upload-123" {
		t.Fatalf("expected upload ID upload-123, got %q", uploadResp.Data.ID)
	}
	if fileResp.Data.ID != "file-456" {
		t.Fatalf("expected file ID file-456, got %q", fileResp.Data.ID)
	}
}

func TestCommitBuildUploadFileMarksUploadComplete(t *testing.T) {
	client := newBuildUploadsTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/buildUploadFiles/file-456" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("ReadAll() error: %v", err)
		}
		body := string(bodyBytes)
		if !strings.Contains(body, `"uploaded":true`) {
			t.Fatalf("expected uploaded=true in request body: %s", body)
		}
		if !strings.Contains(body, `"hash":"abc123"`) || !strings.Contains(body, `"algorithm":"SHA_256"`) {
			t.Fatalf("expected checksums in request body: %s", body)
		}
		return buildUploadsJSONStatusResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"file-456","attributes":{"uploaded":true}}}`)
	})

	resp, err := CommitBuildUploadFile(context.Background(), client, "file-456", &asc.Checksums{
		File: &asc.Checksum{
			Hash:      "abc123",
			Algorithm: asc.ChecksumAlgorithmSHA256,
		},
	})
	if err != nil {
		t.Fatalf("CommitBuildUploadFile() error: %v", err)
	}
	if resp == nil || resp.Data.Attributes.Uploaded == nil || !*resp.Data.Attributes.Uploaded {
		t.Fatalf("expected uploaded response, got %#v", resp)
	}
}
