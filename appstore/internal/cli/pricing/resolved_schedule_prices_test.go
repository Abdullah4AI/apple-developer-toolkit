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
