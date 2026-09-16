package asc

import "fmt"

// AppPriceScheduleNotConfiguredResult represents CLI output when an app has no
// price schedule yet, so `asc pricing current` has nothing to compute.
type AppPriceScheduleNotConfiguredResult struct {
	AppID      string `json:"appId"`
	Configured bool   `json:"configured"`
}

func appPriceScheduleNotConfiguredRows(result *AppPriceScheduleNotConfiguredResult) ([]string, [][]string) {
	headers := []string{"App ID", "Configured"}
	rows := [][]string{{SanitizeTerminalText(result.AppID), fmt.Sprintf("%t", result.Configured)}}
	return headers, rows
}
