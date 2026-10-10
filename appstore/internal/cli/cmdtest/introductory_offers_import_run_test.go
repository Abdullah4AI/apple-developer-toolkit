package cmdtest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
)

func TestSubscriptionsIntroductoryOffersImport_CreateSuccessSummary(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000003/introductoryOffers" {
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		}
		requestCount++
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionIntroductoryOffers" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}

		var payload map[string]any
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		data := payload["data"].(map[string]any)
		attrs := data["attributes"].(map[string]any)
		relationships := data["relationships"].(map[string]any)
		territory := relationships["territory"].(map[string]any)["data"].(map[string]any)["id"]

		if attrs["duration"] != "ONE_WEEK" {
			t.Fatalf("expected ONE_WEEK duration, got %#v", attrs["duration"])
		}
		if attrs["offerMode"] != "FREE_TRIAL" {
			t.Fatalf("expected FREE_TRIAL offerMode, got %#v", attrs["offerMode"])
		}
		if attrs["numberOfPeriods"] != float64(1) {
			t.Fatalf("expected numberOfPeriods 1, got %#v", attrs["numberOfPeriods"])
		}

		switch requestCount {
		case 1:
			if territory != "USA" {
				t.Fatalf("expected USA territory, got %#v", territory)
			}
		case 2:
			if territory != "AFG" {
				t.Fatalf("expected AFG territory, got %#v", territory)
			}
		default:
			t.Fatalf("unexpected request count %d", requestCount)
		}

		body := `{"data":{"type":"subscriptionIntroductoryOffers","id":"offer-1"}}`
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	csvPath := writeTempIntroOffersCSV(t, "territory\nUSA\nAfghanistan\n")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	type importSummary struct {
		DryRun  bool `json:"dryRun"`
		Total   int  `json:"total"`
		Created int  `json:"created"`
		Failed  int  `json:"failed"`
	}

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "import",
			"--subscription-id", "8000000003",
			"--input", csvPath,
			"--offer-duration", "ONE_WEEK",
			"--offer-mode", "FREE_TRIAL",
			"--number-of-periods", "1",
			"--confirm",
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

	var summary importSummary
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse JSON summary: %v", err)
	}
	if summary.DryRun {
		t.Fatalf("expected dryRun=false")
	}
	if summary.Total != 2 || summary.Created != 2 || summary.Failed != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if requestCount != 2 {
		t.Fatalf("expected 2 requests, got %d", requestCount)
	}
}

