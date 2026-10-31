package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// reviewIAPFields contains the fields accepted by the legacy Iris IAP listing.
const reviewIAPFields = "productId,referenceName,state"

// ReviewIAP identifies a non-subscription IAP in the app-scoped Iris listing.
type ReviewIAP struct {
	ID                            string `json:"id"`
	ProductID                     string `json:"productId,omitempty"`
	ReferenceName                 string `json:"referenceName,omitempty"`
	State                         string `json:"state,omitempty"`
	SubmitWithNextAppStoreVersion bool   `json:"submitWithNextAppStoreVersion"`
}

// ReviewIAPSubmission is the receipt for an IAP version added to a review draft.
type ReviewIAPSubmission struct {
	// ID is the legacy inAppPurchaseSubmissions ID, empty for review-item attachment.
	ID                            string `json:"id"`
	ReviewSubmissionItemID        string `json:"reviewSubmissionItemId,omitempty"`
	InAppPurchaseID               string `json:"inAppPurchaseId,omitempty"`
	ReviewSubmissionID            string `json:"reviewSubmissionId,omitempty"`
	InAppPurchaseVersionID        string `json:"inAppPurchaseVersionId,omitempty"`
	SubmitWithNextAppStoreVersion bool   `json:"submitWithNextAppStoreVersion"`
}

func decodeReviewIAP(resource jsonAPIResource) ReviewIAP {
	return ReviewIAP{
		ID:                            strings.TrimSpace(resource.ID),
		ProductID:                     stringAttr(resource.Attributes, "productId"),
		ReferenceName:                 stringAttr(resource.Attributes, "referenceName"),
		State:                         stringAttr(resource.Attributes, "state"),
		SubmitWithNextAppStoreVersion: boolAttr(resource.Attributes, "submitWithNextAppStoreVersion"),
	}
}

// FindReviewIAP finds a single app-scoped IAP through the private web flow.
//
// The caller may pass either the iris IAP resource ID (a UUID, distinct from
// the numeric public-REST-API in-app purchase ID), the product ID
// (e.g. `com.example.pro.lifetime`), or the exact current reference name. The
// product-ID match exists because the public REST API surfaces a numeric ID
// that does not match the iris resource's ID, and users typically know either
// the iris UUID, product ID, or display name — not all three.
func (c *Client) FindReviewIAP(ctx context.Context, appID, iapID string) (ReviewIAP, bool, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return ReviewIAP{}, false, fmt.Errorf("app id is required")
	}
	iapID = strings.TrimSpace(iapID)
	if iapID == "" {
		return ReviewIAP{}, false, fmt.Errorf("iap id is required")
	}

	query := url.Values{}
	query.Set("fields[inAppPurchases]", reviewIAPFields)
	query.Set("limit", "300")
	query.Set("sort", "referenceName")

	nextPath := queryPath("/apps/"+url.PathEscape(appID)+"/inAppPurchases", query)
	visited := map[string]struct{}{}
	productIDMatches := make([]ReviewIAP, 0, 1)
	referenceNameMatches := make([]ReviewIAP, 0, 1)

	for nextPath != "" {
		if _, seen := visited[nextPath]; seen {
			return ReviewIAP{}, false, fmt.Errorf("review iaps pagination loop detected")
		}
		visited[nextPath] = struct{}{}

		responseBody, err := c.doRequest(ctx, http.MethodGet, nextPath, nil)
		if err != nil {
			return ReviewIAP{}, false, err
		}

		var payload jsonAPIListPayload
		if err := json.Unmarshal(responseBody, &payload); err != nil {
			return ReviewIAP{}, false, fmt.Errorf("failed to parse review iaps response: %w", err)
		}
		for _, resource := range payload.Data {
			if resourceType := strings.TrimSpace(resource.Type); !strings.EqualFold(resourceType, "inAppPurchases") {
				return ReviewIAP{}, false, fmt.Errorf("failed to parse review iaps response: unexpected resource type %q", resource.Type)
			}
			decoded := decodeReviewIAP(resource)
			if decoded.ID == iapID {
				return decoded, true, nil
			}
			if decoded.ProductID == iapID {
				appendUniqueReviewIAPMatch(&productIDMatches, decoded)
			}
			if strings.EqualFold(strings.TrimSpace(decoded.ReferenceName), iapID) {
				appendUniqueReviewIAPMatch(&referenceNameMatches, decoded)
			}
		}

		nextLink, err := extractNextLink(payload.Links)
		if err != nil {
			return ReviewIAP{}, false, fmt.Errorf("failed to parse review iaps pagination links: %w", err)
		}
		if strings.TrimSpace(nextLink) == "" {
			break
		}
		nextPath, err = normalizeNextPath(nextLink, c.baseURL)
		if err != nil {
			return ReviewIAP{}, false, fmt.Errorf("failed to normalize review iaps pagination link: %w", err)
		}
	}

	if len(productIDMatches) > 1 {
		return ReviewIAP{}, false, ambiguousReviewIAPSelectorError(iapID, "product ID", productIDMatches)
	}
	if len(productIDMatches) == 1 {
		return productIDMatches[0], true, nil
	}
	if len(referenceNameMatches) > 1 {
		return ReviewIAP{}, false, ambiguousReviewIAPSelectorError(iapID, "reference name", referenceNameMatches)
	}
	if len(referenceNameMatches) == 1 {
		return referenceNameMatches[0], true, nil
	}

	return ReviewIAP{}, false, nil
}

