package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

// statusBuildStateRequestCounts records every status request path so the
// internal-state and expiry fields can be proven to reuse data already in hand.
type statusBuildStateRequestCounts struct {
	builds                   lockedCounter
	buildBetaDetails         lockedCounter
	betaAppReviewSubmissions lockedCounter
}

// installStatusBuildStateTransport serves a latest build that is expired and
// still processing for internal testers while an older build is the only one
// distributed externally.
func installStatusBuildStateTransport(t *testing.T) *statusBuildStateRequestCounts {
	t.Helper()

	counts := &statusBuildStateRequestCounts{}
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/builds":
			counts.builds.Inc()
			if req.URL.Query().Get("filter[betaAppReviewSubmission.betaReviewState]") != "" {
				return statusJSONResponse(`{"data":[],"links":{"next":""}}`), nil
			}
			if got := req.URL.Query().Get("include"); got != "preReleaseVersion,buildBetaDetail" {
				t.Fatalf("expected snapshot builds to include preReleaseVersion,buildBetaDetail, got include=%q", got)
			}
			return statusJSONResponse(`{
				"data":[
					{
						"type":"builds",
						"id":"build-2",
						"attributes":{
							"version":"45",
							"uploadedDate":"2026-02-20T00:00:00Z",
							"processingState":"VALID",
							"expired":true
						},
						"relationships":{
							"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"train-1.2.3-ios"}},
							"buildBetaDetail":{"data":{"type":"buildBetaDetails","id":"bbd-2"}}
						}
					},
					{
						"type":"builds",
						"id":"build-1",
						"attributes":{
							"version":"44",
							"uploadedDate":"2026-02-19T00:00:00Z",
							"processingState":"VALID",
							"expired":false
						},
						"relationships":{
							"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"train-1.2.3-ios"}},
							"buildBetaDetail":{"data":{"type":"buildBetaDetails","id":"bbd-1"}}
						}
					}
				],
				"included":[
					{"type":"preReleaseVersions","id":"train-1.2.3-ios","attributes":{"version":"1.2.3","platform":"IOS"}},
					{"type":"buildBetaDetails","id":"bbd-1","attributes":{"internalBuildState":"IN_BETA_TESTING","externalBuildState":"IN_BETA_TESTING"}},
					{"type":"buildBetaDetails","id":"bbd-2","attributes":{"internalBuildState":"PROCESSING","externalBuildState":"NOT_READY_FOR_TESTING"}}
				],
				"links":{"next":""}
			}`), nil
		case "/v1/buildBetaDetails":
			counts.buildBetaDetails.Inc()
			return statusJSONResponse(`{"data":[],"links":{"next":""}}`), nil
		case "/v1/betaAppReviewSubmissions":
			counts.betaAppReviewSubmissions.Inc()
			return statusJSONResponse(`{"data":[],"links":{"next":""}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	return counts
}

// assertStatusBuildStateRequestCounts pins the request budget so surfacing the
// new fields cannot silently add API calls.
func assertStatusBuildStateRequestCounts(t *testing.T, counts *statusBuildStateRequestCounts) {
	t.Helper()

	if got := counts.builds.Load(); got != 2 {
		t.Fatalf("expected 2 build requests (snapshot plus active beta review scan), got %d", got)
	}
	if got := counts.buildBetaDetails.Load(); got != 0 {
		t.Fatalf("expected no build beta details requests (states come from included buildBetaDetail), got %d", got)
	}
	if got := counts.betaAppReviewSubmissions.Load(); got != 1 {
		t.Fatalf("expected 1 beta app review submissions request, got %d", got)
	}
}

func TestStatusJSONSurfacesLatestBuildInternalStateAndExpiry(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	counts := installStatusBuildStateTransport(t)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"status", "--app", "6748252780", "--include", "builds,testflight", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		Builds struct {
			Latest struct {
				ID              string `json:"id"`
				ProcessingState string `json:"processingState"`
				Expired         bool   `json:"expired"`
			} `json:"latest"`
		} `json:"builds"`
		TestFlight struct {
			LatestDistributedBuildID string `json:"latestDistributedBuildId"`
			InternalBuildState       string `json:"internalBuildState"`
			ExternalBuildState       string `json:"externalBuildState"`
		} `json:"testflight"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	if payload.Builds.Latest.ID != "build-2" || payload.Builds.Latest.ProcessingState != "VALID" {
		t.Fatalf("expected latest build-2 reported as VALID, got %+v", payload.Builds.Latest)
	}
	if !payload.Builds.Latest.Expired {
		t.Fatalf("expected builds.latest.expired=true for an expired VALID build, got %+v", payload.Builds.Latest)
	}
	if payload.TestFlight.InternalBuildState != "PROCESSING" {
		t.Fatalf("expected internalBuildState of the latest build, got %+v", payload.TestFlight)
	}
	if payload.TestFlight.LatestDistributedBuildID != "build-1" || payload.TestFlight.ExternalBuildState != "IN_BETA_TESTING" {
		t.Fatalf("expected external distribution fields to keep describing build-1, got %+v", payload.TestFlight)
	}

	assertStatusBuildStateRequestCounts(t, counts)
}

func TestStatusTableSurfacesLatestBuildInternalStateAndExpiry(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	counts := installStatusBuildStateTransport(t)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"status", "--app", "6748252780", "--include", "builds,testflight", "--output", "table"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	for _, want := range []string{
		"latest.expired",
		"[x] true",
		"internalBuildState",
		"[~] PROCESSING",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected table output containing %q:\n%s", want, stdout)
		}
	}

	assertStatusBuildStateRequestCounts(t, counts)
}