func TestSubscriptionsIntroductoryOffersImport_DryRunAcceptsSupportedThreeLetterTerritoryWithoutDisplayName(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request during dry-run: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	csvPath := writeTempIntroOffersCSV(t, "territory\nANT\n")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	type importSummary struct {
		DryRun  bool `json:"dryRun"`
		Total   int  `json:"total"`
		Created int  `json:"created"`
		Failed  int  `json:"failed"`
	}

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "import",
			"--subscription-id", "8000000003",
			"--input", csvPath,
			"--offer-duration", "ONE_WEEK",
			"--offer-mode", "FREE_TRIAL",
			"--number-of-periods", "1",
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

	var summary importSummary
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse JSON summary: %v", err)
	}
	if !summary.DryRun {
		t.Fatalf("expected dryRun=true")
	}
	if summary.Total != 1 || summary.Created != 1 || summary.Failed != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestSubscriptionsIntroductoryOffersImport_PartialFailureReturnsReportedErrorAndSummary(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Chdir(t.TempDir())

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000003/introductoryOffers" {
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		}
		requestCount++
		if req.Method != http.MethodPost || req.URL.Path != "/v1/subscriptionIntroductoryOffers" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}

		switch requestCount {
		case 1, 3:
			body := `{"data":{"type":"subscriptionIntroductoryOffers","id":"offer-1"}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			body := `{"errors":[{"status":"422","title":"Unprocessable Entity","detail":"invalid intro offer"}]}`
			return &http.Response{
				StatusCode: http.StatusUnprocessableEntity,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	csvPath := writeTempIntroOffersCSV(t, "territory\nUSA\nAFG\nCAN\n")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	type importFailure struct {
		Row int `json:"row"`
	}
	type importSummary struct {
		Created  int             `json:"created"`
		Failed   int             `json:"failed"`
		Failures []importFailure `json:"failures"`
	}

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "import",
			"--subscription-id", "8000000003",
			"--input", csvPath,
			"--offer-duration", "ONE_WEEK",
			"--offer-mode", "FREE_TRIAL",
			"--number-of-periods", "1",
			"--confirm",
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

	var summary importSummary
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse JSON summary: %v", err)
	}
	if summary.Created != 2 || summary.Failed != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(summary.Failures) != 1 || summary.Failures[0].Row != 2 {
		t.Fatalf("expected one row-2 failure, got %+v", summary.Failures)
	}
	if requestCount != 3 {
		t.Fatalf("expected 3 requests, got %d", requestCount)
	}
}

func TestSubscriptionsIntroductoryOffersImport_StopOnFirstFailureWhenRequested(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Chdir(t.TempDir())

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000003/introductoryOffers" {
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		}
		requestCount++
		if req.Method != http.MethodPost || req.URL.Path != "/v1/subscriptionIntroductoryOffers" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}

		switch requestCount {
		case 1:
			body := `{"data":{"type":"subscriptionIntroductoryOffers","id":"offer-1"}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			body := `{"errors":[{"status":"422","title":"Unprocessable Entity","detail":"invalid intro offer"}]}`
			return &http.Response{
				StatusCode: http.StatusUnprocessableEntity,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	csvPath := writeTempIntroOffersCSV(t, "territory\nUSA\nAFG\nCAN\n")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	type importSummary struct {
		Created int `json:"created"`
		Failed  int `json:"failed"`
	}

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "import",
			"--subscription-id", "8000000003",
			"--input", csvPath,
			"--offer-duration", "ONE_WEEK",
			"--offer-mode", "FREE_TRIAL",
			"--number-of-periods", "1",
			"--continue-on-error=false",
			"--confirm",
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

	var summary importSummary
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse JSON summary: %v", err)
	}
	if summary.Created != 1 || summary.Failed != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if requestCount != 2 {
		t.Fatalf("expected 2 requests before stop, got %d", requestCount)
	}
}

func TestSubscriptionsIntroductoryOffersImport_RowValuesOverrideDefaults(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000003/introductoryOffers" {
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		}
		if req.Method != http.MethodPost || req.URL.Path != "/v1/subscriptionIntroductoryOffers" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}

		var payload map[string]any
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		data := payload["data"].(map[string]any)
		attrs := data["attributes"].(map[string]any)
		relationships := data["relationships"].(map[string]any)
		territory := relationships["territory"].(map[string]any)["data"].(map[string]any)
		pricePoint := relationships["subscriptionPricePoint"].(map[string]any)["data"].(map[string]any)

		if attrs["duration"] != "ONE_MONTH" {
			t.Fatalf("expected row duration ONE_MONTH, got %#v", attrs["duration"])
		}
		if attrs["offerMode"] != "PAY_AS_YOU_GO" {
			t.Fatalf("expected row offerMode PAY_AS_YOU_GO, got %#v", attrs["offerMode"])
		}
		if attrs["numberOfPeriods"] != float64(3) {
			t.Fatalf("expected row numberOfPeriods 3, got %#v", attrs["numberOfPeriods"])
		}
		if attrs["startDate"] != "2026-04-01" {
			t.Fatalf("expected row startDate 2026-04-01, got %#v", attrs["startDate"])
		}
		if attrs["endDate"] != "2026-05-01" {
			t.Fatalf("expected row endDate 2026-05-01, got %#v", attrs["endDate"])
		}
		if territory["id"] != "CAN" {
			t.Fatalf("expected CAN territory, got %#v", territory["id"])
		}
		if pricePoint["id"] != "pp-can-1" {
			t.Fatalf("expected row price point pp-can-1, got %#v", pricePoint["id"])
		}

		body := `{"data":{"type":"subscriptionIntroductoryOffers","id":"offer-1"}}`
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	csvPath := writeTempIntroOffersCSV(t, "territory,offer_mode,offer_duration,number_of_periods,start_date,end_date,price_point_id\nCAN,PAY_AS_YOU_GO,ONE_MONTH,3,2026-04-01,2026-05-01,pp-can-1\n")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "import",
			"--subscription-id", "8000000003",
			"--input", csvPath,
			"--offer-duration", "ONE_WEEK",
			"--offer-mode", "FREE_TRIAL",
			"--number-of-periods", "1",
			"--start-date", "2026-03-01",
			"--end-date", "2026-03-15",
			"--confirm",
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
	if !strings.Contains(stdout, `"created":1`) {
		t.Fatalf("expected created summary in stdout, got %q", stdout)
	}
}

