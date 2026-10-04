package subscriptions

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

func TestSelectCurrentSubscriptionPriceValue_PicksLatestEffectiveCurrent(t *testing.T) {
	now := time.Date(2026, time.February, 7, 12, 0, 0, 0, time.UTC)

	prices := []asc.Resource[asc.SubscriptionPriceAttributes]{
		newSubscriptionPriceResource("pp-old", "2020-01-01", false),
		newSubscriptionPriceResource("pp-current", "2024-01-01", false),
		newSubscriptionPriceResource("pp-future", "2030-01-01", false),
	}

	values := map[string]subscriptionPricePointValue{
		"pp-old":     {CustomerPrice: "1.99", Proceeds: "1.40", ProceedsYear2: "1.60"},
		"pp-current": {CustomerPrice: "9.99", Proceeds: "7.00", ProceedsYear2: "8.49"},
		"pp-future":  {CustomerPrice: "12.99", Proceeds: "11.00", ProceedsYear2: "11.99"},
	}

	got, ok := selectCurrentSubscriptionPriceValue(prices, values, now)
	if !ok {
		t.Fatalf("expected selected value")
	}
	if got.CustomerPrice != "9.99" {
		t.Fatalf("expected latest effective current price 9.99, got %q", got.CustomerPrice)
	}
}

func TestSelectCurrentSubscriptionPriceValue_PrefersNonPreservedWhenStartDateMatches(t *testing.T) {
	now := time.Date(2026, time.February, 7, 12, 0, 0, 0, time.UTC)

	prices := []asc.Resource[asc.SubscriptionPriceAttributes]{
		newSubscriptionPriceResource("pp-preserved", "2024-01-01", true),
		newSubscriptionPriceResource("pp-standard", "2024-01-01", false),
	}

	values := map[string]subscriptionPricePointValue{
		"pp-preserved": {CustomerPrice: "4.99"},
		"pp-standard":  {CustomerPrice: "9.99"},
	}

	got, ok := selectCurrentSubscriptionPriceValue(prices, values, now)
	if !ok {
		t.Fatalf("expected selected value")
	}
	if got.CustomerPrice != "9.99" {
		t.Fatalf("expected non-preserved price 9.99, got %q", got.CustomerPrice)
	}
}

func TestSelectCurrentSubscriptionPriceValue_FallsBackToEarliestFutureWhenNoCurrent(t *testing.T) {
	now := time.Date(2026, time.February, 7, 12, 0, 0, 0, time.UTC)

	prices := []asc.Resource[asc.SubscriptionPriceAttributes]{
		newSubscriptionPriceResource("pp-later", "2027-05-01", false),
		newSubscriptionPriceResource("pp-sooner", "2026-03-01", false),
	}

	values := map[string]subscriptionPricePointValue{
		"pp-sooner": {CustomerPrice: "10.99"},
		"pp-later":  {CustomerPrice: "12.99"},
	}

	got, ok := selectCurrentSubscriptionPriceValue(prices, values, now)
	if !ok {
		t.Fatalf("expected selected value")
	}
	if got.CustomerPrice != "10.99" {
		t.Fatalf("expected earliest future price 10.99, got %q", got.CustomerPrice)
	}
}

func TestSelectCurrentSubscriptionPriceValue_PrefersUndatedOverFuture(t *testing.T) {
	now := time.Date(2026, time.February, 7, 12, 0, 0, 0, time.UTC)

	prices := []asc.Resource[asc.SubscriptionPriceAttributes]{
		newSubscriptionPriceResource("pp-undated", "", false),
		newSubscriptionPriceResource("pp-future", "2026-03-01", false),
	}

	values := map[string]subscriptionPricePointValue{
		"pp-undated": {CustomerPrice: "8.99"},
		"pp-future":  {CustomerPrice: "10.99"},
	}

	got, ok := selectCurrentSubscriptionPriceValue(prices, values, now)
	if !ok {
		t.Fatalf("expected selected value")
	}
	if got.CustomerPrice != "8.99" {
		t.Fatalf("expected undated price 8.99, got %q", got.CustomerPrice)
	}
}

