package asc

import "fmt"

// PriceSchedulePPPResult is the stable computed output contract for
// pricing ppp (app) and iap pricing ppp. Rows use the subscription PPP row
// shape: sourcePrice is Apple's equalized price for the base price, and
// requestedMultiple is the territory's price level relative to the base
// territory. Skipped rows keep their current price in the new schedule.
type PriceSchedulePPPResult struct {
	AppID            string                                `json:"appId,omitempty"`
	IAPID            string                                `json:"iapId,omitempty"`
	BaseTerritory    string                                `json:"baseTerritory"`
	BasePrice        string                                `json:"basePrice"`
	BasePricePointID string                                `json:"basePricePointId"`
	Index            SubscriptionPricingPPPIndex           `json:"index"`
	Rounding         string                                `json:"rounding"`
	StartDate        string                                `json:"startDate,omitempty"`
	DryRun           bool                                  `json:"dryRun"`
	Summary          SubscriptionPricingDeriveSummary      `json:"summary"`
	Rows             []SubscriptionPricingDeriveRow        `json:"rows"`
	Verification     SubscriptionPricingDeriveVerification `json:"verification"`
}

func priceSchedulePPPSummaryRows(result *PriceSchedulePPPResult) ([]string, [][]string) {
	resource := result.AppID
	if result.IAPID != "" {
		resource = result.IAPID
	}
	return []string{
		"Resource", "Base Territory", "Base Price", "Index As Of", "Rounding",
		"Dry Run", "Start Date", "Total", "Planned", "Noop", "Skipped", "Applied",
		"Verified", "Unresolved", "Failed",
	}, [][]string{{
		resource,
		result.BaseTerritory,
		result.BasePrice,
		result.Index.AsOf,
		result.Rounding,
		fmt.Sprintf("%t", result.DryRun),
		result.StartDate,
		fmt.Sprintf("%d", result.Summary.Total),
		fmt.Sprintf("%d", result.Summary.Planned),
		fmt.Sprintf("%d", result.Summary.Noop),
		fmt.Sprintf("%d", result.Summary.Skipped),
		fmt.Sprintf("%d", result.Summary.Applied),
		fmt.Sprintf("%d", result.Summary.Verified),
		fmt.Sprintf("%d", result.Summary.Unresolved),
		fmt.Sprintf("%d", result.Summary.Failed),
	}}
}
