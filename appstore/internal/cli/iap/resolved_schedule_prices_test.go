package iap

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

func TestConsumeResolvedIAPPricePage_PrefersManualSameDay(t *testing.T) {
	now := time.Date(2026, time.March, 29, 12, 0, 0, 0, time.UTC)

	page := &asc.InAppPurchasePricesResponse{
		Data: []asc.Resource[asc.InAppPurchasePriceAttributes]{
			newResolvedIAPPriceResource("auto-price", "pp-auto", "2025-01-01", "", false),
			newResolvedIAPPriceResource("manual-price", "pp-manual", "2025-01-01", "", true),
		},
		Included: mustMarshalResolvedIAPJSON(t, []map[string]any{
			inAppPurchasePricePointIncluded("pp-auto", "4.99", "3.49"),
			inAppPurchasePricePointIncluded("pp-manual", "9.99", "8.49"),
			resolvedTerritoryIncluded("USA", "USD"),
		}),
	}

	candidates := make(map[string]resolvedIAPPriceCandidate)
	if err := consumeResolvedIAPPricePage(candidates, page, now); err != nil {
		t.Fatalf("consumeResolvedIAPPricePage() error = %v", err)
	}

	row := candidates["USA"].row
	if row.CustomerPrice != "9.99" {
		t.Fatalf("expected manual row to win, got %+v", row)
	}
	if row.Manual == nil || !*row.Manual {
		t.Fatalf("expected manual=true, got %+v", row.Manual)
	}
}

func TestConsumeResolvedIAPPricePage_SkipsFutureAndExpiredRows(t *testing.T) {
	now := time.Date(2026, time.March, 29, 12, 0, 0, 0, time.UTC)

	page := &asc.InAppPurchasePricesResponse{
		Data: []asc.Resource[asc.InAppPurchasePriceAttributes]{
			newResolvedIAPPriceResource("expired-price", "pp-expired", "2024-01-01", "2025-01-01", true),
			newResolvedIAPPriceResource("future-price", "pp-future", "2030-01-01", "", true),
		},
		Included: mustMarshalResolvedIAPJSON(t, []map[string]any{
			inAppPurchasePricePointIncluded("pp-expired", "1.99", "1.40"),
			inAppPurchasePricePointIncluded("pp-future", "12.99", "11.04"),
			resolvedTerritoryIncluded("USA", "USD"),
		}),
	}

	candidates := make(map[string]resolvedIAPPriceCandidate)
	if err := consumeResolvedIAPPricePage(candidates, page, now); err != nil {
		t.Fatalf("consumeResolvedIAPPricePage() error = %v", err)
	}

	if len(candidates) != 0 {
		t.Fatalf("expected no resolved rows, got %+v", candidates)
	}
}

func newResolvedIAPPriceResource(
	priceID string,
	pricePointID string,
	startDate string,
	endDate string,
	manual bool,
) asc.Resource[asc.InAppPurchasePriceAttributes] {
	relationships := map[string]any{
		"territory": map[string]any{
			"data": map[string]any{
				"type": "territories",
				"id":   "USA",
			},
		},
		"inAppPurchasePricePoint": map[string]any{
			"data": map[string]any{
				"type": "inAppPurchasePricePoints",
				"id":   pricePointID,
			},
		},
	}

	return asc.Resource[asc.InAppPurchasePriceAttributes]{
		Type:          asc.ResourceTypeInAppPurchasePrices,
		ID:            priceID,
		Attributes:    asc.InAppPurchasePriceAttributes{StartDate: startDate, EndDate: endDate, Manual: manual},
		Relationships: mustMarshalResolvedIAPValue(relationships),
	}
}

func inAppPurchasePricePointIncluded(id, customerPrice, proceeds string) map[string]any {
	return map[string]any{
		"type": "inAppPurchasePricePoints",
		"id":   id,
		"attributes": map[string]any{
			"customerPrice": customerPrice,
			"proceeds":      proceeds,
		},
	}
}

func resolvedTerritoryIncluded(id, currency string) map[string]any {
	return map[string]any{
		"type": "territories",
		"id":   id,
		"attributes": map[string]any{
			"currency": currency,
		},
	}
}

func mustMarshalResolvedIAPJSON(t *testing.T, value any) []byte {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return data
}

func mustMarshalResolvedIAPValue(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}
