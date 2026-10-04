package pricing

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

func TestConsumeResolvedAppPricePage_PrefersManualSameDay(t *testing.T) {
	now := time.Date(2026, time.March, 29, 12, 0, 0, 0, time.UTC)

	page := &asc.AppPricesResponse{
		Data: []asc.Resource[asc.AppPriceAttributes]{
			newResolvedAppPriceResource("automatic-price", "pp-auto", "2025-01-01", "", false),
			newResolvedAppPriceResource("manual-price", "pp-manual", "2025-01-01", "", true),
		},
		Included: mustMarshalResolvedAppJSON(t, []map[string]any{
			appPricePointIncluded("pp-auto", "4.99", "3.49"),
			appPricePointIncluded("pp-manual", "9.99", "8.49"),
			appResolvedTerritoryIncluded("USA", "USD"),
		}),
	}

	candidates := make(map[string]resolvedAppPriceCandidate)
	if err := consumeResolvedAppPricePage(candidates, page, now); err != nil {
		t.Fatalf("consumeResolvedAppPricePage() error = %v", err)
	}

	row := candidates["USA"].row
	if row.CustomerPrice != "9.99" {
		t.Fatalf("expected manual row to win, got %+v", row)
	}
	if row.Manual == nil || !*row.Manual {
		t.Fatalf("expected manual=true, got %+v", row.Manual)
	}
}

func TestConsumeResolvedAppPricePage_SkipsFutureAndExpiredRows(t *testing.T) {
	now := time.Date(2026, time.March, 29, 12, 0, 0, 0, time.UTC)

	page := &asc.AppPricesResponse{
		Data: []asc.Resource[asc.AppPriceAttributes]{
			newResolvedAppPriceResource("expired-price", "pp-expired", "2024-01-01", "2025-01-01", true),
			newResolvedAppPriceResource("future-price", "pp-future", "2030-01-01", "", true),
		},
		Included: mustMarshalResolvedAppJSON(t, []map[string]any{
			appPricePointIncluded("pp-expired", "1.99", "1.40"),
			appPricePointIncluded("pp-future", "12.99", "11.04"),
			appResolvedTerritoryIncluded("USA", "USD"),
		}),
	}

	candidates := make(map[string]resolvedAppPriceCandidate)
	if err := consumeResolvedAppPricePage(candidates, page, now); err != nil {
		t.Fatalf("consumeResolvedAppPricePage() error = %v", err)
	}

	if len(candidates) != 0 {
		t.Fatalf("expected no resolved rows, got %+v", candidates)
	}
}

func TestConsumeResolvedAppPricePage_DateBoundaries(t *testing.T) {
	page := &asc.AppPricesResponse{
		Data: []asc.Resource[asc.AppPriceAttributes]{
			newResolvedAppPriceResource("old-price", "pp-old", "2026-01-01", "2026-10-01", true),
			newResolvedAppPriceResource("new-price", "pp-new", "2026-10-01", "", true),
		},
		Included: mustMarshalResolvedAppJSON(t, []map[string]any{
			appPricePointIncluded("pp-old", "0.99", "0.84"),
			appPricePointIncluded("pp-new", "1.99", "1.69"),
		}),
	}

	tests := []struct {
		now  time.Time
		want string
	}{
		// 17:30 PDT on 2026-09-30: the price ending 2026-10-01 still applies.
		{now: time.Date(2026, time.October, 1, 0, 30, 0, 0, time.UTC), want: "0.99"},
		// Pacific midnight: the old price ends on its end date.
		{now: time.Date(2026, time.October, 1, 7, 0, 0, 0, time.UTC), want: "1.99"},
	}
	for _, test := range tests {
		t.Run(test.now.Format(time.RFC3339), func(t *testing.T) {
			candidates := make(map[string]resolvedAppPriceCandidate)
			if err := consumeResolvedAppPricePage(candidates, page, test.now); err != nil {
				t.Fatalf("consumeResolvedAppPricePage() error = %v", err)
			}
			if got := candidates["USA"].row.CustomerPrice; got != test.want {
				t.Fatalf("customerPrice = %q, want %q", got, test.want)
			}
		})
	}
}

func TestConsumeResolvedAppPricePage_SkipsPriceOnItsEndDate(t *testing.T) {
	page := &asc.AppPricesResponse{
		Data: []asc.Resource[asc.AppPriceAttributes]{
			newResolvedAppPriceResource("ending-price", "pp-ending", "2026-01-01", "2026-09-30", true),
		},
		Included: mustMarshalResolvedAppJSON(t, []map[string]any{
			appPricePointIncluded("pp-ending", "0.99", "0.84"),
		}),
	}

	candidates := make(map[string]resolvedAppPriceCandidate)
	if err := consumeResolvedAppPricePage(candidates, page, time.Date(2026, time.September, 30, 20, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("consumeResolvedAppPricePage() error = %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected the price ending today to be excluded, got %+v", candidates)
	}
}

func newResolvedAppPriceResource(
	priceID string,
	pricePointID string,
	startDate string,
	endDate string,
	manual bool,
) asc.Resource[asc.AppPriceAttributes] {
	relationships := map[string]any{
		"territory": map[string]any{
			"data": map[string]any{
				"type": "territories",
				"id":   "USA",
			},
		},
		"appPricePoint": map[string]any{
			"data": map[string]any{
				"type": "appPricePoints",
				"id":   pricePointID,
			},
		},
	}

	return asc.Resource[asc.AppPriceAttributes]{
		Type:          asc.ResourceTypeAppPrices,
		ID:            priceID,
		Attributes:    asc.AppPriceAttributes{StartDate: startDate, EndDate: endDate, Manual: manual},
		Relationships: mustMarshalResolvedAppValue(relationships),
	}
}

func appPricePointIncluded(id, customerPrice, proceeds string) map[string]any {
	return map[string]any{
		"type": "appPricePoints",
		"id":   id,
		"attributes": map[string]any{
			"customerPrice": customerPrice,
			"proceeds":      proceeds,
		},
	}
}

func appResolvedTerritoryIncluded(id, currency string) map[string]any {
	return map[string]any{
		"type": "territories",
		"id":   id,
		"attributes": map[string]any{
			"currency": currency,
		},
	}
}

func mustMarshalResolvedAppJSON(t *testing.T, value any) []byte {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return data
}

func mustMarshalResolvedAppValue(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}
