package shared

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

type buildWaitRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn buildWaitRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func newBuildWaitTestClient(t *testing.T, transport buildWaitRoundTripFunc) *asc.Client {
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

func buildWaitJSONResponse(body string) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     fmt.Sprintf("%d %s", http.StatusOK, http.StatusText(http.StatusOK)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func TestWaitForBuildByNumberOrUploadFailureRejectsStaleBuildFromDifferentUpload(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	uploadCalls := 0
	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("expected GET, got %s", req.Method)
		}

		switch req.URL.Path {
		case "/v1/buildUploads/upload-current":
			uploadCalls++
			return buildWaitJSONResponse(`{
				"data": {
					"type": "buildUploads",
					"id": "upload-current",
					"attributes": {
						"cfBundleShortVersionString": "1.2.3",
						"cfBundleVersion": "42",
						"platform": "IOS",
						"state": {
							"state": "PROCESSING"
						}
					}
				}
			}`)
		case "/v1/preReleaseVersions":
			return buildWaitJSONResponse(`{
				"data": [
					{
						"type": "preReleaseVersions",
						"id": "prv-1",
						"attributes": {
							"version": "1.2.3",
							"platform": "IOS"
						}
					}
				],
				"links": {}
			}`)
		case "/v1/builds":
			if got := req.URL.Query().Get("include"); got != "buildUpload" {
				t.Fatalf("expected include=buildUpload when upload ID is known, got %q", got)
			}
			cancel()
			return buildWaitJSONResponse(`{
				"data": [
					{
						"type": "builds",
						"id": "stale-build",
						"attributes": {
							"version": "42",
							"uploadedDate": "2026-03-16T12:00:05Z"
						},
						"relationships": {
							"buildUpload": {
								"data": {
									"type": "buildUploads",
									"id": "stale-upload"
								}
							}
						}
					}
				],
				"links": {}
			}`)
		default:
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
	})

	_, err := WaitForBuildByNumberOrUploadFailure(ctx, client, "app-1", "upload-current", "1.2.3", "42", "IOS", time.Millisecond)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation after rejecting stale build, got %v", err)
	}
	if uploadCalls == 0 {
		t.Fatal("expected build upload lookup before accepting a discovered build")
	}
}

func TestWaitForBuildByNumberOrUploadFailureReturnsBuildLinkedFromUpload(t *testing.T) {
	client := newBuildWaitTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("expected GET, got %s", req.Method)
		}

		switch req.URL.Path {
		case "/v1/buildUploads/upload-current":
			return buildWaitJSONResponse(`{
				"data": {
					"type": "buildUploads",
					"id": "upload-current",
					"attributes": {
						"cfBundleShortVersionString": "1.2.3",
						"cfBundleVersion": "42",
						"platform": "IOS"
					},
					"relationships": {
						"build": {
							"data": {
								"type": "builds",
								"id": "build-123"
							}
						}
					}
				}
			}`)
		case "/v1/builds/build-123":
			return buildWaitJSONResponse(`{
				"data": {
					"type": "builds",
					"id": "build-123",
					"attributes": {
						"version": "42",
						"processingState": "PROCESSING"
					}
				}
			}`)
		case "/v1/preReleaseVersions", "/v1/builds":
			t.Fatalf("did not expect build discovery list request once upload links a build: %s", req.URL.Path)
			return nil, nil
		default:
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
	})

	buildResp, err := WaitForBuildByNumberOrUploadFailure(context.Background(), client, "app-1", "upload-current", "1.2.3", "42", "IOS", time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForBuildByNumberOrUploadFailure() error: %v", err)
	}
	if buildResp == nil {
		t.Fatal("expected linked build response")
	}
	if buildResp.Data.ID != "build-123" {
		t.Fatalf("expected linked build ID build-123, got %q", buildResp.Data.ID)
	}
}