func TestSelectCurrentSubscriptionPriceValue_IgnoresPricesMissingIncludedValues(t *testing.T) {
	now := time.Date(2026, time.February, 7, 12, 0, 0, 0, time.UTC)

	prices := []asc.Resource[asc.SubscriptionPriceAttributes]{
		newSubscriptionPriceResource("pp-missing", "2024-01-01", false),
	}

	got, ok := selectCurrentSubscriptionPriceValue(prices, map[string]subscriptionPricePointValue{}, now)
	if ok {
		t.Fatalf("expected no selected value, got %+v", got)
	}
}

func TestSelectCurrentSubscriptionPriceValue_InvalidDateDoesNotBeatValidCurrent(t *testing.T) {
	now := time.Date(2026, time.February, 7, 12, 0, 0, 0, time.UTC)

	prices := []asc.Resource[asc.SubscriptionPriceAttributes]{
		newSubscriptionPriceResource("pp-invalid-date", "not-a-date", false),
		newSubscriptionPriceResource("pp-current", "2024-01-01", false),
	}

	values := map[string]subscriptionPricePointValue{
		"pp-invalid-date": {CustomerPrice: "1.99"},
		"pp-current":      {CustomerPrice: "9.99"},
	}

	got, ok := selectCurrentSubscriptionPriceValue(prices, values, now)
	if !ok {
		t.Fatalf("expected selected value")
	}
	if got.CustomerPrice != "9.99" {
		t.Fatalf("expected valid current date price 9.99, got %q", got.CustomerPrice)
	}
}

func newSubscriptionPriceResource(
	pricePointID string,
	startDate string,
	preserved bool,
) asc.Resource[asc.SubscriptionPriceAttributes] {
	relationships := map[string]any{
		"subscriptionPricePoint": map[string]any{
			"data": map[string]any{
				"type": "subscriptionPricePoints",
				"id":   pricePointID,
			},
		},
	}
	rawRelationships, _ := json.Marshal(relationships)

	return asc.Resource[asc.SubscriptionPriceAttributes]{
		Type:          "subscriptionPrices",
		ID:            "price-" + pricePointID,
		Attributes:    asc.SubscriptionPriceAttributes{StartDate: startDate, Preserved: preserved},
		Relationships: rawRelationships,
	}
}

func TestSelectCurrentSubscriptionPriceValue_UsesUSPacificDay(t *testing.T) {
	// Subscription prices carry only a startDate: each price ends on the date
	// the next one starts, the end-equals-next-start shape App Store Connect
	// uses for app and in-app purchase schedules.
	prices := []asc.Resource[asc.SubscriptionPriceAttributes]{
		newSubscriptionPriceResource("pp-old", "2026-01-01", false),
		newSubscriptionPriceResource("pp-new", "2026-10-01", false),
	}
	values := map[string]subscriptionPricePointValue{
		"pp-old": {CustomerPrice: "0.99"},
		"pp-new": {CustomerPrice: "1.99"},
	}

	tests := []struct {
		name string
		now  time.Time
		want string
	}{
		// 17:30 PDT on 2026-09-30: the 2026-10-01 price has not started.
		{name: "00:30 UTC is the previous Pacific day", now: time.Date(2026, time.October, 1, 0, 30, 0, 0, time.UTC), want: "0.99"},
		// Pacific midnight: the old price ends as the new one starts.
		{name: "Pacific midnight starts the next price", now: time.Date(2026, time.October, 1, 7, 0, 0, 0, time.UTC), want: "1.99"},
		// PST midnight (08:00 UTC) on a winter boundary.
		{name: "PST midnight starts the next price", now: time.Date(2026, time.December, 1, 8, 0, 0, 0, time.UTC), want: "1.99"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := selectCurrentSubscriptionPriceValue(prices, values, test.now)
			if !ok {
				t.Fatal("expected selected value")
			}
			if got.CustomerPrice != test.want {
				t.Fatalf("customerPrice = %q, want %q", got.CustomerPrice, test.want)
			}
		})
	}
}
