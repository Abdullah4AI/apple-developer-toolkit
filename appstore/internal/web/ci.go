package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// NewCIClient creates a CI API client reusing an authenticated web session.
// The CI API lives at /ci/api and uses the same session cookies as IRIS.
func NewCIClient(session *AuthSession) *Client {
	return &Client{
		httpClient:         session.Client,
		baseURL:            appStoreBaseURL + "/ci/api",
		minRequestInterval: resolveWebMinRequestInterval(),
	}
}

// NOTE: The CI API (/ci/api) uses snake_case JSON keys and query parameters,
// unlike the IRIS API (/iris/v1) which uses camelCase. Confirmed via browser
// network inspection of the ASC web UI.

// CIUsageSummary is the response from the usage summary endpoint.
type CIUsageSummary struct {
	Plan  CIUsagePlan       `json:"plan"`
	Links map[string]string `json:"links,omitempty"`
}

// CIUsagePlan describes the Xcode Cloud plan quota.
type CIUsagePlan struct {
	Name          string `json:"name"`
	ResetDate     string `json:"reset_date"`
	ResetDateTime string `json:"reset_date_time"`
	Available     int    `json:"available"`
	Used          int    `json:"used"`
	Total         int    `json:"total"`
}

// CIUsageMonths is the response from the monthly usage endpoint.
type CIUsageMonths struct {
	Usage        []CIMonthUsage   `json:"usage"`
	ProductUsage []CIProductUsage `json:"product_usage"`
	Info         CIUsageInfo      `json:"info"`
}

// CIMonthUsage describes usage for a single month.
type CIMonthUsage struct {
	Month          int `json:"month"`
	Year           int `json:"year"`
	Duration       int `json:"duration"`
	NumberOfBuilds int `json:"number_of_builds,omitempty"`
}

// CIProductUsage describes per-product monthly usage.
type CIProductUsage struct {
	ProductID              string         `json:"product_id"`
	ProductName            string         `json:"product_name,omitempty"`
	BundleID               string         `json:"bundle_id,omitempty"`
	Usage                  []CIMonthUsage `json:"usage,omitempty"`
	UsageInMinutes         int            `json:"usage_in_minutes,omitempty"`
	UsageInSeconds         int            `json:"usage_in_seconds,omitempty"`
	NumberOfBuilds         int            `json:"number_of_builds,omitempty"`
	PreviousUsageInMinutes int            `json:"previous_usage_in_minutes,omitempty"`
	PreviousNumberOfBuilds int            `json:"previous_number_of_builds,omitempty"`
}

// CIUsageInfo holds metadata about the usage response.
type CIUsageInfo struct {
	StartMonth         int                `json:"start_month,omitempty"`
	StartYear          int                `json:"start_year,omitempty"`
	EndMonth           int                `json:"end_month,omitempty"`
	EndYear            int                `json:"end_year,omitempty"`
	CanViewAllProducts bool               `json:"can_view_all_products,omitempty"`
	Current            CIUsageInfoCurrent `json:"current,omitempty"`
	Previous           CIUsageInfoCurrent `json:"previous,omitempty"`
	Links              map[string]string  `json:"links,omitempty"`
}

// CIUsageInfoCurrent summarizes usage in the current/previous period.
type CIUsageInfoCurrent struct {
	Builds        int `json:"builds"`
	Used          int `json:"used"`
	Average30Days int `json:"average_30_days"`
}

// CIUsageDays is the response from the daily usage endpoint.
type CIUsageDays struct {
	Usage         []CIDayUsage      `json:"usage"`
	ProductUsage  []CIProductUsage  `json:"product_usage,omitempty"`
	WorkflowUsage []CIWorkflowUsage `json:"workflow_usage"`
	Info          CIUsageInfo       `json:"info"`
}

// CIDayUsage describes usage for a single day.
type CIDayUsage struct {
	Date           string `json:"date"`
	Duration       int    `json:"duration"`
	NumberOfBuilds int    `json:"number_of_builds,omitempty"`
}

// CIWorkflowUsage describes per-workflow daily usage.
type CIWorkflowUsage struct {
	WorkflowID             string       `json:"workflow_id"`
	WorkflowName           string       `json:"workflow_name,omitempty"`
	Usage                  []CIDayUsage `json:"usage,omitempty"`
	UsageInMinutes         int          `json:"usage_in_minutes,omitempty"`
	NumberOfBuilds         int          `json:"number_of_builds,omitempty"`
	PreviousUsageInMinutes int          `json:"previous_usage_in_minutes,omitempty"`
	PreviousNumberOfBuilds int          `json:"previous_number_of_builds,omitempty"`
}

// CIProduct describes a Xcode Cloud product.
type CIProduct struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	BundleID string `json:"bundle_id"`
	Type     string `json:"type"`
	IconURL  string `json:"icon_url,omitempty"`
}

// CIProductListResponse is the response from the products endpoint.
type CIProductListResponse struct {
	Items []CIProduct `json:"items"`
}

// CIWorkflow describes a Xcode Cloud workflow.
type CIWorkflow struct {
	ID      string            `json:"id"`
	Content CIWorkflowContent `json:"content"`
}

// CIWorkflowContent holds the workflow's configuration including its name.
type CIWorkflowContent struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// CIWorkflowListResponse is the response from the workflows endpoint.
type CIWorkflowListResponse struct {
	Items []CIWorkflow `json:"items"`
}

