package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

type resolvedAppPriceCandidate struct {
	entry appPriceEntry
	row   shared.ResolvedPriceRow
}

type appSchedulePriceFetcher func(context.Context, string, ...asc.AppPriceSchedulePricesOption) (*asc.AppPricesResponse, error)

func fetchResolvedAppSchedulePrices(
	ctx context.Context,
	client *asc.Client,
	scheduleID string,
	surface string,
	limit int,
	nextURL string,
	now time.Time,
) (*shared.ResolvedPricesResult, error) {
	if limit <= 0 {
		limit = 200
	}

	candidates := make(map[string]resolvedAppPriceCandidate)
	surface = strings.TrimSpace(surface)
	includeSibling := strings.TrimSpace(nextURL) == ""

	switch surface {
	case "manual":
		if err := consumeResolvedAppSchedulePrices(ctx, scheduleID, nextURL, limit, now, candidates, client.GetAppPriceScheduleManualPrices); err != nil {
			return nil, fmt.Errorf("fetch manual prices: %w", err)
		}
		if includeSibling {
			if err := consumeResolvedAppSchedulePrices(ctx, scheduleID, "", limit, now, candidates, client.GetAppPriceScheduleAutomaticPrices); err != nil {
				return nil, fmt.Errorf("fetch automatic prices: %w", err)
			}
		}
	case "automatic":
		if err := consumeResolvedAppSchedulePrices(ctx, scheduleID, nextURL, limit, now, candidates, client.GetAppPriceScheduleAutomaticPrices); err != nil {
			return nil, fmt.Errorf("fetch automatic prices: %w", err)
		}
		if includeSibling {
			if err := consumeResolvedAppSchedulePrices(ctx, scheduleID, "", limit, now, candidates, client.GetAppPriceScheduleManualPrices); err != nil {
				return nil, fmt.Errorf("fetch manual prices: %w", err)
			}
		}
	default:
		return nil, fmt.Errorf("unknown app price surface %q", surface)
	}

	rows := make([]shared.ResolvedPriceRow, 0, len(candidates))
	for _, candidate := range candidates {
		rows = append(rows, candidate.row)
	}
	shared.SortResolvedPrices(rows)
	return &shared.ResolvedPricesResult{Prices: rows}, nil
}

func consumeResolvedAppSchedulePrices(
	ctx context.Context,
	scheduleID string,
	nextURL string,
	limit int,
	now time.Time,
	candidates map[string]resolvedAppPriceCandidate,
	fetch appSchedulePriceFetcher,
) error {
	opts := []asc.AppPriceSchedulePricesOption{
		asc.WithAppPriceSchedulePricesLimit(limit),
		asc.WithAppPriceSchedulePricesNextURL(nextURL),
		asc.WithAppPriceSchedulePricesInclude([]string{"appPricePoint", "territory"}),
		asc.WithAppPriceSchedulePricesFields([]string{"manual", "startDate", "endDate", "appPricePoint", "territory"}),
		asc.WithAppPriceSchedulePricesPricePointFields([]string{"customerPrice", "proceeds", "territory"}),
		asc.WithAppPriceSchedulePricesTerritoryFields([]string{"currency"}),
	}

	firstPage, err := fetch(ctx, scheduleID, opts...)
	if err != nil {
		return err
	}

	return asc.PaginateEach(ctx, firstPage, func(ctx context.Context, next string) (asc.PaginatedResponse, error) {
		nextURL, err := shared.MergeNextURLQuery(next, appSchedulePricesQuery(limit))
		if err != nil {
			return nil, err
		}
		return fetch(ctx, scheduleID, asc.WithAppPriceSchedulePricesNextURL(nextURL))
	}, func(page asc.PaginatedResponse) error {
		resp, ok := page.(*asc.AppPricesResponse)
		if !ok {
			return fmt.Errorf("unexpected app prices response type %T", page)
		}
		return consumeResolvedAppPricePage(candidates, resp, now)
	})
}

