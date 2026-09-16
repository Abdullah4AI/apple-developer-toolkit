package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

func TestTestFlightDistributionViewOutputWithLimit(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/buildBetaDetails" {
			t.Fatalf("expected path /v1/buildBetaDetails, got %s", req.URL.Path)
		}
		query := req.URL.Query()
		if query.Get("filter[build]") != "build-1" {
			t.Fatalf("expected build filter build-1, got %q", query.Get("filter[build]"))
		}
		if query.Get("limit") != "1" {
			t.Fatalf("expected limit 1, got %q", query.Get("limit"))
		}
		body := `{"data":[{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":true}}],"links":{"next":""}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "distribution", "view", "--build-id", "build-1", "--limit", "1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"detail-1"`) {
		t.Fatalf("expected detail id in output, got %q", stdout)
	}
}

func TestTestFlightDistributionBuildViewOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/buildBetaDetails/detail-1/build" {
			t.Fatalf("expected path /v1/buildBetaDetails/detail-1/build, got %s", req.URL.Path)
		}
		body := `{"data":{"type":"builds","id":"build-1","attributes":{"version":"1.0"}}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "distribution", "build", "view", "--id", "detail-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-1"`) {
		t.Fatalf("expected build id in output, got %q", stdout)
	}
}

func TestTestFlightDistributionEditOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/buildBetaDetails/detail-1" {
			t.Fatalf("expected path /v1/buildBetaDetails/detail-1, got %s", req.URL.Path)
		}
		var payload struct {
			Data struct {
				Type       string         `json:"type"`
				ID         string         `json:"id"`
				Attributes map[string]any `json:"attributes"`
			} `json:"data"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != "buildBetaDetails" || payload.Data.ID != "detail-1" {
			t.Fatalf("unexpected resource linkage: %#v", payload.Data)
		}
		if len(payload.Data.Attributes) != 1 || payload.Data.Attributes["autoNotifyEnabled"] != true {
			t.Fatalf("expected only autoNotifyEnabled=true, got %#v", payload.Data.Attributes)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":true}}}`)
	}))
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	})
	client, err := asc.NewClientWithHTTPClient(
		"TEST_KEY",
		"TEST_ISSUER",
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: transport},
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "distribution", "edit", "--id", "detail-1", "--auto-notify"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"detail-1"`) {
		t.Fatalf("expected detail id in output, got %q", stdout)
	}
}

func TestTestFlightDistributionEditRejectsRemovedExternalTestingFlag(t *testing.T) {
	clientFactoryCalled := false
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		clientFactoryCalled = true
		return nil, errors.New("client factory must not be called")
	}))

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "enable",
			args: []string{"testflight", "distribution", "edit", "--id", "detail-1", "--external-testing=true"},
		},
		{
			name: "disable",
			args: []string{"testflight", "distribution", "edit", "--id", "detail-1", "--external-testing=false"},
		},
		{
			name: "mixed with supported update",
			args: []string{"testflight", "distribution", "edit", "--id", "detail-1", "--auto-notify", "--external-testing", "true"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled = false
			assertRemovedFlagGuidance(t, test.args, "--external-testing")
			if clientFactoryCalled {
				t.Fatal("expected removed flag to fail before client creation or HTTP")
			}
		})
	}
}

func TestTestFlightRecruitmentOptionsOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaRecruitmentCriterionOptions" {
			t.Fatalf("expected path /v1/betaRecruitmentCriterionOptions, got %s", req.URL.Path)
		}
		query := req.URL.Query()
		if query.Get("fields[betaRecruitmentCriterionOptions]") != "deviceFamilyOsVersions" {
			t.Fatalf("expected fields deviceFamilyOsVersions, got %q", query.Get("fields[betaRecruitmentCriterionOptions]"))
		}
		if query.Get("limit") != "1" {
			t.Fatalf("expected limit 1, got %q", query.Get("limit"))
		}
		body := `{"data":[{"type":"betaRecruitmentCriterionOptions","id":"opt-1","attributes":{"deviceFamilyOsVersions":[{"deviceFamily":"IPHONE","osVersions":["26"]}]}}],"links":{"next":""}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "recruitment", "options", "--fields", "deviceFamilyOsVersions", "--limit", "1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"opt-1"`) {
		t.Fatalf("expected option id in output, got %q", stdout)
	}
}

func TestTestFlightRecruitmentSetUpdatesExisting(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/betaGroups/group-1/betaRecruitmentCriteria" {
				t.Fatalf("expected path /v1/betaGroups/group-1/betaRecruitmentCriteria, got %s", req.URL.Path)
			}
			body := `{"data":{"type":"betaRecruitmentCriteria","id":"criteria-1","attributes":{"deviceFamilyOsVersionFilters":[{"deviceFamily":"IPHONE","minimumOsInclusive":"26"}]}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodPatch {
				t.Fatalf("expected PATCH, got %s", req.Method)
			}
			if req.URL.Path != "/v1/betaRecruitmentCriteria/criteria-1" {
				t.Fatalf("expected path /v1/betaRecruitmentCriteria/criteria-1, got %s", req.URL.Path)
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body error: %v", err)
			}
			if !strings.Contains(string(payload), `"deviceFamilyOsVersionFilters"`) {
				t.Fatalf("expected filters in body, got %s", string(payload))
			}
			body := `{"data":{"type":"betaRecruitmentCriteria","id":"criteria-1","attributes":{"deviceFamilyOsVersionFilters":[{"deviceFamily":"IPHONE","minimumOsInclusive":"26"}]}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", callCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "recruitment", "set", "--group", "group-1", "--os-version-filter", "IPHONE=26"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"criteria-1"`) {
		t.Fatalf("expected criteria id in output, got %q", stdout)
	}
}

func TestTestFlightRecruitmentSetCreatesWhenMissing(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/betaGroups/group-2/betaRecruitmentCriteria" {
				t.Fatalf("expected path /v1/betaGroups/group-2/betaRecruitmentCriteria, got %s", req.URL.Path)
			}
			body := `{"errors":[{"code":"NOT_FOUND","title":"Not Found"}]}`
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			if req.URL.Path != "/v1/betaRecruitmentCriteria" {
				t.Fatalf("expected path /v1/betaRecruitmentCriteria, got %s", req.URL.Path)
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body error: %v", err)
			}
			if !strings.Contains(string(payload), `"id":"group-2"`) {
				t.Fatalf("expected group id in body, got %s", string(payload))
			}
			body := `{"data":{"type":"betaRecruitmentCriteria","id":"criteria-2","attributes":{"deviceFamilyOsVersionFilters":[{"deviceFamily":"IPHONE","minimumOsInclusive":"26"}]}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", callCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "recruitment", "set", "--group", "group-2", "--os-version-filter", "IPHONE=26"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"criteria-2"`) {
		t.Fatalf("expected criteria id in output, got %q", stdout)
	}
}

func TestTestFlightRecruitmentSetReturnsErrorOnFetchFailure(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := `{"errors":[{"code":"FORBIDDEN","title":"Forbidden"}]}`
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "recruitment", "set", "--group", "group-3", "--os-version-filter", "IPHONE=26"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected non-help error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(runErr.Error(), "failed to fetch existing criteria") {
		t.Fatalf("expected fetch error, got %v (stderr: %q)", runErr, stderr)
	}
}
