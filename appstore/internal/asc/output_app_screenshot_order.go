package asc

import (
	"strconv"
	"strings"
)

// AppScreenshotReorderResult records a screenshot set whose order was replaced.
type AppScreenshotReorderResult struct {
	SetID         string   `json:"setId"`
	Reordered     bool     `json:"reordered"`
	ScreenshotIDs []string `json:"screenshotIds"`
}

func appScreenshotReorderResultRows(r *AppScreenshotReorderResult) ([]string, [][]string) {
	return []string{"Set ID", "Reordered", "Screenshot IDs"}, [][]string{{SanitizeTerminalText(r.SetID), strconv.FormatBool(r.Reordered), SanitizeTerminalText(strings.Join(r.ScreenshotIDs, ","))}}
}
