package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	appSandboxEntitlementResourceType      = "appSandboxEntitlements"
	appSandboxEntitlementUsageResourceType = "appSandboxEntitlementUsages"
	appStoreReviewDetailResourceType       = "appStoreReviewDetails"
	appStoreVersionResourceType            = "appStoreVersions"

	// Apple's catalog holds fewer than 100 entitlement keys, and a review
	// detail has at most one usage row per key, so one page of this size is a
	// complete collection. Reads fail closed when Apple reports more.
	appSandboxEntitlementPageSize = "200"
)

// AppStoreVersionReviewDetail identifies the App Review details of one App
// Store version. ReviewDetailID is empty when the version has none yet.
type AppStoreVersionReviewDetail struct {
	VersionID      string
	Platform       string
	ReviewDetailID string
}

// AppSandboxEntitlement is one entitlement key accepted in the App Sandbox
// Information section. Optional reports Apple's optional attribute and is nil
// when Apple omitted it.
type AppSandboxEntitlement struct {
	ID       string
	Optional *bool
}

// AppSandboxEntitlementsResult is Apple's entitlement key catalog. JSON output
// emits Raw verbatim; the typed fields feed validation and human output.
type AppSandboxEntitlementsResult struct {
	Data []AppSandboxEntitlement
	Raw  json.RawMessage `json:"-"`
}

// MarshalJSON preserves Apple's raw catalog envelope.
func (r AppSandboxEntitlementsResult) MarshalJSON() ([]byte, error) {
	return marshalAppSandboxRaw(r.Raw, "app sandbox entitlements")
}

// AppSandboxEntitlementUsage is one App Sandbox Information row: an
// entitlement key and the usage information App Review reads.
type AppSandboxEntitlementUsage struct {
	ID            string
	EntitlementID string
	Justification string
}

// AppSandboxEntitlementUsagesResult is the complete usage collection of one
// review detail. JSON output emits Raw verbatim; the typed fields feed
// mutation preflight and human output.
type AppSandboxEntitlementUsagesResult struct {
	Data []AppSandboxEntitlementUsage
	Raw  json.RawMessage `json:"-"`
}

// MarshalJSON preserves Apple's raw usage collection envelope.
func (r AppSandboxEntitlementUsagesResult) MarshalJSON() ([]byte, error) {
	return marshalAppSandboxRaw(r.Raw, "app sandbox entitlement usages")
}

