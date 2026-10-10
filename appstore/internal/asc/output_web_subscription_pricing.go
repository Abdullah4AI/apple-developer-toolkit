package asc

import "fmt"

// Web monthly-commitment bootstrap stages name the last applied mutation so a
// rerun can resume without repeating completed work.
const (
	WebMonthlyCommitmentStagePlanAvailability = "plan_availability"
	WebMonthlyCommitmentStagePrices           = "prices"
	WebMonthlyCommitmentStageVerified         = "verified"
)

// WebSubscriptionMonthlyCommitmentBootstrapResult is the mutation receipt for
// `asc web subscriptions pricing monthly-commitment bootstrap`.
type WebSubscriptionMonthlyCommitmentBootstrapResult struct {
	SubscriptionID              string `json:"subscriptionId"`
	Territory                   string `json:"territory"`
	PlanAvailabilityID          string `json:"planAvailabilityId"`
	PlanAvailabilityNew         bool   `json:"planAvailabilityCreated"`
	PlanAvailabilityWouldCreate bool   `json:"planAvailabilityWouldCreate,omitempty"`
	UpfrontPricePointID         string `json:"upfrontPricePointId"`
	MonthlyPricePointID         string `json:"monthlyPricePointId"`
	PricesCreated               bool   `json:"pricesCreated"`
	Verified                    bool   `json:"verified"`
	CompletedStage              string `json:"completedStage,omitempty"`
	Failure                     string `json:"failure,omitempty"`
	DryRun                      bool   `json:"dryRun"`
	StartDate                   string `json:"startDate,omitempty"`
	PreserveCurrentPrice        bool   `json:"preserveCurrentPrice,omitempty"`
}

func webSubscriptionMonthlyCommitmentBootstrapRows(result *WebSubscriptionMonthlyCommitmentBootstrapResult) ([]string, [][]string) {
	if result.DryRun {
		return []string{
			"Dry Run",
			"Subscription ID",
			"Territory",
			"Plan Availability ID",
			"Would Create Availability",
			"Upfront Price Point ID",
			"Monthly Price Point ID",
			"Start Date",
			"Preserve Current Price",
		}, [][]string{{
			fmt.Sprintf("%t", result.DryRun),
			result.SubscriptionID,
			result.Territory,
			result.PlanAvailabilityID,
			fmt.Sprintf("%t", result.PlanAvailabilityWouldCreate),
			result.UpfrontPricePointID,
			result.MonthlyPricePointID,
			result.StartDate,
			fmt.Sprintf("%t", result.PreserveCurrentPrice),
		}}
	}
	return []string{
		"Subscription ID",
		"Territory",
		"Plan Availability ID",
		"Availability Created",
		"Prices Created",
		"Verified",
		"Completed Stage",
		"Failure",
	}, [][]string{{
		result.SubscriptionID,
		result.Territory,
		result.PlanAvailabilityID,
		fmt.Sprintf("%t", result.PlanAvailabilityNew),
		fmt.Sprintf("%t", result.PricesCreated),
		fmt.Sprintf("%t", result.Verified),
		result.CompletedStage,
		result.Failure,
	}}
}
