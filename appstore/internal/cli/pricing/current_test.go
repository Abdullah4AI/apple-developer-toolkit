package pricing

import (
	"testing"
	"time"
)

func TestBuildAppCurrentPricingResult_UsesSingleTimestamp(t *testing.T) {
	now := time.Date(2024, time.December, 31, 23, 59, 59, 0, time.UTC)

	entries := []appPriceEntry{
		newAppPriceEntry("USA", "old-free", "2024-01-01", "2024-12-31", true),
		newAppPriceEntry("USA", "new-paid", "2025-01-01", "", true),
	}
	values := map[string]appPricePointValue{
		appPricePointLookupKey("USA", "old-free"): {CustomerPrice: "0.00", Proceeds: "0.00"},
		appPricePointLookupKey("USA", "new-paid"): {CustomerPrice: "1.99", Proceeds: "1.39"},
	}
	currencies := map[string]string{
		"USA": "USD",
	}

	result, err := buildAppCurrentPricingResult("app-1", "USA", entries, values, currencies, nil, false, now)
	if err != nil {
		t.Fatalf("buildAppCurrentPricingResult() error = %v", err)
	}

	if !result.IsFree {
		t.Fatalf("expected IsFree=true at %s, got false", now.Format(time.RFC3339))
	}
	if result.CustomerPrice != "0.00" {
		t.Fatalf("expected current customerPrice 0.00 at %s, got %q", now.Format(time.RFC3339), result.CustomerPrice)
	}
	if result.Proceeds != "0.00" {
		t.Fatalf("expected current proceeds 0.00 at %s, got %q", now.Format(time.RFC3339), result.Proceeds)
	}
	if result.Currency != "USD" {
		t.Fatalf("expected currency USD, got %q", result.Currency)
	}
}
