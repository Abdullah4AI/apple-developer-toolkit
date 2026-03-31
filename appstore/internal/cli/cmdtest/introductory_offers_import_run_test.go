package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
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
			"--subscription-id", "SUB_ID",
			"--input", csvPath,
			"--offer-duration", "ONE_WEEK",
			"--offer-mode", "FREE_TRIAL",
			"--number-of-periods", "1",
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
			"--subscription-id", "SUB_ID",
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

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
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
			"--subscription-id", "SUB_ID",
			"--input", csvPath,
			"--offer-duration", "ONE_WEEK",
			"--offer-mode", "FREE_TRIAL",
			"--number-of-periods", "1",
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

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
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
			"--subscription-id", "SUB_ID",
			"--input", csvPath,
			"--offer-duration", "ONE_WEEK",
			"--offer-mode", "FREE_TRIAL",
			"--number-of-periods", "1",
			"--continue-on-error=false",
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
			"--subscription-id", "SUB_ID",
			"--input", csvPath,
			"--offer-duration", "ONE_WEEK",
			"--offer-mode", "FREE_TRIAL",
			"--number-of-periods", "1",
			"--start-date", "2026-03-01",
			"--end-date", "2026-03-15",
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
			"--subscription-id", "SUB_ID",
			"--input", csvPath,
			"--offer-duration", "one_week",
			"--offer-mode", "free_trial",
			"--number-of-periods", "1",
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