// ResolveReviewIAPV2ID resolves the selected legacy IAP to its app-scoped V2 ID.
func (c *Client) ResolveReviewIAPV2ID(ctx context.Context, appID, productID string) (string, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return "", fmt.Errorf("app id is required")
	}
	if productID == "" {
		return "", fmt.Errorf("selected in-app purchase is missing a product ID; refusing to attach")
	}
	query := url.Values{}
	query.Set("fields[inAppPurchases]", "productId")
	query.Set("limit", "300")
	payload, err := c.fetchJSONAPIPages(ctx, queryPath("/apps/"+url.PathEscape(appID)+"/inAppPurchasesV2", query), "review IAP V2 catalog")
	if err != nil {
		return "", err
	}
	resolvedID := ""
	for _, resource := range payload.Data {
		if resource.Type != "inAppPurchases" {
			return "", fmt.Errorf("review IAP V2 catalog: unexpected resource type %q", resource.Type)
		}
		if stringAttrPreserveWhitespace(resource.Attributes, "productId") != productID {
			continue
		}
		id := strings.TrimSpace(resource.ID)
		if id == "" {
			return "", fmt.Errorf("review IAP V2 catalog: missing resource id for product ID %q; refusing to attach", productID)
		}
		if resolvedID != "" && resolvedID != id {
			return "", fmt.Errorf("multiple V2 in-app purchases for product ID %q under app %q; refusing to attach", productID, appID)
		}
		resolvedID = id
	}
	if resolvedID == "" {
		return "", fmt.Errorf("no V2 in-app purchase for product ID %q under app %q; refusing to attach", productID, appID)
	}
	return resolvedID, nil
}

func appendUniqueReviewIAPMatch(matches *[]ReviewIAP, candidate ReviewIAP) {
	for _, existing := range *matches {
		if strings.TrimSpace(existing.ID) == strings.TrimSpace(candidate.ID) {
			return
		}
	}
	*matches = append(*matches, candidate)
}

// ReviewIAPAmbiguousError reports that a selector matched more than one IAP.
// Matches remain available to the CLI so it can render bounded, terminal-safe
// recovery details without making this web client depend on CLI packages.
type ReviewIAPAmbiguousError struct {
	Selector string
	Field    string
	Matches  []ReviewIAP
}

