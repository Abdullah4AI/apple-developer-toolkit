package pricing

import (
	"testing"
	"time"
)

func TestBuildAppCurrentPricingResult_UsesSingleTimestamp(t *testing.T) {
	now := time.Date(2024, time.December, 31, 23, 59, 59, 0, time.UTC)

	entries := []appPriceEntry{
		newAppPriceEntry("USA", "old-free", "2024-01-01", "2025-01-01", true),
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

func TestBuildAppCurrentPricingResultDateBoundaries(t *testing.T) {
	values := map[string]appPricePointValue{
		appPricePointLookupKey("USA", "old"): {CustomerPrice: "0.00", Proceeds: "0.00"},
		appPricePointLookupKey("USA", "new"): {CustomerPrice: "1.99", Proceeds: "1.69"},
	}
	currencies := map[string]string{"USA": "USD"}
	successive := []appPriceEntry{
		newAppPriceEntry("USA", "old", "2026-01-01", "2026-10-01", true),
		newAppPriceEntry("USA", "new", "2026-10-01", "", true),
	}

	tests := []struct {
		name      string
		entries   []appPriceEntry
		now       string
		wantPrice string
	}{
		{name: "price ending tomorrow applies", entries: successive[:1], now: "2026-09-30T20:00:00Z", wantPrice: "0.00"},
		{name: "price ending today does not apply", entries: successive[:1], now: "2026-10-01T20:00:00Z"},
		{name: "successor applies on the end date", entries: successive, now: "2026-10-01T20:00:00Z", wantPrice: "1.99"},
		{name: "UTC midnight is still the previous Pacific day", entries: successive, now: "2026-10-01T00:30:00Z", wantPrice: "0.00"},
		{name: "ending price lapses at Pacific midnight", entries: successive[:1], now: "2026-10-01T07:00:00Z"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			now, err := time.Parse(time.RFC3339, test.now)
			if err != nil {
				t.Fatalf("parse now: %v", err)
			}
			result, err := buildAppCurrentPricingResult("app-1", "USA", test.entries, values, currencies, nil, false, now)
			if test.wantPrice == "" {
				if err == nil {
					t.Fatalf("expected no current base price, got %+v", result)
				}
				return
			}
			if err != nil {
				t.Fatalf("buildAppCurrentPricingResult() error = %v", err)
			}
			if result.CustomerPrice != test.wantPrice {
				t.Fatalf("customerPrice = %q, want %q", result.CustomerPrice, test.wantPrice)
			}
		})
	}
}

func TestAppPriceActiveOn(t *testing.T) {
	tests := []struct {
		name       string
		start, end string
		now        string
		wantActive bool
		wantKnown  bool
	}{
		{name: "open interval", now: "2026-09-30T20:00:00Z", wantActive: true, wantKnown: true},
		{name: "ends tomorrow", end: "2026-10-01", now: "2026-09-30T20:00:00Z", wantActive: true, wantKnown: true},
		{name: "ends today", end: "2026-09-30", now: "2026-09-30T20:00:00Z", wantKnown: true},
		{name: "starts today", start: "2026-09-30", now: "2026-09-30T20:00:00Z", wantActive: true, wantKnown: true},
		{name: "starts on the UTC date but not the Pacific date", start: "2026-10-01", now: "2026-10-01T00:30:00Z", wantKnown: true},
		{name: "ends on the UTC date but not the Pacific date", end: "2026-10-01", now: "2026-10-01T00:30:00Z", wantActive: true, wantKnown: true},
		{name: "unreadable start date", start: "10/01/2026", now: "2026-09-30T20:00:00Z"},
		{name: "unreadable end date", end: "soon", now: "2026-09-30T20:00:00Z"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			now, err := time.Parse(time.RFC3339, test.now)
			if err != nil {
				t.Fatalf("parse now: %v", err)
			}
			active, known := AppPriceActiveOn(test.start, test.end, now)
			if active != test.wantActive || known != test.wantKnown {
				t.Fatalf("AppPriceActiveOn() = active %t known %t, want active %t known %t", active, known, test.wantActive, test.wantKnown)
			}
		})
	}
}
