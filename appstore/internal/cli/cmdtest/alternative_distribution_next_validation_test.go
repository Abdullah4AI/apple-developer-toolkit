package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func runAlternativeDistributionPaginateFromNext(
	t *testing.T,
	argsPrefix []string,
	firstURL string,
	secondURL string,
	firstBody string,
	secondBody string,
	wantIDs ...string,
) {
	t.Helper()

	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.String() != firstURL {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(firstBody)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.String() != secondURL {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(secondBody)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	args := append(append([]string{}, argsPrefix...), "--paginate", "--next", firstURL)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, id := range wantIDs {
		needle := `"id":"` + id + `"`
		if !strings.Contains(stdout, needle) {
			t.Fatalf("expected output to contain %q, got %q", needle, stdout)
		}
	}
}

func TestAlternativeDistributionDomainsListPaginateFromNext(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/alternativeDistributionDomains?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/alternativeDistributionDomains?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"alternativeDistributionDomains","id":"alt-domain-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"alternativeDistributionDomains","id":"alt-domain-next-2"}],"links":{"next":""}}`

	runAlternativeDistributionPaginateFromNext(
		t,
		[]string{"alternative-distribution", "domains", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"alt-domain-next-1",
		"alt-domain-next-2",
	)
}

func TestAlternativeDistributionKeysListPaginateFromNext(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/alternativeDistributionKeys?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/alternativeDistributionKeys?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"alternativeDistributionKeys","id":"alt-key-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"alternativeDistributionKeys","id":"alt-key-next-2"}],"links":{"next":""}}`

	runAlternativeDistributionPaginateFromNext(
		t,
		[]string{"alternative-distribution", "keys", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"alt-key-next-1",
		"alt-key-next-2",
	)
}

func TestAlternativeDistributionPackageVersionsListPaginateFromNext(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/alternativeDistributionPackages/pkg-1/versions?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/alternativeDistributionPackages/pkg-1/versions?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"alternativeDistributionPackageVersions","id":"alt-version-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"alternativeDistributionPackageVersions","id":"alt-version-next-2"}],"links":{"next":""}}`

	runAlternativeDistributionPaginateFromNext(
		t,
		[]string{"alternative-distribution", "packages", "versions", "list", "--package-id", "pkg-1"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"alt-version-next-1",
		"alt-version-next-2",
	)
}

func TestAlternativeDistributionPackageVersionsDeltasPaginateFromNext(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/alternativeDistributionPackageVersions/ver-1/deltas?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/alternativeDistributionPackageVersions/ver-1/deltas?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"alternativeDistributionPackageDeltas","id":"alt-delta-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"alternativeDistributionPackageDeltas","id":"alt-delta-next-2"}],"links":{"next":""}}`

	runAlternativeDistributionPaginateFromNext(
		t,
		[]string{"alternative-distribution", "packages", "versions", "deltas", "--version-id", "ver-1"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"alt-delta-next-1",
		"alt-delta-next-2",
	)
}

func TestAlternativeDistributionPackageVersionsVariantsPaginateFromNext(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/alternativeDistributionPackageVersions/ver-1/variants?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/alternativeDistributionPackageVersions/ver-1/variants?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"alternativeDistributionPackageVariants","id":"alt-variant-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"alternativeDistributionPackageVariants","id":"alt-variant-next-2"}],"links":{"next":""}}`

	runAlternativeDistributionPaginateFromNext(
		t,
		[]string{"alternative-distribution", "packages", "versions", "variants", "--version-id", "ver-1"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"alt-variant-next-1",
		"alt-variant-next-2",
	)
}