// GetAppStoreVersionReviewDetail reads a version's platform and the ID of its
// App Review details in one request.
func (c *Client) GetAppStoreVersionReviewDetail(ctx context.Context, versionID string) (*AppStoreVersionReviewDetail, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("version id is required")
	}
	query := url.Values{}
	query.Set("fields["+appStoreVersionResourceType+"]", "platform,appStoreReviewDetail")
	query.Set("include", "appStoreReviewDetail")
	path := queryPath("/"+appStoreVersionResourceType+"/"+url.PathEscape(versionID), query)
	body, err := c.doAppSandboxRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data *jsonAPIResource `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse app store version response: %w", err)
	}
	if payload.Data == nil || payload.Data.Type != appStoreVersionResourceType || strings.TrimSpace(payload.Data.ID) != versionID {
		return nil, fmt.Errorf("app store version response does not identify version %q", versionID)
	}
	detail := &AppStoreVersionReviewDetail{
		VersionID: versionID,
		Platform:  stringAttr(payload.Data.Attributes, "platform"),
	}
	if ref := firstRelationshipRef(*payload.Data, "appStoreReviewDetail"); ref != nil {
		if ref.Type != appStoreReviewDetailResourceType {
			return nil, fmt.Errorf("app store version %q review detail relationship has type %q", versionID, ref.Type)
		}
		detail.ReviewDetailID = strings.TrimSpace(ref.ID)
	}
	return detail, nil
}

// ListAppSandboxEntitlements reads the catalog of entitlement keys App Store
// Connect accepts in the App Sandbox Information section.
func (c *Client) ListAppSandboxEntitlements(ctx context.Context) (*AppSandboxEntitlementsResult, error) {
	query := url.Values{}
	query.Set("limit", appSandboxEntitlementPageSize)
	resources, raw, err := c.readCompleteAppSandboxCollection(ctx, queryPath("/"+appSandboxEntitlementResourceType, query), "app sandbox entitlements")
	if err != nil {
		return nil, err
	}
	result := &AppSandboxEntitlementsResult{Data: make([]AppSandboxEntitlement, 0, len(resources)), Raw: raw}
	for _, resource := range resources {
		if resource.Type != appSandboxEntitlementResourceType || strings.TrimSpace(resource.ID) == "" {
			return nil, fmt.Errorf("app sandbox entitlements response contains resource type %q and id %q", resource.Type, resource.ID)
		}
		entitlement := AppSandboxEntitlement{ID: strings.TrimSpace(resource.ID)}
		if optional, known := boolAttrKnown(resource.Attributes, "optional"); known {
			entitlement.Optional = &optional
		}
		result.Data = append(result.Data, entitlement)
	}
	return result, nil
}

// ListAppSandboxEntitlementUsages reads every App Sandbox Information row of
// one review detail, with the entitlement key of each row.
func (c *Client) ListAppSandboxEntitlementUsages(ctx context.Context, reviewDetailID string) (*AppSandboxEntitlementUsagesResult, error) {
	reviewDetailID = strings.TrimSpace(reviewDetailID)
	if reviewDetailID == "" {
		return nil, fmt.Errorf("review detail id is required")
	}
	query := url.Values{}
	query.Set("include", "appSandboxEntitlement")
	query.Set("limit", appSandboxEntitlementPageSize)
	path := queryPath("/"+appStoreReviewDetailResourceType+"/"+url.PathEscape(reviewDetailID)+"/"+appSandboxEntitlementUsageResourceType, query)
	resources, raw, err := c.readCompleteAppSandboxCollection(ctx, path, "app sandbox entitlement usages")
	if err != nil {
		return nil, err
	}
	result := &AppSandboxEntitlementUsagesResult{Data: make([]AppSandboxEntitlementUsage, 0, len(resources)), Raw: raw}
	for _, resource := range resources {
		usage, err := decodeAppSandboxEntitlementUsage(resource)
		if err != nil {
			return nil, err
		}
		if usage.EntitlementID == "" {
			return nil, fmt.Errorf("app sandbox entitlement usage %q has no entitlement key", usage.ID)
		}
		result.Data = append(result.Data, usage)
	}
	return result, nil
}

// CreateAppSandboxEntitlementUsage adds one App Sandbox Information row to a
// review detail and returns the created row.
func (c *Client) CreateAppSandboxEntitlementUsage(ctx context.Context, reviewDetailID, entitlementID, justification string) (*AppSandboxEntitlementUsage, error) {
	reviewDetailID = strings.TrimSpace(reviewDetailID)
	entitlementID = strings.TrimSpace(entitlementID)
	if reviewDetailID == "" {
		return nil, fmt.Errorf("review detail id is required")
	}
	if entitlementID == "" {
		return nil, fmt.Errorf("entitlement is required")
	}
	payload := map[string]any{
		"data": map[string]any{
			"type": appSandboxEntitlementUsageResourceType,
			"attributes": map[string]string{
				"justification": justification,
			},
			"relationships": map[string]any{
				"appStoreReviewDetail": map[string]any{
					"data": map[string]string{"type": appStoreReviewDetailResourceType, "id": reviewDetailID},
				},
				"appSandboxEntitlement": map[string]any{
					"data": map[string]string{"type": appSandboxEntitlementResourceType, "id": entitlementID},
				},
			},
		},
	}
	body, err := c.doAppSandboxRequest(ctx, http.MethodPost, "/"+appSandboxEntitlementUsageResourceType, payload)
	if err != nil {
		return nil, err
	}
	return parseAppSandboxEntitlementUsageWriteResponse(body, "")
}

// UpdateAppSandboxEntitlementUsage replaces the usage information of one App
// Sandbox Information row.
func (c *Client) UpdateAppSandboxEntitlementUsage(ctx context.Context, usageID, justification string) (*AppSandboxEntitlementUsage, error) {
	usageID = strings.TrimSpace(usageID)
	if usageID == "" {
		return nil, fmt.Errorf("app sandbox entitlement usage id is required")
	}
	payload := map[string]any{
		"data": map[string]any{
			"type": appSandboxEntitlementUsageResourceType,
			"id":   usageID,
			"attributes": map[string]string{
				"justification": justification,
			},
		},
	}
	body, err := c.doAppSandboxRequest(ctx, http.MethodPatch, "/"+appSandboxEntitlementUsageResourceType+"/"+url.PathEscape(usageID), payload)
	if err != nil {
		return nil, err
	}
	return parseAppSandboxEntitlementUsageWriteResponse(body, usageID)
}

// DeleteAppSandboxEntitlementUsage removes one App Sandbox Information row.
func (c *Client) DeleteAppSandboxEntitlementUsage(ctx context.Context, usageID string) error {
	usageID = strings.TrimSpace(usageID)
	if usageID == "" {
		return fmt.Errorf("app sandbox entitlement usage id is required")
	}
	_, err := c.doAppSandboxRequest(ctx, http.MethodDelete, "/"+appSandboxEntitlementUsageResourceType+"/"+url.PathEscape(usageID), nil)
	return err
}

// doAppSandboxRequest sends the headers App Store Connect's own web client
// adds to every JSON:API request, including its CSRF marker.
func (c *Client) doAppSandboxRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
	headers := make(http.Header)
	headers.Set("Accept", "application/json")
	headers.Set("Content-Type", "application/json")
	headers.Set("Origin", appStoreBaseURL)
	headers.Set("Referer", appStoreBaseURL+"/")
	headers.Set("X-Requested-With", "XMLHttpRequest")
	headers.Set("X-CSRF-ITC", "[asc-ui]")
	return c.doRequestBase(ctx, c.baseURL, method, path, body, headers)
}

// readCompleteAppSandboxCollection reads one collection page and refuses to
// return it when Apple reports further pages, so callers never print or act
// on a partial collection.
func (c *Client) readCompleteAppSandboxCollection(ctx context.Context, path, responseName string) ([]jsonAPIResource, json.RawMessage, error) {
	body, err := c.doAppSandboxRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, nil, err
	}
	var envelope struct {
		Data  json.RawMessage `json:"data"`
		Links map[string]any  `json:"links"`
		Meta  struct {
			Paging struct {
				Total *int `json:"total"`
			} `json:"paging"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, nil, fmt.Errorf("failed to parse %s response: %w", responseName, err)
	}
	data := bytes.TrimSpace(envelope.Data)
	if len(data) == 0 || data[0] != '[' {
		return nil, nil, fmt.Errorf("%s response missing data collection", responseName)
	}
	var resources []jsonAPIResource
	if err := json.Unmarshal(data, &resources); err != nil {
		return nil, nil, fmt.Errorf("failed to parse %s response data: %w", responseName, err)
	}
	next, err := extractNextLink(envelope.Links)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid %s response pagination links: %w", responseName, err)
	}
	if next != "" {
		return nil, nil, fmt.Errorf("%s response has more than one page; refusing to use a partial collection", responseName)
	}
	if total := envelope.Meta.Paging.Total; total != nil && *total != len(resources) {
		return nil, nil, fmt.Errorf("%s response contains %d resources but paging total is %d; refusing to use a partial collection", responseName, len(resources), *total)
	}
	return resources, append(json.RawMessage(nil), body...), nil
}

