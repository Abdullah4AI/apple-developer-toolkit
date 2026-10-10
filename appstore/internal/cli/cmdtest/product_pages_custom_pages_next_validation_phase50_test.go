package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func runProductPagesPaginateFromNext(
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

func TestCustomPagesListPaginateFromNext(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/appCustomProductPages?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/appCustomProductPages?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"appCustomProductPages","id":"custom-page-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"appCustomProductPages","id":"custom-page-next-2"}],"links":{"next":""}}`

	runProductPagesPaginateFromNext(
		t,
		[]string{"product-pages", "custom-pages", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"custom-page-next-1",
		"custom-page-next-2",
	)
}

func TestCustomPageVersionsListPaginateFromNext(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/appCustomProductPages/page-1/appCustomProductPageVersions?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/appCustomProductPages/page-1/appCustomProductPageVersions?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"appCustomProductPageVersions","id":"custom-page-version-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"appCustomProductPageVersions","id":"custom-page-version-next-2"}],"links":{"next":""}}`

	runProductPagesPaginateFromNext(
		t,
		[]string{"product-pages", "custom-pages", "versions", "list", "--custom-page-id", "page-1"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"custom-page-version-next-1",
		"custom-page-version-next-2",
	)
}

func TestCustomPageLocalizationsListPaginateFromNext(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/appCustomProductPageVersions/version-1/appCustomProductPageLocalizations?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/appCustomProductPageVersions/version-1/appCustomProductPageLocalizations?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"appCustomProductPageLocalizations","id":"custom-page-localization-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"appCustomProductPageLocalizations","id":"custom-page-localization-next-2"}],"links":{"next":""}}`

	runProductPagesPaginateFromNext(
		t,
		[]string{"product-pages", "custom-pages", "localizations", "list", "--custom-page-version-id", "version-1"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"custom-page-localization-next-1",
		"custom-page-localization-next-2",
	)
}

func TestCustomPageLocalizationPreviewSetsListPaginateFromNextWithoutLocalizationID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/appCustomProductPageLocalizations/loc-1/appPreviewSets?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/appCustomProductPageLocalizations/loc-1/appPreviewSets?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"appPreviewSets","id":"custom-page-preview-set-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"appPreviewSets","id":"custom-page-preview-set-next-2"}],"links":{"next":""}}`

	runProductPagesPaginateFromNext(
		t,
		[]string{"product-pages", "custom-pages", "localizations", "preview-sets", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"custom-page-preview-set-next-1",
		"custom-page-preview-set-next-2",
	)
}

func TestCustomPageLocalizationScreenshotSetsListPaginateFromNextWithoutLocalizationID(t *testing.T) {
	const firstURL = "https://api.appstoreconnect.apple.com/v1/appCustomProductPageLocalizations/loc-1/appScreenshotSets?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/appCustomProductPageLocalizations/loc-1/appScreenshotSets?cursor=BQ&limit=200"

	firstBody := `{"data":[{"type":"appScreenshotSets","id":"custom-page-screenshot-set-next-1"}],"links":{"next":"` + secondURL + `"}}`
	secondBody := `{"data":[{"type":"appScreenshotSets","id":"custom-page-screenshot-set-next-2"}],"links":{"next":""}}`

	runProductPagesPaginateFromNext(
		t,
		[]string{"product-pages", "custom-pages", "localizations", "screenshot-sets", "list"},
		firstURL,
		secondURL,
		firstBody,
		secondBody,
		"custom-page-screenshot-set-next-1",
		"custom-page-screenshot-set-next-2",
	)
}
