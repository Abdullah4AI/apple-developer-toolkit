package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// reviewIAPFields is the set of `fields[inAppPurchases]` keys Apple's iris
// `/apps/{APP_ID}/inAppPurchases` endpoint accepts.
//
// The iris flavor of this resource is narrower than the public REST API:
// `name`, `inAppPurchaseType`, `isAppStoreReviewInProgress`, and
// `submitWithNextAppStoreVersion` all return `PARAMETER_ERROR.INVALID`
// here. Apple uses `referenceName` for the human-readable label on this
// endpoint. The post-attach response from `/inAppPurchaseSubmissions`
// carries `submitWithNextAppStoreVersion`, so the listing only needs the
// fields below to scope the IAP being attached.
const reviewIAPFields = "productId,referenceName,state"

// ReviewIAP summarizes a non-subscription IAP returned by the iris listing
// used during the next app version review attach flow.
//
// `SubmitWithNextAppStoreVersion` is retained on the struct for API
// compatibility but is never populated from the iris listing — Apple's
// `/apps/{APP_ID}/inAppPurchases` rejects it as an invalid field. The
// post-attach response from `/inAppPurchaseSubmissions` carries the same
// flag; see ReviewIAPSubmission for the populated version.
type ReviewIAP struct {
	ID                            string `json:"id"`
	ProductID                     string `json:"productId,omitempty"`
	ReferenceName                 string `json:"referenceName,omitempty"`
	State                         string `json:"state,omitempty"`
	SubmitWithNextAppStoreVersion bool   `json:"submitWithNextAppStoreVersion"`
}

// ReviewIAPSubmission captures the hidden submission resource returned by the
// web flow that attaches a non-renewing in-app purchase to the next app version
// review. Mirrors ReviewSubscriptionSubmission but for non-subscription IAPs.
type ReviewIAPSubmission struct {
	ID                            string `json:"id"`
	InAppPurchaseID               string `json:"inAppPurchaseId,omitempty"`
	SubmitWithNextAppStoreVersion bool   `json:"submitWithNextAppStoreVersion"`
}

func decodeReviewIAP(resource jsonAPIResource) ReviewIAP {
	return ReviewIAP{
		ID:            strings.TrimSpace(resource.ID),
		ProductID:     stringAttr(resource.Attributes, "productId"),
		ReferenceName: stringAttr(resource.Attributes, "referenceName"),
		State:         stringAttr(resource.Attributes, "state"),
		// Decoded defensively — Apple's iris listing doesn't currently include
		// it (the field is not in reviewIAPFields), but if Apple starts
		// returning it the idempotency short-circuit in the attach command
		// will pick it up without further code changes.
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

func appendUniqueReviewIAPMatch(matches *[]ReviewIAP, candidate ReviewIAP) {
	for _, existing := range *matches {
		if strings.TrimSpace(existing.ID) == strings.TrimSpace(candidate.ID) {
			return
		}
	}
	*matches = append(*matches, candidate)
}

func ambiguousReviewIAPSelectorError(selector, field string, matches []ReviewIAP) error {
	lines := make([]string, 0, len(matches))
	for _, match := range matches {
		line := strings.TrimSpace(match.ID)
		if productID := strings.TrimSpace(match.ProductID); productID != "" {
			line += ", productId=" + productID
		}
		if referenceName := strings.TrimSpace(match.ReferenceName); referenceName != "" {
			line += ", referenceName=" + referenceName
		}
		lines = append(lines, line)
	}
	return fmt.Errorf(
		"%q matches %d in-app purchases by %s:\n  %s\nUse the Iris resource ID to disambiguate",
		selector,
		len(matches),
		field,
		strings.Join(lines, "\n  "),
	)
}

// CreateInAppPurchaseSubmission attaches a non-renewing in-app purchase to the
// next app version review via the private web flow.
//
// This is the iris-API equivalent of clicking the IAP checkbox in
// "Add App In-App Purchase or Subscription" on the version submission page
// in App Store Connect. POST /iris/v1/inAppPurchaseSubmissions with the
// `submitWithNextAppStoreVersion` attribute set; the public ASC REST API
// has no equivalent for non-subscription IAPs.
func (c *Client) CreateInAppPurchaseSubmission(ctx context.Context, iapID string) (ReviewIAPSubmission, error) {
	iapID = strings.TrimSpace(iapID)
	if iapID == "" {
		return ReviewIAPSubmission{}, fmt.Errorf("iap id is required")
	}

	body := map[string]any{
		"data": map[string]any{
			"type": "inAppPurchaseSubmissions",
			"attributes": map[string]any{
				"submitWithNextAppStoreVersion": true,
			},
			"relationships": map[string]any{
				"inAppPurchaseV2": map[string]any{
					"data": map[string]string{
						"type": "inAppPurchases",
						"id":   iapID,
					},
				},
			},
		},
	}

	responseBody, err := c.doRequest(ctx, http.MethodPost, "/inAppPurchaseSubmissions", body)
	if err != nil {
		return ReviewIAPSubmission{}, fmt.Errorf("iris POST /inAppPurchaseSubmissions: %w", err)
	}

	var payload struct {
		Data jsonAPIResource `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return ReviewIAPSubmission{}, fmt.Errorf("failed to parse iap submission response: %w", err)
	}
	if strings.TrimSpace(payload.Data.ID) == "" {
		return ReviewIAPSubmission{}, fmt.Errorf("failed to parse iap submission response: missing submission id")
	}
	if submissionType := strings.TrimSpace(payload.Data.Type); submissionType == "" {
		return ReviewIAPSubmission{}, fmt.Errorf("failed to parse iap submission response: missing submission resource type")
	} else if submissionType != "inAppPurchaseSubmissions" {
		return ReviewIAPSubmission{}, fmt.Errorf("failed to parse iap submission response: unexpected resource type %q", payload.Data.Type)
	}

	result := ReviewIAPSubmission{
		ID:                            strings.TrimSpace(payload.Data.ID),
		SubmitWithNextAppStoreVersion: boolAttr(payload.Data.Attributes, "submitWithNextAppStoreVersion"),
	}
	relationshipID, relationshipPresent, err := validateReviewSubmissionRelationship(payload.Data, "inAppPurchaseV2", "inAppPurchases", iapID)
	if err != nil {
		return ReviewIAPSubmission{}, fmt.Errorf("failed to parse iap submission response: %w", err)
	}
	if relationshipPresent {
		result.InAppPurchaseID = relationshipID
	} else {
		result.InAppPurchaseID = iapID
	}
	return result, nil
}