func (e *ReviewIAPAmbiguousError) Error() string {
	return fmt.Sprintf(
		"%q matches %d in-app purchases by %s",
		strings.TrimSpace(e.Selector),
		len(e.Matches),
		strings.TrimSpace(e.Field),
	)
}

func ambiguousReviewIAPSelectorError(selector, field string, matches []ReviewIAP) error {
	return &ReviewIAPAmbiguousError{
		Selector: selector,
		Field:    field,
		Matches:  matches,
	}
}

// ResolveReviewIAPVersionID finds the single editable version of an app-scoped IAP.
func (c *Client) ResolveReviewIAPVersionID(ctx context.Context, iapID string) (string, error) {
	iapID = strings.TrimSpace(iapID)
	if iapID == "" {
		return "", fmt.Errorf("iap id is required")
	}
	payload, err := c.fetchJSONAPIPagesFrom(ctx, c.webIrisV2BaseURL(), "/inAppPurchases/"+url.PathEscape(iapID)+"/versions?limit=200", "review IAP versions")
	if err != nil {
		return "", err
	}
	selected := ""
	for _, version := range payload.Data {
		if version.Type != "inAppPurchaseVersions" || strings.TrimSpace(version.ID) == "" {
			return "", fmt.Errorf("review IAP versions: invalid resource identity; refusing to attach")
		}
		switch stringAttr(version.Attributes, "state") {
		case "PREPARE_FOR_SUBMISSION", "READY_FOR_REVIEW", "REJECTED", "DEVELOPER_REJECTED":
			if selected != "" && selected != version.ID {
				return "", fmt.Errorf("multiple editable versions for in-app purchase %q; refusing to attach", iapID)
			}
			selected = version.ID
		}
	}
	if selected == "" {
		return "", fmt.Errorf("no editable version for in-app purchase %q; refusing to attach", iapID)
	}
	return selected, nil
}

// CreateReviewIAPSubmissionItem adds an IAP version to an existing review draft.
func (c *Client) CreateReviewIAPSubmissionItem(ctx context.Context, submissionID, versionID string) (string, error) {
	submissionID, versionID = strings.TrimSpace(submissionID), strings.TrimSpace(versionID)
	if submissionID == "" || versionID == "" {
		return "", fmt.Errorf("review submission id and IAP version id are required")
	}
	body := map[string]any{
		"data": map[string]any{
			"type": "reviewSubmissionItems",
			"relationships": map[string]any{
				"reviewSubmission":     map[string]any{"data": resourceRef{Type: "reviewSubmissions", ID: submissionID}},
				"inAppPurchaseVersion": map[string]any{"data": resourceRef{Type: "inAppPurchaseVersions", ID: versionID}},
			},
		},
	}
	response, err := c.doRequest(ctx, http.MethodPost, "/reviewSubmissionItems", body)
	if err != nil {
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Status >= http.StatusInternalServerError {
			return "", fmt.Errorf("review item may have been created; verify submission %q before retrying: %w", submissionID, err)
		}
		return "", fmt.Errorf("iris POST /reviewSubmissionItems: %w", err)
	}
	resource, _, err := decodeResolutionCenterWriteResource(response, "review submission item create", "reviewSubmissionItems")
	if err != nil {
		return "", fmt.Errorf("review item may have been created; verify submission %q before retrying: %w", submissionID, err)
	}
	for _, target := range []struct{ name, kind, id string }{
		{"inAppPurchaseVersion", "inAppPurchaseVersions", versionID},
		{"reviewSubmission", "reviewSubmissions", submissionID},
	} {
		if _, _, err := validateReviewSubmissionRelationship(resource, target.name, target.kind, target.id); err != nil {
			return "", fmt.Errorf("review item %q may have been created; verify submission %q before retrying: %w", resource.ID, submissionID, err)
		}
	}
	return strings.TrimSpace(resource.ID), nil
}