func decodeAppSandboxEntitlementUsage(resource jsonAPIResource) (AppSandboxEntitlementUsage, error) {
	id := strings.TrimSpace(resource.ID)
	if resource.Type != appSandboxEntitlementUsageResourceType || id == "" {
		return AppSandboxEntitlementUsage{}, fmt.Errorf("app sandbox entitlement usages response contains resource type %q and id %q", resource.Type, resource.ID)
	}
	usage := AppSandboxEntitlementUsage{
		ID:            id,
		Justification: stringAttrPreserveWhitespace(resource.Attributes, "justification"),
	}
	if ref := firstRelationshipRef(resource, "appSandboxEntitlement"); ref != nil {
		if ref.Type != appSandboxEntitlementResourceType {
			return AppSandboxEntitlementUsage{}, fmt.Errorf("app sandbox entitlement usage %q entitlement relationship has type %q", id, ref.Type)
		}
		usage.EntitlementID = strings.TrimSpace(ref.ID)
	}
	return usage, nil
}

func parseAppSandboxEntitlementUsageWriteResponse(body []byte, wantID string) (*AppSandboxEntitlementUsage, error) {
	var payload struct {
		Data *jsonAPIResource `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse app sandbox entitlement usage response: %w", err)
	}
	if payload.Data == nil {
		return nil, fmt.Errorf("app sandbox entitlement usage response missing data")
	}
	usage, err := decodeAppSandboxEntitlementUsage(*payload.Data)
	if err != nil {
		return nil, err
	}
	if wantID != "" && usage.ID != wantID {
		return nil, fmt.Errorf("app sandbox entitlement usage response identifies %q, want %q", usage.ID, wantID)
	}
	return &usage, nil
}

func marshalAppSandboxRaw(raw json.RawMessage, responseName string) ([]byte, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("%s response is empty", responseName)
	}
	return append([]byte(nil), raw...), nil
}
