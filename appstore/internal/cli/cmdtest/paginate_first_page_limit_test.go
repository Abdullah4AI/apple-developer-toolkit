package cmdtest

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPaginateRequestsMaxLimitOnFirstPage(t *testing.T) {
	tests := []struct {
		args      []string
		path      string
		wantLimit string
	}{
		{[]string{"subscriptions", "versions", "list", "--subscription-id", "8000000001"}, "/v1/subscriptions/8000000001/versions", "200"},
		{[]string{"subscriptions", "versions", "links", "--subscription-id", "8000000001"}, "/v1/subscriptions/8000000001/relationships/versions", "200"},
		{[]string{"subscriptions", "versions", "localizations", "list", "--version-id", "ver-1"}, "/v1/subscriptionVersions/ver-1/localizations", "200"},
		{[]string{"subscriptions", "versions", "localizations", "links", "--version-id", "ver-1"}, "/v1/subscriptionVersions/ver-1/relationships/localizations", "200"},
		{[]string{"subscriptions", "versions", "images", "list", "--version-id", "ver-1"}, "/v1/subscriptionVersions/ver-1/images", "200"},
		{[]string{"subscriptions", "versions", "images", "links", "--version-id", "ver-1"}, "/v1/subscriptionVersions/ver-1/relationships/images", "200"},
		{[]string{"subscriptions", "groups", "versions", "list", "--group-id", "group-1"}, "/v1/subscriptionGroups/group-1/versions", "200"},
		{[]string{"subscriptions", "groups", "versions", "links", "versions", "--group-id", "group-1"}, "/v1/subscriptionGroups/group-1/relationships/versions", "200"},
		{[]string{"subscriptions", "groups", "versions", "localizations", "list", "--version-id", "ver-1"}, "/v1/subscriptionGroupVersions/ver-1/localizations", "200"},
		{[]string{"iap", "versions", "list", "--iap-id", "iap-1"}, "/v2/inAppPurchases/iap-1/versions", "200"},
		{[]string{"iap", "versions", "list", "--next", "https://api.appstoreconnect.apple.com/v2/inAppPurchases/iap-1/versions?cursor=Mg"}, "/v2/inAppPurchases/iap-1/versions", ""},
		{[]string{"iap", "versions", "links", "versions", "--iap-id", "iap-1"}, "/v2/inAppPurchases/iap-1/relationships/versions", "200"},
		{[]string{"iap", "versions", "localizations", "list", "--version-id", "ver-1"}, "/v1/inAppPurchaseVersions/ver-1/localizations", "200"},
		{[]string{"iap", "versions", "images", "list", "--version-id", "ver-1"}, "/v1/inAppPurchaseVersions/ver-1/images", "200"},
		{[]string{"game-center", "details", "blocked-players", "list", "--detail-id", "detail-1"}, "/v1/gameCenterDetails/detail-1/blockedPlayers", "200"},
		{[]string{"game-center", "leaderboards", "v2", "score-moderations", "list", "--leaderboard-id", "lb-1"}, "/v2/gameCenterLeaderboards/lb-1/gameCenterScoreModerations", "200"},
		{[]string{"asset-library", "images", "list", "--library-id", "lib-1"}, "/v1/appAssetLibraries/lib-1/images", "200"},
		{[]string{"localizations", "placements", "list", "--localization-id", "loc-1"}, "/v1/appStoreVersionLocalizations/loc-1/placements", "200"},
		{[]string{"subscriptions", "versions", "images", "list", "--version-id", "ver-1", "--limit", "7"}, "/v1/subscriptionVersions/ver-1/images", "7"},
	}

	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			setupAuth(t)
			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })

			var requests []string
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests = append(requests, req.URL.String())
				if req.URL.Path != test.path {
					t.Fatalf("unexpected request: %s", req.URL.String())
				}
				if got := req.URL.Query().Get("limit"); got != test.wantLimit {
					t.Fatalf("expected first-page limit=%s, got %q (%s)", test.wantLimit, got, req.URL.String())
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"data":[],"links":{}}`)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			captureOutput(t, func() {
				if err := root.Parse(append(test.args, "--paginate")); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})
			if len(requests) != 1 {
				t.Fatalf("expected 1 request, got %d: %v", len(requests), requests)
			}
		})
	}
}
