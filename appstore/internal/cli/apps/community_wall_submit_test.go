package apps

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCollectCommunityWallSubmitInputUsesGitHubLoginWhenNonInteractive(t *testing.T) {
	previousPromptEnabled := communityWallPromptEnabled
	communityWallPromptEnabled = func() bool { return false }
	t.Cleanup(func() { communityWallPromptEnabled = previousPromptEnabled })

	input, err := collectCommunityWallSubmitInput(
		"1234567890",
		"",
		"",
		"",
		"ios, macos",
		"octocat",
	)
	if err != nil {
		t.Fatalf("collect input: %v", err)
	}

	if input.AppID != "1234567890" {
		t.Fatalf("expected app ID to be preserved, got %q", input.AppID)
	}
	if input.Creator != "octocat" {
		t.Fatalf("expected creator defaulted from gh login, got %q", input.Creator)
	}
	if got := strings.Join(input.Platform, ","); got != "iOS,macOS" {
		t.Fatalf("expected canonicalized platforms, got %q", got)
	}
}

func TestSubmitCommunityWallEntryDryRunReturnsPlan(t *testing.T) {
	sourceJSON := `[
  {
    "app": "Alpha",
    "link": "https://example.com/alpha",
    "creator": "alpha-dev",
    "platform": ["iOS"]
  }
]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/tester/App-Store-Connect-CLI":
			http.NotFound(w, r)
		case r.Method == http.MethodGet && r.URL.Path == "/repos/rudrankriyam/App-Store-Connect-CLI/git/ref/heads/main":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"object": map[string]any{
					"sha": "base-sha-123",
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/repos/rudrankriyam/App-Store-Connect-CLI/contents/docs/wall-of-apps.json":
			if got := r.URL.Query().Get("ref"); got != "base-sha-123" {
				t.Fatalf("expected ref=base-sha-123, got %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sha":      "blob123",
				"encoding": "base64",
				"content":  base64.StdEncoding.EncodeToString([]byte(sourceJSON)),
			})
		default:
			t.Fatalf("unexpected request during dry-run: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	previousAPIBase := communityWallGitHubAPIBase
	previousHTTPClient := communityWallGitHubClient
	previousLookupDetails := communityWallLookupAppDetails
	previousNow := communityWallNow
	communityWallGitHubAPIBase = server.URL
	communityWallGitHubClient = func() *http.Client { return server.Client() }
	communityWallLookupAppDetails = func(ctx context.Context, ids []string) (map[string]communityWallAppDetails, error) {
		return map[string]communityWallAppDetails{
			"1234567890": {
				Name: "Beta",
				Link: "https://apps.apple.com/us/app/beta/id1234567890",
				Icon: "https://example.com/icon.png",
			},
		}, nil
	}
	communityWallNow = func() time.Time {
		return time.Date(2026, time.March, 10, 12, 0, 0, 0, time.UTC)
	}
	t.Cleanup(func() {
		communityWallGitHubAPIBase = previousAPIBase
		communityWallGitHubClient = previousHTTPClient
		communityWallLookupAppDetails = previousLookupDetails
		communityWallNow = previousNow
	})

	result, err := submitCommunityWallEntry(context.Background(), communityWallSubmitRequest{
		Input: communityWallSubmitInput{
			AppID:    "1234567890",
			Creator:  "tester",
			Platform: []string{"iOS", "macOS"},
		},
		GitHubToken: "token",
		GitHubLogin: "tester",
		DryRun:      true,
	})
	if err != nil {
		t.Fatalf("submit dry-run: %v", err)
	}

	if result.Mode != "dry-run" {
		t.Fatalf("expected dry-run mode, got %q", result.Mode)
	}
	if !result.WillCreateFork {
		t.Fatalf("expected dry-run to indicate fork creation")
	}
	if result.PullRequestURL != "" {
		t.Fatalf("expected no PR URL in dry-run, got %q", result.PullRequestURL)
	}
	if len(result.ChangedFiles) != 1 || result.ChangedFiles[0] != communityWallSourcePath {
		t.Fatalf("expected only %s to change, got %+v", communityWallSourcePath, result.ChangedFiles)
	}
	if result.AppID != "1234567890" {
		t.Fatalf("expected app ID in result, got %q", result.AppID)
	}
	if result.Link != "https://apps.apple.com/us/app/beta/id1234567890" {
		t.Fatalf("expected resolved App Store link, got %q", result.Link)
	}
	if !strings.Contains(result.PullRequestTitle, "apps wall: add Beta") {
		t.Fatalf("unexpected PR title %q", result.PullRequestTitle)
	}
}

func TestSubmitCommunityWallEntryRejectsDuplicateAppID(t *testing.T) {
	sourceJSON := `[
  {
    "app": "Beta",
    "link": "https://apps.apple.com/us/app/beta/id1234567890",
    "creator": "beta-dev",
    "platform": ["iOS"]
  }
]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/tester/App-Store-Connect-CLI":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"full_name":"tester/App-Store-Connect-CLI","fork":true,"parent":{"full_name":"rudrankriyam/App-Store-Connect-CLI"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/repos/rudrankriyam/App-Store-Connect-CLI/git/ref/heads/main":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"object": map[string]any{
					"sha": "base-sha-123",
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/repos/rudrankriyam/App-Store-Connect-CLI/contents/docs/wall-of-apps.json":
			if got := r.URL.Query().Get("ref"); got != "base-sha-123" {
				t.Fatalf("expected ref=base-sha-123, got %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sha":      "blob123",
				"encoding": "base64",
				"content":  base64.StdEncoding.EncodeToString([]byte(sourceJSON)),
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	previousAPIBase := communityWallGitHubAPIBase
	previousHTTPClient := communityWallGitHubClient
	previousLookupDetails := communityWallLookupAppDetails
	communityWallGitHubAPIBase = server.URL
	communityWallGitHubClient = func() *http.Client { return server.Client() }
	communityWallLookupAppDetails = func(ctx context.Context, ids []string) (map[string]communityWallAppDetails, error) {
		return map[string]communityWallAppDetails{
			"1234567890": {
				Name: "Beta 2",
				Link: "https://apps.apple.com/us/app/beta-2/id1234567890",
			},
		}, nil
	}
	t.Cleanup(func() {
		communityWallGitHubAPIBase = previousAPIBase
		communityWallGitHubClient = previousHTTPClient
		communityWallLookupAppDetails = previousLookupDetails
	})

	_, err := submitCommunityWallEntry(context.Background(), communityWallSubmitRequest{
		Input: communityWallSubmitInput{
			AppID:    "1234567890",
			Creator:  "tester",
			Platform: []string{"iOS"},
		},
		GitHubToken: "token",
		GitHubLogin: "tester",
		DryRun:      true,
	})
	if err == nil {
		t.Fatal("expected duplicate app ID error")
	}
	if !strings.Contains(err.Error(), `app ID "1234567890" already exists`) {
		t.Fatalf("expected duplicate app ID message, got %v", err)
	}
}

func TestSubmitCommunityWallEntryRejectsExistingNonForkRepo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/repos/tester/App-Store-Connect-CLI" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		_, _ = w.Write([]byte(`{"full_name":"tester/App-Store-Connect-CLI","fork":false}`))
	}))
	defer server.Close()

	previousAPIBase := communityWallGitHubAPIBase
	previousHTTPClient := communityWallGitHubClient
	previousLookupDetails := communityWallLookupAppDetails
	communityWallGitHubAPIBase = server.URL
	communityWallGitHubClient = func() *http.Client { return server.Client() }
	communityWallLookupAppDetails = func(ctx context.Context, ids []string) (map[string]communityWallAppDetails, error) {
		return map[string]communityWallAppDetails{
			"1234567890": {
				Name: "Beta",
				Link: "https://apps.apple.com/us/app/beta/id1234567890",
			},
		}, nil
	}
	t.Cleanup(func() {
		communityWallGitHubAPIBase = previousAPIBase
		communityWallGitHubClient = previousHTTPClient
		communityWallLookupAppDetails = previousLookupDetails
	})

	_, err := submitCommunityWallEntry(context.Background(), communityWallSubmitRequest{
		Input: communityWallSubmitInput{
			AppID:    "1234567890",
			Creator:  "tester",
			Platform: []string{"iOS"},
		},
		GitHubToken: "token",
		GitHubLogin: "tester",
		DryRun:      true,
	})
	if err == nil {
		t.Fatal("expected non-fork repo error")
	}
	if !strings.Contains(err.Error(), "is not a fork of rudrankriyam/App-Store-Connect-CLI") {
		t.Fatalf("expected non-fork repo error, got %v", err)
	}
}

