package asc

import (
	"fmt"
	"strings"
)

// LocalizationImportLocaleResult reports the plan and outcome for one locale
// of a version-scoped localization import. Action is create, update, or skip;
// Status is planned, succeeded, skipped, or failed.
type LocalizationImportLocaleResult struct {
	Locale         string   `json:"locale"`
	Action         string   `json:"action"`
	Status         string   `json:"status"`
	Fields         []string `json:"fields,omitempty"`
	LocalizationID string   `json:"localizationId,omitempty"`
	Error          string   `json:"error,omitempty"`
}

// LocalizationImportResult is the receipt for the version-scoped
// `localizations import` commands. Total always equals the locales in the
// file: Planned + Skipped for a dry run, Succeeded + Failed + Skipped otherwise.
type LocalizationImportResult struct {
	Type      string                           `json:"type"`
	VersionID string                           `json:"versionId"`
	File      string                           `json:"file"`
	DryRun    bool                             `json:"dryRun"`
	Total     int                              `json:"total"`
	Planned   int                              `json:"planned"`
	Succeeded int                              `json:"succeeded"`
	Skipped   int                              `json:"skipped"`
	Failed    int                              `json:"failed"`
	Results   []LocalizationImportLocaleResult `json:"results"`
}

func localizationImportTables(result *LocalizationImportResult, render func([]string, [][]string)) error {
	render(
		[]string{"Type", "Version ID", "File", "Dry Run", "Total", "Planned", "Succeeded", "Skipped", "Failed"},
		[][]string{{
			SanitizeTerminalText(result.Type),
			SanitizeTerminalText(result.VersionID),
			SanitizeTerminalText(result.File),
			fmt.Sprintf("%t", result.DryRun),
			fmt.Sprintf("%d", result.Total),
			fmt.Sprintf("%d", result.Planned),
			fmt.Sprintf("%d", result.Succeeded),
			fmt.Sprintf("%d", result.Skipped),
			fmt.Sprintf("%d", result.Failed),
		}},
	)
	if len(result.Results) == 0 {
		return nil
	}
	rows := make([][]string, 0, len(result.Results))
	for _, item := range result.Results {
		rows = append(rows, []string{
			SanitizeTerminalText(item.Locale),
			SanitizeTerminalText(item.Action),
			SanitizeTerminalText(item.Status),
			SanitizeTerminalText(strings.Join(item.Fields, ",")),
			SanitizeTerminalText(item.LocalizationID),
			compactWhitespace(item.Error),
		})
	}
	render([]string{"Locale", "Action", "Status", "Fields", "Localization ID", "Error"}, rows)
	return nil
}
