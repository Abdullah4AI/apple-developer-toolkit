package asc

import "fmt"

// AppPriceScheduleNotConfiguredResult represents CLI output when `asc pricing
// current` has no current base territory price to report. Without a Reason,
// the app has no price schedule yet. Reason "no_current_base_price" means the
// schedule exists but has no price for BaseTerritory today; NextStartDate is
// the start date of a later base territory price when one is scheduled.
type AppPriceScheduleNotConfiguredResult struct {
	AppID         string `json:"appId"`
	Configured    bool   `json:"configured"`
	BaseTerritory string `json:"baseTerritory,omitempty"`
	Reason        string `json:"reason,omitempty"`
	NextStartDate string `json:"nextStartDate,omitempty"`
}

func appPriceScheduleNotConfiguredRows(result *AppPriceScheduleNotConfiguredResult) ([]string, [][]string) {
	headers := []string{"App ID", "Configured"}
	row := []string{SanitizeTerminalText(result.AppID), fmt.Sprintf("%t", result.Configured)}
	if result.Reason != "" {
		headers = append(headers, "Base Territory", "Reason", "Next Start Date")
		row = append(
			row,
			SanitizeTerminalText(result.BaseTerritory),
			SanitizeTerminalText(result.Reason),
			SanitizeTerminalText(result.NextStartDate),
		)
	}
	return headers, [][]string{row}
}