func TestSubscriptionsIntroductoryOffersImport_NormalizesInheritedDefaultEnums(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000003/introductoryOffers" {
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		}
		if req.Method != http.MethodPost || req.URL.Path != "/v1/subscriptionIntroductoryOffers" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}

		var payload map[string]any
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		data := payload["data"].(map[string]any)
		attrs := data["attributes"].(map[string]any)

		if attrs["duration"] != "ONE_WEEK" {
			t.Fatalf("expected normalized duration ONE_WEEK, got %#v", attrs["duration"])
		}
		if attrs["offerMode"] != "FREE_TRIAL" {
			t.Fatalf("expected normalized offerMode FREE_TRIAL, got %#v", attrs["offerMode"])
		}

		body := `{"data":{"type":"subscriptionIntroductoryOffers","id":"offer-1"}}`
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	csvPath := writeTempIntroOffersCSV(t, "territory\nUSA\n")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "import",
			"--subscription-id", "8000000003",
			"--input", csvPath,
			"--offer-duration", "one_week",
			"--offer-mode", "free_trial",
			"--number-of-periods", "1",
			"--confirm",
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
	if !strings.Contains(stdout, `"created":1`) {
		t.Fatalf("expected created summary in stdout, got %q", stdout)
	}
}

func TestSubscriptionsIntroductoryOffersImport_SkipsExactExistingOffer(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	postCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000003/introductoryOffers":
			assertIntroductoryOfferImportStateQuery(t, req)
			body := `{"data":[{"type":"subscriptionIntroductoryOffers","id":"offer-existing","attributes":{"startDate":"2020-01-01","duration":"ONE_WEEK","offerMode":"FREE_TRIAL","numberOfPeriods":1,"targetSubscriptionPlanType":"UPFRONT"},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}}],"links":{}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		case req.Method == http.MethodPost:
			postCount++
			t.Fatalf("exact existing offer must not be posted again")
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	csvPath := writeTempIntroOffersCSV(t, "territory\nUSA\n")
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "import",
			"--subscription-id", "8000000003",
			"--input", csvPath,
			"--offer-duration", "ONE_WEEK",
			"--offer-mode", "FREE_TRIAL",
			"--number-of-periods", "1",
			"--output", "json",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	var summary struct {
		Created int `json:"created"`
		Skipped int `json:"skipped"`
		Results []struct {
			Status string `json:"status"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse summary: %v", err)
	}
	if summary.Created != 0 || summary.Skipped != 1 || postCount != 0 || len(summary.Results) != 1 || summary.Results[0].Status != "skipped" {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestSubscriptionsIntroductoryOffersImport_ReconcilesAmbiguousCreateWithoutReplay(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_MAX_RETRIES", "2")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	readCount := 0
	postCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000003/introductoryOffers":
			assertIntroductoryOfferImportStateQuery(t, req)
			readCount++
			if readCount == 1 {
				return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
			}
			body := `{"data":[{"type":"subscriptionIntroductoryOffers","id":"offer-created","attributes":{"startDate":"2020-01-01","duration":"ONE_WEEK","offerMode":"FREE_TRIAL","numberOfPeriods":1,"targetSubscriptionPlanType":"UPFRONT"},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}}],"links":{}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionIntroductoryOffers":
			postCount++
			return jsonHTTPResponse(http.StatusGatewayTimeout, `{"errors":[{"status":"504","code":"UNEXPECTED_ERROR","detail":"ambiguous"}]}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	csvPath := writeTempIntroOffersCSV(t, "territory\nUSA\n")
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "import",
			"--subscription-id", "8000000003",
			"--input", csvPath,
			"--offer-duration", "ONE_WEEK",
			"--offer-mode", "FREE_TRIAL",
			"--number-of-periods", "1",
			"--output", "json",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	var summary struct {
		Reconciled int `json:"reconciled"`
		Failed     int `json:"failed"`
		Results    []struct {
			Status string `json:"status"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse summary: %v", err)
	}
	if summary.Reconciled != 1 || summary.Failed != 0 || postCount != 1 || readCount != 2 || len(summary.Results) != 1 || summary.Results[0].Status != "reconciled" {
		t.Fatalf("unexpected recovery: summary=%+v posts=%d reads=%d", summary, postCount, readCount)
	}
}

func TestSubscriptionsIntroductoryOffersImport_PaginatesIndexAndUsesEncodedTerritoryFallback(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	readCount := 0
	encodedUSAID := base64.RawURLEncoding.EncodeToString([]byte(`{"i":"USA"}`))
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000003/introductoryOffers":
			assertIntroductoryOfferImportStateQuery(t, req)
			readCount++
			if req.URL.Query().Get("cursor") == "page-2" {
				body := `{"data":[{"type":"subscriptionIntroductoryOffers","id":"offer-afg","attributes":{"startDate":"2020-01-01","duration":"ONE_WEEK","offerMode":"FREE_TRIAL","numberOfPeriods":1,"targetSubscriptionPlanType":"UPFRONT"},"relationships":{"territory":{"data":{"type":"territories","id":"AFG"}}}}],"links":{}}`
				return jsonHTTPResponse(http.StatusOK, body), nil
			}
			body := `{"data":[{"type":"subscriptionIntroductoryOffers","id":"` + encodedUSAID + `","attributes":{"startDate":"2020-01-01","duration":"ONE_WEEK","offerMode":"FREE_TRIAL","numberOfPeriods":1,"targetSubscriptionPlanType":"UPFRONT"}}],"links":{"next":"/v1/subscriptions/8000000003/introductoryOffers?cursor=page-2"}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		case req.Method == http.MethodPost:
			t.Fatalf("indexed exact offers must not be posted again")
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	csvPath := writeTempIntroOffersCSV(t, "territory\nUSA\nAFG\n")
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "import",
			"--subscription-id", "8000000003",
			"--input", csvPath,
			"--offer-duration", "ONE_WEEK",
			"--offer-mode", "FREE_TRIAL",
			"--number-of-periods", "1",
			"--output", "json",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	var summary struct {
		Skipped int `json:"skipped"`
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse summary: %v", err)
	}
	if summary.Skipped != 2 || readCount != 2 {
		t.Fatalf("expected two skips from two indexed pages, summary=%+v reads=%d", summary, readCount)
	}
}

func assertIntroductoryOfferImportStateQuery(t *testing.T, req *http.Request) {
	t.Helper()
	wantFields := "startDate,endDate,duration,offerMode,numberOfPeriods,targetSubscriptionPlanType,territory,subscriptionPricePoint"
	if got := req.URL.Query().Get("fields[subscriptionIntroductoryOffers]"); got != wantFields {
		t.Fatalf("unexpected introductory offer fields: %q", got)
	}
	if got := req.URL.Query().Get("include"); got != "territory,subscriptionPricePoint" {
		t.Fatalf("unexpected introductory offer include: %q", got)
	}
	if got := req.URL.Query().Get("limit"); got != "200" {
		t.Fatalf("unexpected introductory offer limit: %q", got)
	}
}

func TestSubscriptionsIntroductoryOffersImport_PaidOfferMatchesPricePoint(t *testing.T) {
	tests := []struct {
		name          string
		pricePointID  string
		wantCreated   int
		wantSkipped   int
		wantPostCount int
	}{
		{name: "exact price point skips", pricePointID: "point-1", wantSkipped: 1},
		{name: "different price point creates", pricePointID: "point-2", wantCreated: 1, wantPostCount: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			postCount := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000003/introductoryOffers":
					assertIntroductoryOfferImportStateQuery(t, req)
					body := `{"data":[{"type":"subscriptionIntroductoryOffers","id":"offer-paid","attributes":{"startDate":"2020-01-01","duration":"ONE_MONTH","offerMode":"PAY_AS_YOU_GO","numberOfPeriods":1,"targetSubscriptionPlanType":"UPFRONT"},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"point-1"}}}}],"links":{}}`
					return jsonHTTPResponse(http.StatusOK, body), nil
				case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionIntroductoryOffers":
					postCount++
					body, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("ReadAll() error: %v", err)
					}
					if !strings.Contains(string(body), `"id":"`+test.pricePointID+`"`) {
						t.Fatalf("expected target price point in payload, got %s", body)
					}
					return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"subscriptionIntroductoryOffers","id":"offer-new"}}`), nil
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			})

			csvPath := writeTempIntroOffersCSV(t, "territory,offer_mode,offer_duration,number_of_periods,price_point_id\nUSA,PAY_AS_YOU_GO,ONE_MONTH,1,"+test.pricePointID+"\n")
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, _ := captureOutput(t, func() {
				if err := root.Parse([]string{
					"subscriptions", "offers", "introductory", "import",
					"--subscription-id", "8000000003", "--input", csvPath, "--output", "json",
					"--confirm",
				}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})
			var summary struct {
				Created int `json:"created"`
				Skipped int `json:"skipped"`
			}
			if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
				t.Fatalf("parse summary: %v", err)
			}
			if summary.Created != test.wantCreated || summary.Skipped != test.wantSkipped || postCount != test.wantPostCount {
				t.Fatalf("unexpected summary=%+v posts=%d", summary, postCount)
			}
		})
	}
}

func TestSubscriptionsIntroductoryOffersImport_RetriesTimedOutInitialStateRead(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")
	t.Setenv("ASC_TIMEOUT", "50ms")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	readCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/8000000003/introductoryOffers" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, errors.New("test transport assertion failed")
		}
		assertIntroductoryOfferImportStateQuery(t, req)
		readCount++
		if readCount == 1 {
			<-req.Context().Done()
			return nil, req.Context().Err()
		}
		if err := req.Context().Err(); err != nil {
			t.Errorf("expected fresh request context, got %v", err)
			return nil, errors.New("test transport assertion failed")
		}
		body := `{"data":[{"type":"subscriptionIntroductoryOffers","id":"offer-existing","attributes":{"startDate":"1999-01-01","duration":"ONE_WEEK","offerMode":"FREE_TRIAL","numberOfPeriods":1,"targetSubscriptionPlanType":"UPFRONT"},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}}],"links":{}}`
		return jsonHTTPResponse(http.StatusOK, body), nil
	})

	csvPath := writeTempIntroOffersCSV(t, "territory\nUSA\n")
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "import",
			"--subscription-id", "8000000003", "--input", csvPath,
			"--offer-duration", "ONE_WEEK", "--offer-mode", "FREE_TRIAL", "--number-of-periods", "1", "--output", "json",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		var runErr error
		synctest.Test(t, func(*testing.T) { runErr = root.Run(context.Background()) })
		if err := runErr; err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if !strings.Contains(stdout, `"skipped":1`) || readCount != 2 {
		t.Fatalf("expected fresh-context retry and skip, reads=%d output=%s", readCount, stdout)
	}
}

func TestSubscriptionsIntroductoryOffersImport_PrintsFailuresWhenArtifactWriteFails(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_MAX_RETRIES", "0")
	t.Chdir(t.TempDir())
	if err := os.WriteFile(".asc", []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000003/introductoryOffers":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionIntroductoryOffers":
			return jsonHTTPResponse(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","detail":"invalid offer"}]}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	csvPath := writeTempIntroOffersCSV(t, "territory\nUSA\n")
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "import",
			"--subscription-id", "8000000003", "--input", csvPath,
			"--offer-duration", "ONE_WEEK", "--offer-mode", "FREE_TRIAL", "--number-of-periods", "1", "--output", "json",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}
	var summary struct {
		Failed               int    `json:"failed"`
		FailureArtifactError string `json:"failureArtifactError"`
		Results              []struct {
			Status         string `json:"status"`
			Error          string `json:"error"`
			TargetPlanType string `json:"targetSubscriptionPlanType"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse summary: %v\n%s", err, stdout)
	}
	if summary.Failed != 1 || summary.FailureArtifactError == "" || len(summary.Results) != 1 || summary.Results[0].Status != "failed" || summary.Results[0].Error == "" || summary.Results[0].TargetPlanType != "UPFRONT" {
		t.Fatalf("unexpected failure summary: %+v", summary)
	}
}

func TestSubscriptionsIntroductoryOffersImport_WritesVersionedFailureArtifact(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_MAX_RETRIES", "0")
	t.Chdir(t.TempDir())

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000003/introductoryOffers":
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionIntroductoryOffers":
			return jsonHTTPResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"UNEXPECTED_ERROR","detail":"still unavailable"}]}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	csvPath := writeTempIntroOffersCSV(t, "territory\nUSA\n")
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "import",
			"--subscription-id", "8000000003",
			"--input", csvPath,
			"--offer-duration", "ONE_WEEK",
			"--offer-mode", "FREE_TRIAL",
			"--number-of-periods", "1",
			"--output", "json",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}

	var summary struct {
		FailureArtifactPath string `json:"failureArtifactPath"`
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse summary: %v", err)
	}
	data, err := os.ReadFile(summary.FailureArtifactPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	var artifact struct {
		SchemaVersion int `json:"schemaVersion"`
		Failed        int `json:"failed"`
		Results       []struct {
			Status          string `json:"status"`
			Territory       string `json:"territory"`
			OfferMode       string `json:"offerMode"`
			OfferDuration   string `json:"offerDuration"`
			NumberOfPeriods int    `json:"numberOfPeriods"`
			StartDate       string `json:"startDate"`
			EndDate         string `json:"endDate"`
			PricePointID    string `json:"pricePointId"`
			TargetPlanType  string `json:"targetSubscriptionPlanType"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatalf("parse artifact: %v", err)
	}
	if artifact.SchemaVersion != 1 || artifact.Failed != 1 || len(artifact.Results) != 1 || artifact.Results[0].Status != "failed" {
		t.Fatalf("unexpected artifact: %+v", artifact)
	}
	result := artifact.Results[0]
	if result.Territory != "USA" || result.OfferMode != "FREE_TRIAL" || result.OfferDuration != "ONE_WEEK" || result.NumberOfPeriods != 1 || result.StartDate != "" || result.EndDate != "" || result.PricePointID != "" || result.TargetPlanType != "UPFRONT" {
		t.Fatalf("artifact is missing desired introductory-offer state: %+v", result)
	}
}