func (m *CIMonthUsage) UnmarshalJSON(data []byte) error {
	type alias struct {
		Month          int  `json:"month"`
		Year           int  `json:"year"`
		Duration       *int `json:"duration"`
		Minutes        *int `json:"minutes"`
		NumberOfBuilds int  `json:"number_of_builds"`
	}
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	m.Month = value.Month
	m.Year = value.Year
	m.NumberOfBuilds = value.NumberOfBuilds
	switch {
	case value.Duration != nil:
		m.Duration = *value.Duration
	case value.Minutes != nil:
		m.Duration = *value.Minutes
	default:
		m.Duration = 0
	}
	return nil
}

func (d *CIDayUsage) UnmarshalJSON(data []byte) error {
	type alias struct {
		Date           string `json:"date"`
		Duration       *int   `json:"duration"`
		Minutes        *int   `json:"minutes"`
		NumberOfBuilds int    `json:"number_of_builds"`
	}
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	d.Date = value.Date
	d.NumberOfBuilds = value.NumberOfBuilds
	switch {
	case value.Duration != nil:
		d.Duration = *value.Duration
	case value.Minutes != nil:
		d.Duration = *value.Minutes
	default:
		d.Duration = 0
	}
	return nil
}

// GetCIUsageSummary retrieves the Xcode Cloud plan usage summary.
func (c *Client) GetCIUsageSummary(ctx context.Context, teamID string) (*CIUsageSummary, error) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil, fmt.Errorf("team id is required")
	}
	path := "/teams/" + url.PathEscape(teamID) + "/usage/summary"
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result CIUsageSummary
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode ci usage summary: %w", err)
	}
	return &result, nil
}

// GetCIUsageMonths retrieves monthly Xcode Cloud usage for a date range.
func (c *Client) GetCIUsageMonths(ctx context.Context, teamID string, startMonth, startYear, endMonth, endYear int) (*CIUsageMonths, error) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil, fmt.Errorf("team id is required")
	}
	query := url.Values{}
	query.Set("start_month", strconv.Itoa(startMonth))
	query.Set("start_year", strconv.Itoa(startYear))
	query.Set("end_month", strconv.Itoa(endMonth))
	query.Set("end_year", strconv.Itoa(endYear))
	path := queryPath("/teams/"+url.PathEscape(teamID)+"/usage/months", query)
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result CIUsageMonths
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode ci usage months: %w", err)
	}
	return &result, nil
}

// GetCIUsageDays retrieves daily Xcode Cloud usage for a product in a date range.
func (c *Client) GetCIUsageDays(ctx context.Context, teamID, productID, start, end string) (*CIUsageDays, error) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil, fmt.Errorf("team id is required")
	}
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return nil, fmt.Errorf("product id is required")
	}
	start = strings.TrimSpace(start)
	if start == "" {
		return nil, fmt.Errorf("start date is required")
	}
	end = strings.TrimSpace(end)
	if end == "" {
		return nil, fmt.Errorf("end date is required")
	}
	query := url.Values{}
	query.Set("start", start)
	query.Set("end", end)
	path := queryPath("/teams/"+url.PathEscape(teamID)+"/products/"+url.PathEscape(productID)+"/usage/days", query)
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result CIUsageDays
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode ci usage days: %w", err)
	}
	return &result, nil
}

// GetCIUsageDaysOverall retrieves daily Xcode Cloud usage overview for a team.
func (c *Client) GetCIUsageDaysOverall(ctx context.Context, teamID, start, end string) (*CIUsageDays, error) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil, fmt.Errorf("team id is required")
	}
	start = strings.TrimSpace(start)
	if start == "" {
		return nil, fmt.Errorf("start date is required")
	}
	end = strings.TrimSpace(end)
	if end == "" {
		return nil, fmt.Errorf("end date is required")
	}
	query := url.Values{}
	query.Set("start", start)
	query.Set("end", end)
	path := queryPath("/teams/"+url.PathEscape(teamID)+"/usage/days", query)
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result CIUsageDays
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode ci usage days overview: %w", err)
	}
	return &result, nil
}

// ListCIProducts lists Xcode Cloud products for a team.
// The CI API does not expose pagination for this endpoint; limit=100 covers
// the vast majority of teams.
func (c *Client) ListCIProducts(ctx context.Context, teamID string) (*CIProductListResponse, error) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil, fmt.Errorf("team id is required")
	}
	query := url.Values{}
	query.Set("limit", "100")
	path := queryPath("/teams/"+url.PathEscape(teamID)+"/products-v4", query)
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result CIProductListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode ci products: %w", err)
	}
	return &result, nil
}

// ListCIWorkflows lists Xcode Cloud workflows for a product.
func (c *Client) ListCIWorkflows(ctx context.Context, teamID, productID string) (*CIWorkflowListResponse, error) {
	teamID = strings.TrimSpace(teamID)
	if teamID == "" {
		return nil, fmt.Errorf("team id is required")
	}
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return nil, fmt.Errorf("product id is required")
	}
	query := url.Values{}
	query.Set("limit", "100")
	query.Set("include_deleted", "false")
	path := queryPath("/teams/"+url.PathEscape(teamID)+"/products/"+url.PathEscape(productID)+"/workflows-v15", query)
	body, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var result CIWorkflowListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode ci workflows: %w", err)
	}
	return &result, nil
}
