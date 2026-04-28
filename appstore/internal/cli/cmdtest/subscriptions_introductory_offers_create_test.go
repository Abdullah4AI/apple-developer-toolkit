package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestSubscriptionsIntroductoryOffersCreateNormalizesTerritory(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/subscriptionIntroductoryOffers" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}

		var payload struct {
			Data struct {
				Relationships struct {
					Territory struct {
						Data struct {
							ID string `json:"id"`
						} `json:"data"`
					} `json:"territory"`
				} `json:"relationships"`
			} `json:"data"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request payload: %v", err)
		}
		if got := payload.Data.Relationships.Territory.Data.ID; got != "USA" {
			t.Fatalf("expected normalized territory USA, got %q", got)
		}

		return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"subscriptionIntroductoryOffers","id":"intro-1","attributes":{"duration":"ONE_MONTH","offerMode":"FREE_TRIAL","numberOfPeriods":1}}}`), nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	if err := root.Parse([]string{
		"subscriptions", "offers", "introductory", "create",
		"--subscription-id", "8000000001",
		"--offer-duration", "ONE_MONTH",
		"--offer-mode", "FREE_TRIAL",
		"--number-of-periods", "1",
		"--territory", "US",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := root.Run(context.Background()); err != nil {
		t.Fatalf("run error: %v", err)
	}
}

func TestSubscriptionsIntroductoryOffersCreateAllTerritoriesDryRunSummarizesAvailability(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	seen := make([]string, 0, 3)
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		seen = append(seen, req.Method+" "+req.URL.Path)
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/subscriptionAvailability":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1"}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAvailabilities/avail-1/availableTerritories" && req.URL.Query().Get("cursor") == "":
			body := `{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"CAN"}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/subscriptionAvailabilities/avail-1/availableTerritories?cursor=2"}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAvailabilities/avail-1/availableTerritories" && req.URL.Query().Get("cursor") == "2":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"territories","id":"GBR"}],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/introductoryOffers":
			body := `{"data":[{"type":"subscriptionIntroductoryOffers","id":"eyJpIjoiVVMifQ"}],"links":{}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var summary struct {
		SubscriptionID string `json:"subscriptionId"`
		AllTerritories bool   `json:"allTerritories"`
		DryRun         bool   `json:"dryRun"`
		Total          int    `json:"total"`
		Created        int    `json:"created"`
		Skipped        int    `json:"skipped"`
		Failed         int    `json:"failed"`
	}
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "create",
			"--subscription-id", "8000000001",
			"--offer-duration", "ONE_MONTH",
			"--offer-mode", "FREE_TRIAL",
			"--number-of-periods", "1",
			"--all-territories",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse JSON summary: %v", err)
	}
	if summary.SubscriptionID != "8000000001" || !summary.AllTerritories || !summary.DryRun {
		t.Fatalf("unexpected summary identity: %+v", summary)
	}
	if summary.Total != 3 || summary.Created != 2 || summary.Skipped != 1 || summary.Failed != 0 {
		t.Fatalf("unexpected summary counts: %+v", summary)
	}
	for _, request := range seen {
		if strings.HasPrefix(request, http.MethodPost+" ") {
			t.Fatalf("dry-run should not POST, saw requests: %v", seen)
		}
	}
}

func TestSubscriptionsIntroductoryOffersCreateAllTerritoriesCreatesPerAvailabilityTerritory(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	postedTerritories := make([]string, 0, 2)
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/subscriptionAvailability":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1"}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAvailabilities/avail-1/availableTerritories":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"territories","id":"CAN"},{"type":"territories","id":"USA"}],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/introductoryOffers":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionIntroductoryOffers":
			var payload struct {
				Data struct {
					Attributes struct {
						Duration        string `json:"duration"`
						OfferMode       string `json:"offerMode"`
						NumberOfPeriods int    `json:"numberOfPeriods"`
					} `json:"attributes"`
					Relationships struct {
						Territory struct {
							Data struct {
								ID string `json:"id"`
							} `json:"data"`
						} `json:"territory"`
					} `json:"relationships"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if payload.Data.Attributes.Duration != "ONE_MONTH" || payload.Data.Attributes.OfferMode != "FREE_TRIAL" || payload.Data.Attributes.NumberOfPeriods != 1 {
				t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
			}
			postedTerritories = append(postedTerritories, payload.Data.Relationships.Territory.Data.ID)
			return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"subscriptionIntroductoryOffers","id":"intro-new"}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var summary struct {
		Total   int `json:"total"`
		Created int `json:"created"`
		Skipped int `json:"skipped"`
		Failed  int `json:"failed"`
	}
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "create",
			"--subscription-id", "8000000001",
			"--offer-duration", "ONE_MONTH",
			"--offer-mode", "FREE_TRIAL",
			"--number-of-periods", "1",
			"--territory", "ALL",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse JSON summary: %v", err)
	}
	if summary.Total != 2 || summary.Created != 2 || summary.Skipped != 0 || summary.Failed != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if got := strings.Join(postedTerritories, ","); got != "CAN,USA" {
		t.Fatalf("expected POSTs for CAN,USA in availability order, got %s", got)
	}
}

func TestSubscriptionsIntroductoryOffersCreateAllTerritoriesPartialFailureReturnsReportedError(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/subscriptionAvailability":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1"}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAvailabilities/avail-1/availableTerritories":
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"territories","id":"CAN"},{"type":"territories","id":"USA"}],"links":{}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/introductoryOffers":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionIntroductoryOffers":
			var payload struct {
				Data struct {
					Relationships struct {
						Territory struct {
							Data struct {
								ID string `json:"id"`
							} `json:"data"`
						} `json:"territory"`
					} `json:"relationships"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if payload.Data.Relationships.Territory.Data.ID == "CAN" {
				return jsonHTTPResponse(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","detail":"duplicate territory"}]}`), nil
			}
			return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"subscriptionIntroductoryOffers","id":"intro-new"}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "create",
			"--subscription-id", "8000000001",
			"--offer-duration", "ONE_MONTH",
			"--offer-mode", "FREE_TRIAL",
			"--number-of-periods", "1",
			"--all-territories",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var summary struct {
		Created  int `json:"created"`
		Failed   int `json:"failed"`
		Failures []struct {
			Territory string `json:"territory"`
		} `json:"failures"`
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse JSON summary: %v", err)
	}
	if summary.Created != 1 || summary.Failed != 1 || len(summary.Failures) != 1 || summary.Failures[0].Territory != "CAN" {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestSubscriptionsIntroductoryOffersCreateAllTerritoriesRejectsConcreteTerritoryAndPricePoint(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "all territories and concrete territory",
			args: []string{
				"subscriptions", "offers", "introductory", "create",
				"--subscription-id", "8000000001",
				"--offer-duration", "ONE_MONTH",
				"--offer-mode", "FREE_TRIAL",
				"--number-of-periods", "1",
				"--all-territories",
				"--territory", "USA",
			},
			wantErr: "Error: --territory and --all-territories are mutually exclusive",
		},
		{
			name: "all territories and price point",
			args: []string{
				"subscriptions", "offers", "introductory", "create",
				"--subscription-id", "8000000001",
				"--offer-duration", "ONE_MONTH",
				"--offer-mode", "FREE_TRIAL",
				"--number-of-periods", "1",
				"--all-territories",
				"--price-point", "price-1",
			},
			wantErr: "Error: --price-point cannot be used with --all-territories or --territory ALL",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			_, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected flag.ErrHelp, got %v", err)
				}
			})
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}
