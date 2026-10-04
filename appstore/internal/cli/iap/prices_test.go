package iap

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

func TestParseIAPPriceScheduleIncluded_DecodesDatesFromResourceID(t *testing.T) {
	raw := []byte(`[
		{
			"type":"inAppPurchasePrices",
			"id":"eyJzIjoiNjc0OTI3MzQ1NyIsInQiOiJNVVMiLCJwIjoiMTAzNTciLCJzZCI6MC4wLCJlZCI6MTc3MTIyODgwMC4wMDAwMDAwMDB9",
			"attributes":{}
		},
		{
			"type":"inAppPurchasePrices",
			"id":"eyJzIjoiNjc0OTI3MzQ1NyIsInQiOiJNVVMiLCJwIjoiMTAzODciLCJzZCI6MTc3MTIyODgwMC4wMDAwMDAwMDAsImVkIjowLjB9",
			"attributes":{}
		}
	]`)

	entries, _, err := parseIAPPriceScheduleIncluded(raw)
	if err != nil {
		t.Fatalf("parseIAPPriceScheduleIncluded returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	now := time.Date(2026, time.February, 7, 0, 0, 0, 0, time.UTC)
	changes := buildScheduledChanges(entries, now, "")
	if len(changes) != 1 {
		t.Fatalf("expected 1 scheduled change, got %d", len(changes))
	}

	change := changes[0]
	if change.Territory != "MUS" {
		t.Fatalf("expected territory MUS, got %q", change.Territory)
	}
	if change.FromPricePoint != "10357" {
		t.Fatalf("expected from price point 10357, got %q", change.FromPricePoint)
	}
	if change.ToPricePoint != "10387" {
		t.Fatalf("expected to price point 10387, got %q", change.ToPricePoint)
	}
	if change.EffectiveDate != "2026-02-16" {
		t.Fatalf("expected effective date 2026-02-16, got %q", change.EffectiveDate)
	}
}

func TestBuildScheduledChanges_UsesUSPacificDay(t *testing.T) {
	entries := []iapPriceEntry{
		newIAPPriceEntry("USA", "old", "", "2026-10-01", true),
		newIAPPriceEntry("USA", "new", "2026-10-01", "", true),
	}

	// 17:30 PDT on 2026-09-30: the 2026-10-01 change is still upcoming.
	changes := buildScheduledChanges(entries, time.Date(2026, time.October, 1, 0, 30, 0, 0, time.UTC), "")
	if len(changes) != 1 || changes[0].FromPricePoint != "old" || changes[0].ToPricePoint != "new" {
		t.Fatalf("expected the old-to-new change at 00:30 UTC, got %+v", changes)
	}

	// Pacific midnight: the change has taken effect.
	changes = buildScheduledChanges(entries, time.Date(2026, time.October, 1, 7, 0, 0, 0, time.UTC), "")
	if len(changes) != 0 {
		t.Fatalf("expected no scheduled changes after Pacific midnight, got %+v", changes)
	}
}

func TestFindActivePriceEntry_ExcludesPriceOnItsEndDate(t *testing.T) {
	today := time.Date(2026, time.September, 30, 0, 0, 0, 0, time.UTC)

	ending := []iapPriceEntry{newIAPPriceEntry("USA", "ending", "2026-01-01", "2026-09-30", true)}
	if entry, ok := findActivePriceEntry(ending, "USA", today); ok {
		t.Fatalf("expected no active price on its end date, got %+v", entry)
	}

	endingTomorrow := []iapPriceEntry{newIAPPriceEntry("USA", "ending", "2026-01-01", "2026-10-01", true)}
	if _, ok := findActivePriceEntry(endingTomorrow, "USA", today); !ok {
		t.Fatal("expected the price ending tomorrow to be active")
	}
}

func TestIsFutureSetupStartDate_UsesUSPacificDay(t *testing.T) {
	tests := []struct {
		startDate string
		now       time.Time
		want      bool
	}{
		{startDate: "2026-10-01", now: time.Date(2026, time.October, 1, 0, 30, 0, 0, time.UTC), want: true},
		{startDate: "2026-10-01", now: time.Date(2026, time.October, 1, 7, 0, 0, 0, time.UTC), want: false},
		{startDate: "2026-09-30", now: time.Date(2026, time.October, 1, 0, 30, 0, 0, time.UTC), want: false},
		{startDate: "", now: time.Date(2026, time.October, 1, 0, 30, 0, 0, time.UTC), want: false},
	}
	for _, test := range tests {
		if got := isFutureSetupStartDate(test.startDate, test.now); got != test.want {
			t.Fatalf("isFutureSetupStartDate(%q, %s) = %t, want %t", test.startDate, test.now.Format(time.RFC3339), got, test.want)
		}
	}
}

func TestResolveIAPPriceSummaries_ContextCancelledReturnsError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	summaries, err := resolveIAPPriceSummaries(
		ctx,
		nil,
		[]asc.Resource[asc.InAppPurchaseV2Attributes]{
			{ID: "iap-1"},
		},
		"",
		time.Now().UTC(),
	)
	if err == nil {
		t.Fatalf("expected error for cancelled context")
	}
	if !strings.Contains(err.Error(), "context cancelled") {
		t.Fatalf("expected context cancelled error, got %v", err)
	}
	if summaries != nil {
		t.Fatalf("expected nil summaries on cancelled context, got %#v", summaries)
	}
}

func TestParseManualSchedulePricePointValues_DecodesLegacyPricePointIDs(t *testing.T) {
	raw := []byte(`[
		{
			"type":"inAppPurchasePricePoints",
			"id":"eyJzIjoiMTU1OTI5NDEzOSIsInQiOiJVU0EiLCJwIjoiMyJ9",
			"attributes":{"customerPrice":"2.99","proceeds":"2.54"}
		},
		{
			"type":"territories",
			"id":"USA",
			"attributes":{"currency":"USD"}
		}
	]`)

	values, currency, err := parseManualSchedulePricePointValues(raw, "USA")
	if err != nil {
		t.Fatalf("parseManualSchedulePricePointValues returned error: %v", err)
	}
	if currency != "USD" {
		t.Fatalf("expected currency USD, got %q", currency)
	}
	value, ok := values["3"]
	if !ok {
		t.Fatalf("expected decoded point id 3 in values map")
	}
	if value.CustomerPrice != "2.99" || value.Proceeds != "2.54" {
		t.Fatalf("unexpected value: %#v", value)
	}
}
