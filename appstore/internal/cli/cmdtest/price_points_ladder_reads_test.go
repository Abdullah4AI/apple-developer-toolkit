package cmdtest

import (
	"net/http"
	"strings"
	"testing"
)

func TestPricePointLadderReadsSendTerritoryFiltersIncludesAndReadEveryPageForPriceFilters(t *testing.T) {
	const page2 = "https://api.appstoreconnect.apple.com/v2/inAppPurchases/9000000001/pricePoints?cursor=Mg"
	for _, test := range []struct {
		name       string
		args       []string
		path       string
		wantQuery  map[string]string
		pages      []string
		wantStdout string
	}{
		{
			name:       "iap list preserves a comma within a territory name",
			args:       []string{"iap", "pricing", "price-points", "list", "--iap-id", "9000000001", "--territory", "Moldova, Republic of"},
			path:       "/v2/inAppPurchases/9000000001/pricePoints",
			wantQuery:  map[string]string{"filter[territory]": "MDA"},
			pages:      []string{`{"data":[],"links":{}}`},
			wantStdout: `"data":[]`,
		},
		{
			name:       "app list preserves a comma within a territory name",
			args:       []string{"pricing", "price-points", "--app", "app-1", "--territory", "Moldova, Republic of"},
			path:       "/v1/apps/app-1/appPricePoints",
			wantQuery:  map[string]string{"filter[territory]": "MDA"},
			pages:      []string{`{"data":[],"links":{}}`},
			wantStdout: `"data":[]`,
		},
		{
			name:       "iap list takes several territories and includes territory",
			args:       []string{"iap", "pricing", "price-points", "list", "--iap-id", "9000000001", "--territory", "DEU,GB", "--include", "territory"},
			path:       "/v2/inAppPurchases/9000000001/pricePoints",
			wantQuery:  map[string]string{"filter[territory]": "DEU,GBR", "include": "territory"},
			pages:      []string{`{"data":[],"included":[{"type":"territories","id":"DEU","attributes":{"currency":"EUR"}}],"links":{}}`},
			wantStdout: `"included":[{"type":"territories","id":"DEU","attributes":{"currency":"EUR"}}]`,
		},
		{
			name:       "app list takes several territories and includes territory",
			args:       []string{"pricing", "price-points", "--app", "app-1", "--territory", "DEU,GB", "--include", "territory"},
			path:       "/v1/apps/app-1/appPricePoints",
			wantQuery:  map[string]string{"filter[territory]": "DEU,GBR", "include": "territory"},
			pages:      []string{`{"data":[],"included":[{"type":"territories","id":"GBR","attributes":{"currency":"GBP"}}],"links":{}}`},
			wantStdout: `"id":"GBR"`,
		},
		{
			name:       "iap equalizations include territory",
			args:       []string{"iap", "pricing", "price-points", "equalizations", "--id", "pp-1", "--include", "territory"},
			path:       "/v1/inAppPurchasePricePoints/pp-1/equalizations",
			wantQuery:  map[string]string{"include": "territory"},
			pages:      []string{`{"data":[{"type":"inAppPurchasePricePoints","id":"eq-1","relationships":{"territory":{"data":{"type":"territories","id":"FRA"}}}}],"links":{}}`},
			wantStdout: `"territory":{"data":{"type":"territories","id":"FRA"}}`,
		},
		{
			name:       "app equalizations include territory",
			args:       []string{"pricing", "price-points", "equalizations", "--price-point", "pp-1", "--include", "territory"},
			path:       "/v3/appPricePoints/pp-1/equalizations",
			wantQuery:  map[string]string{"include": "territory"},
			pages:      []string{`{"data":[{"type":"appPricePoints","id":"eq-1","relationships":{"territory":{"data":{"type":"territories","id":"FRA"}}}}],"links":{}}`},
			wantStdout: `"id":"FRA"`,
		},
		{
			name:      "iap price filter reads past the first page without paginate",
			args:      []string{"iap", "pricing", "price-points", "list", "--iap-id", "9000000001", "--territory", "USA", "--price", "4.99"},
			path:      "/v2/inAppPurchases/9000000001/pricePoints",
			wantQuery: map[string]string{"filter[territory]": "USA"},
			pages: []string{
				`{"data":[{"type":"inAppPurchasePricePoints","id":"usa-099","attributes":{"customerPrice":"0.99"}}],"links":{"next":"` + page2 + `"}}`,
				`{"data":[{"type":"inAppPurchasePricePoints","id":"usa-499","attributes":{"customerPrice":"4.99"}}],"links":{}}`,
			},
			wantStdout: `{"data":[{"type":"inAppPurchasePricePoints","id":"usa-499","attributes":{"customerPrice":"4.99"}}]`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_APP_ID", "")
			requests := 0
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.Path != test.path {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
				}
				if requests == 0 {
					for key, want := range test.wantQuery {
						if got := req.URL.Query().Get(key); got != want {
							t.Fatalf("%s = %q, want %q (query %s)", key, got, want, req.URL.RawQuery)
						}
					}
				}
				if requests >= len(test.pages) {
					t.Fatalf("unexpected extra request %s", req.URL.String())
				}
				body := test.pages[requests]
				requests++
				return jsonHTTPResponse(http.StatusOK, body), nil
			}))

			stdout, _, err := runRootCommand(t, append(test.args, "--output", "json"))
			if err != nil {
				t.Fatalf("run error: %v", err)
			}
			if requests != len(test.pages) {
				t.Fatalf("requests = %d, want %d", requests, len(test.pages))
			}
			if !strings.Contains(stdout, test.wantStdout) {
				t.Fatalf("stdout %q does not contain %q", stdout, test.wantStdout)
			}
		})
	}
}
