package pricing

import (
	"context"
	"fmt"
	"strings"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

// Initial availability requires an entry for every territory, including those
// where the app must remain unavailable.
func initialTerritoryAvailabilities(ctx context.Context, client *asc.Client, selected []string, available bool) ([]asc.TerritoryAvailabilityCreate, error) {
	first, err := client.GetTerritories(ctx, asc.WithTerritoriesLimit(200))
	if err != nil {
		return nil, fmt.Errorf("fetch territories: %w", err)
	}
	pages, err := asc.PaginateAll(ctx, first, func(ctx context.Context, next string) (asc.PaginatedResponse, error) {
		return client.GetTerritories(ctx, asc.WithTerritoriesNextURL(next))
	})
	if err != nil {
		return nil, fmt.Errorf("fetch territories: %w", err)
	}
	catalog := pages.(*asc.TerritoriesResponse)
	if len(catalog.Data) == 0 {
		return nil, fmt.Errorf("territory catalog is empty; availability was not created")
	}
	known := make(map[string]bool, len(catalog.Data))
	for _, territory := range catalog.Data {
		id := strings.ToUpper(strings.TrimSpace(territory.ID))
		if id == "" {
			return nil, fmt.Errorf("territory catalog contains an empty ID")
		}
		known[id] = true
	}
	result := make([]asc.TerritoryAvailabilityCreate, 0, len(known))
	for _, id := range selected {
		if !known[id] {
			return nil, fmt.Errorf("territory %q is missing from Apple's territory catalog", id)
		}
	}
	for _, id := range selected {
		if !known[id] {
			continue
		}
		result = append(result, asc.TerritoryAvailabilityCreate{TerritoryID: id, Available: available})
		delete(known, id)
	}
	for _, territory := range catalog.Data {
		id := strings.ToUpper(strings.TrimSpace(territory.ID))
		if !known[id] {
			continue
		}
		result = append(result, asc.TerritoryAvailabilityCreate{TerritoryID: id, Available: false})
		delete(known, id)
	}
	return result, nil
}
