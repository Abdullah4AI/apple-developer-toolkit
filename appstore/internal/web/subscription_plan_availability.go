package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// SubscriptionPlanAvailability models the internal web API subscription plan availability resource.
type SubscriptionPlanAvailability struct {
	ID                         string   `json:"id"`
	Type                       string   `json:"type,omitempty"`
	AvailableInNewTerritories  bool     `json:"availableInNewTerritories"`
	PlanType                   string   `json:"planType,omitempty"`
	AvailableTerritories       []string `json:"availableTerritories,omitempty"`
	AvailableTerritoriesLoaded bool     `json:"-"`
}

func decodeSubscriptionPlanAvailabilityResource(resource jsonAPIResource) SubscriptionPlanAvailability {
	availability := SubscriptionPlanAvailability{
		ID:                        strings.TrimSpace(resource.ID),
		Type:                      strings.TrimSpace(resource.Type),
		AvailableInNewTerritories: boolAttr(resource.Attributes, "availableInNewTerritories"),
		PlanType:                  stringAttr(resource.Attributes, "planType"),
	}

	relationship, ok := resource.Relationships["availableTerritories"]
	if ok {
		trimmedData := strings.TrimSpace(string(relationship.Data))
		availability.AvailableTerritoriesLoaded = trimmedData != "" && trimmedData != "null"
	}

	refs := parseRelationshipRefs(relationship.Data)
	if len(refs) == 0 {
		return availability
	}

	territories := make([]string, 0, len(refs))
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		territoryID := strings.ToUpper(strings.TrimSpace(ref.ID))
		if territoryID == "" {
			continue
		}
		if _, ok := seen[territoryID]; ok {
			continue
		}
		seen[territoryID] = struct{}{}
		territories = append(territories, territoryID)
	}
	availability.AvailableTerritories = territories
	return availability
}

// ListSubscriptionPlanAvailabilities retrieves sale availability plans for a subscription.
func (c *Client) ListSubscriptionPlanAvailabilities(ctx context.Context, subscriptionID string) ([]SubscriptionPlanAvailability, error) {
	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscription id is required")
	}

	query := url.Values{}
	query.Set("include", "availableTerritories")
	query.Set("limit[availableTerritories]", "200")
	path := queryPath("/subscriptions/"+url.PathEscape(subscriptionID)+"/planAvailabilities", query)

	responseBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var payload jsonAPIListPayload
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse subscription plan availabilities response: %w", err)
	}

	availabilities := make([]SubscriptionPlanAvailability, 0, len(payload.Data))
	for _, resource := range payload.Data {
		availabilities = append(availabilities, decodeSubscriptionPlanAvailabilityResource(resource))
	}
	return availabilities, nil
}

// RemoveSubscriptionPlanAvailabilityFromSale clears all available territories for a subscription plan availability.
func (c *Client) RemoveSubscriptionPlanAvailabilityFromSale(ctx context.Context, planAvailabilityID string) (*SubscriptionPlanAvailability, error) {
	planAvailabilityID = strings.TrimSpace(planAvailabilityID)
	if planAvailabilityID == "" {
		return nil, fmt.Errorf("subscription plan availability id is required")
	}

	requestBody := map[string]any{
		"data": map[string]any{
			"type": "subscriptionPlanAvailabilities",
			"id":   planAvailabilityID,
			"attributes": map[string]bool{
				"availableInNewTerritories": false,
			},
			"relationships": map[string]any{
				"availableTerritories": map[string]any{
					"data": []any{},
				},
			},
		},
	}

	path := "/subscriptionPlanAvailabilities/" + url.PathEscape(planAvailabilityID)
	responseBody, err := c.doRequest(ctx, http.MethodPatch, path, requestBody)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Data jsonAPIResource `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse subscription plan availability response: %w", err)
	}

	availability := decodeSubscriptionPlanAvailabilityResource(payload.Data)
	return &availability, nil
}
