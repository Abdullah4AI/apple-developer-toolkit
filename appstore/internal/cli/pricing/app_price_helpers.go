package pricing

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

const appPriceDateLayout = "2006-01-02"

type appPriceEntry struct {
	TerritoryID  string
	PricePointID string
	StartDate    string
	EndDate      string
	Manual       bool
	StartAt      *time.Time
	EndAt        *time.Time
}

func newAppPriceEntry(territoryID, pricePointID, startDate, endDate string, manual bool) appPriceEntry {
	return appPriceEntry{
		TerritoryID:  strings.ToUpper(strings.TrimSpace(territoryID)),
		PricePointID: strings.TrimSpace(pricePointID),
		StartDate:    strings.TrimSpace(startDate),
		EndDate:      strings.TrimSpace(endDate),
		Manual:       manual,
		StartAt:      parseAppPriceDate(startDate),
		EndAt:        parseAppPriceDate(endDate),
	}
}

func parseAppPriceDate(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(appPriceDateLayout, value)
	if err != nil {
		return nil
	}
	normalized := dateOnlyUTC(parsed.UTC())
	return &normalized
}

func dateOnlyUTC(value time.Time) time.Time {
	return time.Date(value.UTC().Year(), value.UTC().Month(), value.UTC().Day(), 0, 0, 0, 0, time.UTC)
}

// appPriceEntryActiveOn reports whether entry applies on date, a pricing date
// from shared.PricingDate.
func appPriceEntryActiveOn(entry appPriceEntry, date time.Time) bool {
	return shared.PriceActiveOn(entry.StartAt, entry.EndAt, date)
}

func appPriceEntryIsNewer(candidate, existing appPriceEntry) bool {
	switch {
	case candidate.StartAt == nil && existing.StartAt != nil:
		return false
	case candidate.StartAt != nil && existing.StartAt == nil:
		return true
	case candidate.StartAt != nil && existing.StartAt != nil:
		if !candidate.StartAt.Equal(*existing.StartAt) {
			return candidate.StartAt.After(*existing.StartAt)
		}
	}
	if candidate.Manual != existing.Manual {
		return candidate.Manual && !existing.Manual
	}
	return candidate.PricePointID > existing.PricePointID
}

func appPriceRelationshipID(relationships json.RawMessage, key string) string {
	if len(relationships) == 0 {
		return ""
	}

	var rels map[string]json.RawMessage
	if err := json.Unmarshal(relationships, &rels); err != nil {
		return ""
	}
	rawRelationship, ok := rels[key]
	if !ok {
		return ""
	}

	var relationship struct {
		Data asc.ResourceData `json:"data"`
	}
	if err := json.Unmarshal(rawRelationship, &relationship); err != nil {
		return ""
	}

	return strings.TrimSpace(relationship.Data.ID)
}
