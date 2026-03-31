package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

func TestIAPPricingSchedulesManualPricesResolvedJSON(t *testing.T) {
	setupAuth(t)

	const secondURL = "https://api.appstoreconnect.apple.com/v1/inAppPurchasePriceSchedules/schedule-1/manualPrices?cursor=Mg"

	requestCount := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/inAppPurchasePriceSchedules/schedule-1/manualPrices" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			query := req.URL.Query()
			if query.Get("include") != "inAppPurchasePricePoint,territory" {
				t.Fatalf("unexpected include query: %q", query.Get("include"))
			}
			if query.Get("fields[inAppPurchasePrices]") != "manual,startDate,endDate,inAppPurchasePricePoint,territory" {
				t.Fatalf("unexpected price fields: %q", query.Get("fields[inAppPurchasePrices]"))
			}
			if query.Get("fields[inAppPurchasePricePoints]") != "customerPrice,proceeds,territory" {
				t.Fatalf("unexpected price point fields: %q", query.Get("fields[inAppPurchasePricePoints]"))
			}
			body := `{
				"data":[
					{
						"type":"inAppPurchasePrices",
						"id":"manual-price-usa",
						"attributes":{"startDate":"2025-01-01","manual":true},
						"relationships":{
							"territory":{"data":{"type":"territories","id":"USA"}},
							"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-manual-usa"}}
						}
					}
				],
				"included":[
					{"type":"inAppPurchasePricePoints","id":"pp-manual-usa","attributes":{"customerPrice":"9.99","proceeds":"8.49"}},
					{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
				],
				"links":{"next":"` + secondURL + `"}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/inAppPurchasePriceSchedules/schedule-1/manualPrices" {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			query := req.URL.Query()
			if query.Get("cursor") != "Mg" {
				t.Fatalf("expected cursor=Mg, got %q", query.Get("cursor"))
			}
			if query.Get("include") != "inAppPurchasePricePoint,territory" {
				t.Fatalf("unexpected paginated include query: %q", query.Get("include"))
			}
			if query.Get("fields[inAppPurchasePrices]") != "manual,startDate,endDate,inAppPurchasePricePoint,territory" {
				t.Fatalf("unexpected paginated price fields: %q", query.Get("fields[inAppPurchasePrices]"))
			}
			if query.Get("fields[inAppPurchasePricePoints]") != "customerPrice,proceeds,territory" {
				t.Fatalf("unexpected paginated price point fields: %q", query.Get("fields[inAppPurchasePricePoints]"))
			}
			body := `{
				"data":[
					{
						"type":"inAppPurchasePrices",
						"id":"manual-price-gbr",
						"attributes":{"startDate":"2024-06-01","manual":true},
						"relationships":{
							"territory":{"data":{"type":"territories","id":"GBR"}},
							"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-manual-gbr"}}
						}
					}
				],
				"included":[
					{"type":"inAppPurchasePricePoints","id":"pp-manual-gbr","attributes":{"customerPrice":"7.99","proceeds":"6.79"}},
					{"type":"territories","id":"GBR","attributes":{"currency":"GBP"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/inAppPurchasePriceSchedules/schedule-1/automaticPrices" {
				t.Fatalf("unexpected third request: %s %s", req.Method, req.URL.String())
			}
			body := `{
				"data":[
					{
						"type":"inAppPurchasePrices",
						"id":"automatic-price-usa",
						"attributes":{"startDate":"2025-01-01","manual":false},
						"relationships":{
							"territory":{"data":{"type":"territories","id":"USA"}},
							"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-automatic-usa"}}
						}
					}
				],
				"included":[
					{"type":"inAppPurchasePricePoints","id":"pp-automatic-usa","attributes":{"customerPrice":"4.99","proceeds":"3.49"}},
					{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "pricing", "schedules", "manual-prices",
			"--schedule-id", "schedule-1",
			"--resolved",
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

	var result shared.ResolvedPricesResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("json.Unmarshal() error = %v, stdout = %q", err, stdout)
	}
	if len(result.Prices) != 2 {
		t.Fatalf("expected 2 resolved prices, got %+v", result.Prices)
	}
	if result.Prices[0].Territory != "GBR" || result.Prices[0].CustomerPrice != "7.99" {
		t.Fatalf("unexpected GBR row: %+v", result.Prices[0])
	}
	if result.Prices[1].Territory != "USA" || result.Prices[1].CustomerPrice != "9.99" {
		t.Fatalf("unexpected USA row: %+v", result.Prices[1])
	}
	if result.Prices[1].Manual == nil || !*result.Prices[1].Manual {
		t.Fatalf("expected manual USA row, got %+v", result.Prices[1].Manual)
	}
}

func TestIAPPricingSchedulesAutomaticPricesResolvedTable(t *testing.T) {
	setupAuth(t)

	requestCount := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.URL.Path != "/v1/inAppPurchasePriceSchedules/schedule-1/automaticPrices" {
				t.Fatalf("unexpected first request: %s", req.URL.String())
			}
			body := `{
				"data":[
					{
						"type":"inAppPurchasePrices",
						"id":"automatic-price-usa",
						"attributes":{"startDate":"2025-01-01","manual":false},
						"relationships":{
							"territory":{"data":{"type":"territories","id":"USA"}},
							"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-automatic-usa"}}
						}
					}
				],
				"included":[
					{"type":"inAppPurchasePricePoints","id":"pp-automatic-usa","attributes":{"customerPrice":"4.99","proceeds":"3.49"}},
					{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.URL.Path != "/v1/inAppPurchasePriceSchedules/schedule-1/manualPrices" {
				t.Fatalf("unexpected second request: %s", req.URL.String())
			}
			body := `{
				"data":[
					{
						"type":"inAppPurchasePrices",
						"id":"manual-price-usa",
						"attributes":{"startDate":"2025-01-01","manual":true},
						"relationships":{
							"territory":{"data":{"type":"territories","id":"USA"}},
							"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-manual-usa"}}
						}
					}
				],
				"included":[
					{"type":"inAppPurchasePricePoints","id":"pp-manual-usa","attributes":{"customerPrice":"9.99","proceeds":"8.49"}},
					{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "pricing", "schedules", "automatic-prices",
			"--schedule-id", "schedule-1",
			"--resolved",
			"--output", "table",
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
	if !strings.Contains(stdout, "Manual") || !strings.Contains(stdout, "Proceeds") {
		t.Fatalf("expected resolved table headers, got %q", stdout)
	}
	if !strings.Contains(stdout, "USA") || !strings.Contains(stdout, "9.99") || !strings.Contains(stdout, "true") {
		t.Fatalf("expected resolved table row, got %q", stdout)
	}
}

func TestIAPPricingSchedulesManualPricesResolvedRejectsNext(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	const nextURL = "https://api.appstoreconnect.apple.com/v1/inAppPurchasePriceSchedules/schedule-1/manualPrices?cursor=Mg"

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "pricing", "schedules", "manual-prices",
			"--next", nextURL,
			"--resolved",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --resolved cannot be combined with --next") {
		t.Fatalf("expected resolved next usage error, got %q", stderr)
	}
}

func TestIAPPricingSchedulesManualPricesRawOutputUnchanged(t *testing.T) {
	setupAuth(t)

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/inAppPurchasePriceSchedules/schedule-1/manualPrices" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		if req.URL.RawQuery != "" {
			t.Fatalf("expected raw request without resolved query fields, got %q", req.URL.RawQuery)
		}

		body := `{"data":[{"type":"inAppPurchasePrices","id":"price-raw"}],"links":{"next":""}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "pricing", "schedules", "manual-prices",
			"--schedule-id", "schedule-1",
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
	if !strings.Contains(stdout, `"type":"inAppPurchasePrices"`) || strings.Contains(stdout, `"prices":[`) {
		t.Fatalf("expected raw output, got %q", stdout)
	}
}
