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

func TestXcodeCloudWorkflowsResolvesAppByBundleID(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
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
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			query := req.URL.Query()
			if query.Get("filter[bundleId]") != "com.example.lookup" {
				t.Fatalf("expected bundle filter com.example.lookup, got %q", query.Get("filter[bundleId]"))
			}
			if query.Get("limit") != "2" {
				t.Fatalf("expected limit=2, got %q", query.Get("limit"))
			}
			body := `{"data":[{"type":"apps","id":"app-lookup"}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/ciProducts" {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			query := req.URL.Query()
			if query.Get("filter[app]") != "app-lookup" {
				t.Fatalf("expected filter[app]=app-lookup, got %q", query.Get("filter[app]"))
			}
			if query.Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", query.Get("limit"))
			}
			body := `{"data":[{"type":"ciProducts","id":"prod-1"}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/ciProducts/prod-1/workflows" {
				t.Fatalf("unexpected third request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"ciWorkflows","id":"wf-1","attributes":{"name":"CI"}}]}`
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
		if err := root.Parse([]string{"xcode-cloud", "workflows", "--app", "com.example.lookup"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"wf-1"`) {
		t.Fatalf("expected workflow output, got %q", stdout)
	}
}

func TestXcodeCloudProductsResolvesAppByExactName(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
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
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			query := req.URL.Query()
			if query.Get("filter[bundleId]") != "Focus Rail" {
				t.Fatalf("expected bundle filter Focus Rail, got %q", query.Get("filter[bundleId]"))
			}
			body := `{"data":[]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps" {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			query := req.URL.Query()
			if query.Get("filter[name]") != "Focus Rail" {
				t.Fatalf("expected name filter Focus Rail, got %q", query.Get("filter[name]"))
			}
			if query.Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", query.Get("limit"))
			}
			body := `{"data":[{"type":"apps","id":"app-lookup","attributes":{"name":"Focus Rail"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/ciProducts" {
				t.Fatalf("unexpected third request: %s %s", req.Method, req.URL.String())
			}
			query := req.URL.Query()
			if query.Get("filter[app]") != "app-lookup" {
				t.Fatalf("expected filter[app]=app-lookup, got %q", query.Get("filter[app]"))
			}
			body := `{"data":[{"type":"ciProducts","id":"prod-1","attributes":{"name":"Focus Rail"}}]}`
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
		if err := root.Parse([]string{"xcode-cloud", "products", "--app", "Focus Rail"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"prod-1"`) {
		t.Fatalf("expected product output, got %q", stdout)
	}
}

func TestXcodeCloudRunResolvesAppByExactNameWhenWorkflowNameProvided(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
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
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			query := req.URL.Query()
			if query.Get("filter[bundleId]") != "Focus Rail" {
				t.Fatalf("expected bundle filter Focus Rail, got %q", query.Get("filter[bundleId]"))
			}
			body := `{"data":[]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps" {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			query := req.URL.Query()
			if query.Get("filter[name]") != "Focus Rail" {
				t.Fatalf("expected name filter Focus Rail, got %q", query.Get("filter[name]"))
			}
			body := `{"data":[{"type":"apps","id":"app-lookup","attributes":{"name":"Focus Rail"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/ciProducts" {
				t.Fatalf("unexpected third request: %s %s", req.Method, req.URL.String())
			}
			query := req.URL.Query()
			if query.Get("filter[app]") != "app-lookup" {
				t.Fatalf("expected filter[app]=app-lookup, got %q", query.Get("filter[app]"))
			}
			if query.Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", query.Get("limit"))
			}
			body := `{"data":[{"type":"ciProducts","id":"prod-1"}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 4:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/ciProducts/prod-1/workflows" {
				t.Fatalf("unexpected fourth request: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", req.URL.Query().Get("limit"))
			}
			body := `{"data":[{"type":"ciWorkflows","id":"wf-1","attributes":{"name":"CI"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 5:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/ciWorkflows/wf-1/repository" {
				t.Fatalf("unexpected fifth request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":{"type":"scmRepositories","id":"repo-1"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 6:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/scmRepositories/repo-1/gitReferences" {
				t.Fatalf("unexpected sixth request: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", req.URL.Query().Get("limit"))
			}
			body := `{"data":[{"type":"scmGitReferences","id":"ref-1","attributes":{"name":"main","canonicalName":"refs/heads/main"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 7:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/ciBuildRuns" {
				t.Fatalf("unexpected seventh request: %s %s", req.Method, req.URL.String())
			}

			var payload map[string]any
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("failed to decode request body: %v", err)
			}

			data, ok := payload["data"].(map[string]any)
			if !ok {
				t.Fatalf("expected data object in payload, got %#v", payload["data"])
			}
			relationships, ok := data["relationships"].(map[string]any)
			if !ok {
				t.Fatalf("expected relationships object in payload, got %#v", data["relationships"])
			}

			workflow := relationships["workflow"].(map[string]any)
			workflowData := workflow["data"].(map[string]any)
			if workflowData["id"] != "wf-1" {
				t.Fatalf("expected workflow ID wf-1, got %#v", workflowData["id"])
			}

			source := relationships["sourceBranchOrTag"].(map[string]any)
			sourceData := source["data"].(map[string]any)
			if sourceData["id"] != "ref-1" {
				t.Fatalf("expected git reference ID ref-1, got %#v", sourceData["id"])
			}

			body := `{"data":{"type":"ciBuildRuns","id":"run-1","attributes":{"number":1,"executionProgress":"PENDING","startReason":"MANUAL","createdDate":"2026-03-14T00:00:00Z"}}}`
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
		if err := root.Parse([]string{"xcode-cloud", "run", "--app", "Focus Rail", "--workflow", "CI", "--branch", "main"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"buildRunId":"run-1"`) {
		t.Fatalf("expected build run output, got %q", stdout)
	}
}

func TestXcodeCloudProductsRejectsUniqueFuzzyAppName(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
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
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("filter[bundleId]") != "Relax" {
				t.Fatalf("expected bundle filter Relax, got %q", req.URL.Query().Get("filter[bundleId]"))
			}
			body := `{"data":[]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps" {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("filter[name]") != "Relax" {
				t.Fatalf("expected name filter Relax, got %q", req.URL.Query().Get("filter[name]"))
			}
			body := `{"data":[{"type":"apps","id":"app-fuzzy","attributes":{"name":"Relax: Sleep + Focus"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps" {
				t.Fatalf("unexpected third request: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("limit") != "200" {
				t.Fatalf("expected full-scan limit=200, got %q", req.URL.Query().Get("limit"))
			}
			body := `{"data":[{"type":"apps","id":"app-fuzzy","attributes":{"name":"Relax: Sleep + Focus"}}],"links":{"next":""}}`
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

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode-cloud", "products", "--app", "Relax"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected fuzzy app-name lookup to fail")
	}
	if !strings.Contains(runErr.Error(), `app "Relax" not found`) {
		t.Fatalf("expected strict not-found error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if callCount != 3 {
		t.Fatalf("expected exactly 3 lookup requests, got %d", callCount)
	}
}

func TestXcodeCloudProductsTreatsNumericAppAsIDBeforeExactNameLookup(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
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
			if req.Method != http.MethodGet || req.URL.Path != "/v1/ciProducts" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("filter[app]") != "2048" {
				t.Fatalf("expected filter[app]=2048, got %q", req.URL.Query().Get("filter[app]"))
			}
			body := `{"data":[{"type":"ciProducts","id":"prod-2048","attributes":{"name":"2048"}}]}`
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
		if err := root.Parse([]string{"xcode-cloud", "products", "--app", "2048"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"prod-2048"`) {
		t.Fatalf("expected product output, got %q", stdout)
	}
	if callCount != 1 {
		t.Fatalf("expected exactly 1 request, got %d", callCount)
	}
}
