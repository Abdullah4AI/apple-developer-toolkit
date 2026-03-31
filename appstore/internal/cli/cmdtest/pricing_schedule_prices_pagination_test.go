package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPricingScheduleManualPricesLimit(t *testing.T) {
	setupAuth(t)

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/appPriceSchedules/schedule-1/manualPrices" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		if req.URL.Query().Get("limit") != "175" {
			t.Fatalf("expected limit=175, got %q", req.URL.Query().Get("limit"))
		}

		body := `{"data":[{"type":"appPrices","id":"manual-price-175"}],"links":{"next":""}}`
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
			"pricing", "schedule", "manual-prices",
			"--schedule", "schedule-1",
			"--limit", "175",
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
	if !strings.Contains(stdout, `"id":"manual-price-175"`) {
		t.Fatalf("expected manual price output, got %q", stdout)
	}
}

func TestPricingScheduleManualPricesPaginate(t *testing.T) {
	setupAuth(t)

	const firstURL = "https://api.appstoreconnect.apple.com/v1/appPriceSchedules/schedule-1/manualPrices?limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/appPriceSchedules/schedule-1/manualPrices?cursor=Mg"

	requestCount := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.String() != firstURL {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"appPrices","id":"manual-price-1"}],"links":{"next":"` + secondURL + `"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.String() != secondURL {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"appPrices","id":"manual-price-2"}],"links":{"next":""}}`
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
			"pricing", "schedule", "manual-prices",
			"--schedule", "schedule-1",
			"--paginate",
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
	if !strings.Contains(stdout, `"id":"manual-price-1"`) || !strings.Contains(stdout, `"id":"manual-price-2"`) {
		t.Fatalf("expected paginated manual price output, got %q", stdout)
	}
}

func TestPricingScheduleAutomaticPricesNext(t *testing.T) {
	setupAuth(t)

	const nextURL = "https://api.appstoreconnect.apple.com/v1/appPriceSchedules/schedule-1/automaticPrices?cursor=Mg"

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.String() != nextURL {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}

		body := `{"data":[{"type":"appPrices","id":"automatic-price-next"}],"links":{"next":""}}`
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
			"pricing", "schedule", "automatic-prices",
			"--next", nextURL,
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
	if !strings.Contains(stdout, `"id":"automatic-price-next"`) {
		t.Fatalf("expected automatic price output, got %q", stdout)
	}
}

func TestPricingScheduleManualPricesRejectsInvalidLimit(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"pricing", "schedule", "manual-prices",
			"--schedule", "schedule-1",
			"--limit", "201",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: pricing schedule manual-prices: --limit must be between 1 and 200") {
		t.Fatalf("expected invalid limit usage error, got %q", stderr)
	}
}

func TestPricingScheduleAutomaticPricesRejectsInvalidNextURL(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"pricing", "schedule", "automatic-prices",
			"--next", "http://api.appstoreconnect.apple.com/v1/appPriceSchedules/schedule-1/automaticPrices?cursor=AQ",
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
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(runErr.Error(), "pricing schedule automatic-prices: --next must be an App Store Connect URL") {
		t.Fatalf("expected invalid next error, got %v", runErr)
	}
}