func appSchedulePricesQuery(limit int) url.Values {
	values := url.Values{}
	values.Set("include", "appPricePoint,territory")
	values.Set("fields[appPrices]", "manual,startDate,endDate,appPricePoint,territory")
	values.Set("fields[appPricePoints]", "customerPrice,proceeds,territory")
	values.Set("fields[territories]", "currency")
	if limit > 0 {
		values.Set("limit", fmt.Sprintf("%d", limit))
	}
	return values
}

func consumeResolvedAppPricePage(
	candidates map[string]resolvedAppPriceCandidate,
	page *asc.AppPricesResponse,
	now time.Time,
) error {
	if page == nil {
		return nil
	}

	values, currencies, err := parseResolvedAppPricePageIncluded(page.Included)
	if err != nil {
		return err
	}

	asOf := dateOnlyUTC(now)
	for _, item := range page.Data {
		entry, row, ok := resolvedAppPriceCandidateFromResource(item, values, currencies)
		if !ok || !appPriceEntryActiveOn(entry, asOf) {
			continue
		}

		existing, exists := candidates[entry.TerritoryID]
		if !exists || appPriceEntryIsNewer(entry, existing.entry) {
			candidates[entry.TerritoryID] = resolvedAppPriceCandidate{entry: entry, row: row}
		}
	}

	return nil
}

func parseResolvedAppPricePageIncluded(raw json.RawMessage) (map[string]asc.AppPricePointV3Attributes, map[string]string, error) {
	values := make(map[string]asc.AppPricePointV3Attributes)
	currencies := make(map[string]string)
	if len(raw) == 0 {
		return values, currencies, nil
	}

	var included []struct {
		Type       string          `json:"type"`
		ID         string          `json:"id"`
		Attributes json.RawMessage `json:"attributes"`
	}
	if err := json.Unmarshal(raw, &included); err != nil {
		return nil, nil, fmt.Errorf("parse app price included resources: %w", err)
	}

	for _, item := range included {
		switch item.Type {
		case string(asc.ResourceTypeAppPricePoints):
			var attrs asc.AppPricePointV3Attributes
			if err := json.Unmarshal(item.Attributes, &attrs); err != nil {
				continue
			}
			values[strings.TrimSpace(item.ID)] = attrs
		case string(asc.ResourceTypeTerritories):
			var attrs struct {
				Currency string `json:"currency"`
			}
			if err := json.Unmarshal(item.Attributes, &attrs); err != nil {
				continue
			}
			if currency := strings.TrimSpace(attrs.Currency); currency != "" {
				currencies[strings.ToUpper(strings.TrimSpace(item.ID))] = currency
			}
		}
	}

	return values, currencies, nil
}

func resolvedAppPriceCandidateFromResource(
	item asc.Resource[asc.AppPriceAttributes],
	values map[string]asc.AppPricePointV3Attributes,
	currencies map[string]string,
) (appPriceEntry, shared.ResolvedPriceRow, bool) {
	territoryID := strings.ToUpper(strings.TrimSpace(appPriceRelationshipID(item.Relationships, "territory")))
	pricePointID := strings.TrimSpace(appPriceRelationshipID(item.Relationships, "appPricePoint"))
	if territoryID == "" || pricePointID == "" {
		return appPriceEntry{}, shared.ResolvedPriceRow{}, false
	}

	value, ok := values[pricePointID]
	if !ok {
		return appPriceEntry{}, shared.ResolvedPriceRow{}, false
	}

	entry := newAppPriceEntry(
		territoryID,
		pricePointID,
		strings.TrimSpace(item.Attributes.StartDate),
		strings.TrimSpace(item.Attributes.EndDate),
		item.Attributes.Manual,
	)
	currency := currencies[territoryID]
	if currency == "" {
		currency = territoryID
	}

	return entry, shared.ResolvedPriceRow{
		Territory:     territoryID,
		PriceID:       strings.TrimSpace(item.ID),
		PricePointID:  pricePointID,
		CustomerPrice: strings.TrimSpace(value.CustomerPrice),
		Currency:      currency,
		Proceeds:      strings.TrimSpace(value.Proceeds),
		StartDate:     entry.StartDate,
		EndDate:       entry.EndDate,
		Manual:        resolvedAppBoolPtr(item.Attributes.Manual),
	}, true
}

func resolvedAppBoolPtr(value bool) *bool {
	v := value
	return &v
}