func TestWaitForRepoReturnsFriendlyTimeoutAfterSleepCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/repos/tester/App-Store-Connect-CLI" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	previousAPIBase := communityWallGitHubAPIBase
	previousHTTPClient := communityWallGitHubClient
	previousSleep := communityWallSleep
	communityWallGitHubAPIBase = server.URL
	communityWallGitHubClient = func() *http.Client { return server.Client() }
	t.Cleanup(func() {
		communityWallGitHubAPIBase = previousAPIBase
		communityWallGitHubClient = previousHTTPClient
		communityWallSleep = previousSleep
	})

	ctx, cancel := context.WithCancel(context.Background())
	communityWallSleep = func(time.Duration) {
		cancel()
	}

	client := communityWallGitHubClientAPI{Token: "token"}
	err := client.waitForRepo(ctx, "tester", "App-Store-Connect-CLI")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out waiting for fork tester/App-Store-Connect-CLI") {
		t.Fatalf("expected friendly timeout error, got %v", err)
	}
}

func TestFetchCommunityWallAppDetailsOmitsCountryFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("id"); got != "1234567890" {
			t.Fatalf("expected id query, got %q", got)
		}
		if got := r.URL.Query().Get("country"); got != "" {
			t.Fatalf("expected no country query filter, got %q", got)
		}
		_, _ = w.Write([]byte(`{"results":[{"trackId":1234567890,"trackName":"Beta","trackViewUrl":"https://apps.apple.com/app/id1234567890","artworkUrl100":"https://example.com/icon.png"}]}`))
	}))
	defer server.Close()

	previousLookupURL := communityWallAppStoreLookupURL
	communityWallAppStoreLookupURL = server.URL
	t.Cleanup(func() {
		communityWallAppStoreLookupURL = previousLookupURL
	})

	details, err := fetchCommunityWallAppDetails(context.Background(), []string{"1234567890"})
	if err != nil {
		t.Fatalf("fetch app details: %v", err)
	}
	if got := details["1234567890"].Name; got != "Beta" {
		t.Fatalf("expected app details for requested ID, got %+v", details)
	}
}
