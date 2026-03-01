package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetCIUsageSummaryParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/teams/team-uuid/usage/summary" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"plan": {
				"name": "Plan",
				"available": 1467,
				"used": 33,
				"total": 1500,
				"reset_date": "2026-03-16",
				"reset_date_time": "2026-03-16T09:43:54Z"
			},
			"links": {
				"manage": "https://developer.apple.com/xcode-cloud/"
			}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	result, err := client.GetCIUsageSummary(context.Background(), "team-uuid")
	if err != nil {
		t.Fatalf("GetCIUsageSummary() error = %v", err)
	}
	if result.Plan.Name != "Plan" {
		t.Fatalf("expected plan name %q, got %q", "Plan", result.Plan.Name)
	}
	if result.Plan.Available != 1467 {
		t.Fatalf("expected available 1467, got %d", result.Plan.Available)
	}
	if result.Plan.Used != 33 {
		t.Fatalf("expected used 33, got %d", result.Plan.Used)
	}
	if result.Plan.Total != 1500 {
		t.Fatalf("expected total 1500, got %d", result.Plan.Total)
	}
	if result.Plan.ResetDate != "2026-03-16" {
		t.Fatalf("expected reset_date %q, got %q", "2026-03-16", result.Plan.ResetDate)
	}
	if result.Plan.ResetDateTime != "2026-03-16T09:43:54Z" {
		t.Fatalf("expected reset_date_time %q, got %q", "2026-03-16T09:43:54Z", result.Plan.ResetDateTime)
	}
	if result.Links["manage"] != "https://developer.apple.com/xcode-cloud/" {
		t.Fatalf("expected manage link, got %v", result.Links)
	}
}

func TestGetCIUsageSummaryRejectsEmptyTeamID(t *testing.T) {
	client := &Client{httpClient: http.DefaultClient, baseURL: "http://localhost"}
	_, err := client.GetCIUsageSummary(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty team ID")
	}
	if !strings.Contains(err.Error(), "team id is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetCIUsageMonthsQueryParams(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"usage":[],"product_usage":[],"info":{}}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	_, err := client.GetCIUsageMonths(context.Background(), "team-uuid", 1, 2025, 12, 2025)
	if err != nil {
		t.Fatalf("GetCIUsageMonths() error = %v", err)
	}
	for _, param := range []string{"start_month=1", "start_year=2025", "end_month=12", "end_year=2025"} {
		if !strings.Contains(gotQuery, param) {
			t.Fatalf("expected query to contain %q, got %q", param, gotQuery)
		}
	}
}

func TestGetCIUsageMonthsParsesProductUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"usage": [{"month":1,"year":2026,"minutes":120,"number_of_builds":3}],
			"product_usage": [
				{
					"product_id": "prod-1",
					"usage_in_minutes": 120,
					"usage_in_seconds": 7200,
					"number_of_builds": 3,
					"previous_usage_in_minutes": 80,
					"previous_number_of_builds": 2
				}
			],
			"info": {
				"can_view_all_products": true,
				"current": {"builds":3,"used":120,"average_30_days":60},
				"previous": {"builds":2,"used":80,"average_30_days":40}
			}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	result, err := client.GetCIUsageMonths(context.Background(), "team-uuid", 1, 2026, 1, 2026)
	if err != nil {
		t.Fatalf("GetCIUsageMonths() error = %v", err)
	}
	if len(result.Usage) != 1 || result.Usage[0].Duration != 120 || result.Usage[0].NumberOfBuilds != 3 {
		t.Fatalf("unexpected usage: %+v", result.Usage)
	}
	if len(result.ProductUsage) != 1 {
		t.Fatalf("expected 1 product usage, got %d", len(result.ProductUsage))
	}
	pu := result.ProductUsage[0]
	if pu.ProductID != "prod-1" || pu.UsageInMinutes != 120 || pu.NumberOfBuilds != 3 || pu.PreviousUsageInMinutes != 80 || pu.PreviousNumberOfBuilds != 2 {
		t.Fatalf("unexpected product usage: %+v", pu)
	}
	if !result.Info.CanViewAllProducts || result.Info.Current.Used != 120 || result.Info.Previous.Used != 80 {
		t.Fatalf("unexpected info: %+v", result.Info)
	}
}

func TestGetCIUsageMonthsRejectsEmptyTeamID(t *testing.T) {
	client := &Client{httpClient: http.DefaultClient, baseURL: "http://localhost"}
	_, err := client.GetCIUsageMonths(context.Background(), "  ", 1, 2026, 1, 2026)
	if err == nil {
		t.Fatal("expected error for empty team ID")
	}
	if !strings.Contains(err.Error(), "team id is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetCIUsageDaysParsesWorkflowUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/products/prod-1/usage/days") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("start") != "2026-01-01" || r.URL.Query().Get("end") != "2026-01-31" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"usage": [{"date":"2026-01-01","minutes":60,"number_of_builds":2}],
			"workflow_usage": [
				{
					"workflow_id": "wf-1",
					"usage_in_minutes": 60,
					"number_of_builds": 2,
					"previous_usage_in_minutes": 30,
					"previous_number_of_builds": 1
				}
			],
			"info": {"current":{"builds":2,"used":60,"average_30_days":45}}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	result, err := client.GetCIUsageDays(context.Background(), "team-uuid", "prod-1", "2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatalf("GetCIUsageDays() error = %v", err)
	}
	if len(result.Usage) != 1 || result.Usage[0].Duration != 60 || result.Usage[0].NumberOfBuilds != 2 {
		t.Fatalf("unexpected usage: %+v", result.Usage)
	}
	if len(result.WorkflowUsage) != 1 {
		t.Fatalf("expected 1 workflow usage, got %d", len(result.WorkflowUsage))
	}
	wf := result.WorkflowUsage[0]
	if wf.WorkflowID != "wf-1" || wf.UsageInMinutes != 60 || wf.NumberOfBuilds != 2 || wf.PreviousUsageInMinutes != 30 || wf.PreviousNumberOfBuilds != 1 {
		t.Fatalf("unexpected workflow usage: %+v", wf)
	}
	if result.Info.Current.Used != 60 {
		t.Fatalf("unexpected current usage info: %+v", result.Info.Current)
	}
}

func TestGetCIUsageDaysOverallParsesProductUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/teams/team-uuid/usage/days") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("start") != "2026-01-01" || r.URL.Query().Get("end") != "2026-01-31" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"usage": [{"date":"2026-01-01","minutes":90,"number_of_builds":4}],
			"product_usage": [
				{
					"product_id":"prod-1",
					"usage_in_minutes":60,
					"number_of_builds":2,
					"previous_usage_in_minutes":30,
					"previous_number_of_builds":1
				}
			],
			"info": {"current":{"builds":4,"used":90,"average_30_days":75}}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	result, err := client.GetCIUsageDaysOverall(context.Background(), "team-uuid", "2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatalf("GetCIUsageDaysOverall() error = %v", err)
	}
	if len(result.Usage) != 1 || result.Usage[0].Duration != 90 {
		t.Fatalf("unexpected usage: %+v", result.Usage)
	}
	if len(result.ProductUsage) != 1 {
		t.Fatalf("expected 1 product usage row, got %d", len(result.ProductUsage))
	}
	pu := result.ProductUsage[0]
	if pu.ProductID != "prod-1" || pu.UsageInMinutes != 60 || pu.NumberOfBuilds != 2 || pu.PreviousUsageInMinutes != 30 || pu.PreviousNumberOfBuilds != 1 {
		t.Fatalf("unexpected product usage: %+v", pu)
	}
	if result.Info.Current.Used != 90 || result.Info.Current.Builds != 4 {
		t.Fatalf("unexpected current info: %+v", result.Info.Current)
	}
}

func TestGetCIUsageDaysOverallRejectsEmptyInputs(t *testing.T) {
	client := &Client{httpClient: http.DefaultClient, baseURL: "http://localhost"}
	tests := []struct {
		name    string
		teamID  string
		start   string
		end     string
		wantErr string
	}{
		{"empty team", "", "2026-01-01", "2026-01-31", "team id is required"},
		{"empty start", "team", "", "2026-01-31", "start date is required"},
		{"empty end", "team", "2026-01-01", "", "end date is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.GetCIUsageDaysOverall(context.Background(), tt.teamID, tt.start, tt.end)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestCIMonthUsageUnmarshalSupportsDurationAlias(t *testing.T) {
	var usage CIMonthUsage
	if err := json.Unmarshal([]byte(`{"year":2026,"month":2,"duration":33}`), &usage); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if usage.Duration != 33 {
		t.Fatalf("expected duration 33, got %d", usage.Duration)
	}
}

func TestCIDayUsageUnmarshalSupportsDurationAlias(t *testing.T) {
	var usage CIDayUsage
	if err := json.Unmarshal([]byte(`{"date":"2026-02-20","duration":17}`), &usage); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if usage.Duration != 17 {
		t.Fatalf("expected duration 17, got %d", usage.Duration)
	}
}

func TestGetCIUsageDaysRejectsEmptyInputs(t *testing.T) {
	client := &Client{httpClient: http.DefaultClient, baseURL: "http://localhost"}
	tests := []struct {
		name      string
		teamID    string
		productID string
		start     string
		end       string
		wantErr   string
	}{
		{"empty team", "", "prod", "2026-01-01", "2026-01-31", "team id is required"},
		{"empty product", "team", "", "2026-01-01", "2026-01-31", "product id is required"},
		{"empty start", "team", "prod", "", "2026-01-31", "start date is required"},
		{"empty end", "team", "prod", "2026-01-01", "", "end date is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.GetCIUsageDays(context.Background(), tt.teamID, tt.productID, tt.start, tt.end)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestListCIProductsParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/products-v4") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("limit") != "100" {
			t.Fatalf("expected limit=100, got %q", r.URL.Query().Get("limit"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items": [
				{"id":"prod-1","name":"My App","bundle_id":"com.example.app","type":"solo"},
				{"id":"prod-2","name":"Other App","bundle_id":"com.other.app","type":"solo","icon_url":"https://example.com/icon.png"}
			]
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	result, err := client.ListCIProducts(context.Background(), "team-uuid")
	if err != nil {
		t.Fatalf("ListCIProducts() error = %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 products, got %d", len(result.Items))
	}
	if result.Items[0].ID != "prod-1" || result.Items[0].BundleID != "com.example.app" {
		t.Fatalf("unexpected first product: %+v", result.Items[0])
	}
	if result.Items[1].IconURL != "https://example.com/icon.png" {
		t.Fatalf("expected icon_url, got %q", result.Items[1].IconURL)
	}
}

func TestListCIProductsRejectsEmptyTeamID(t *testing.T) {
	client := &Client{httpClient: http.DefaultClient, baseURL: "http://localhost"}
	_, err := client.ListCIProducts(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty team ID")
	}
	if !strings.Contains(err.Error(), "team id is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetCIUsageSummaryHandles4xxError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	_, err := client.GetCIUsageSummary(context.Background(), "team-uuid")
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.Status != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", apiErr.Status)
	}
}

func TestCIUsagePlanJSONRoundTrip(t *testing.T) {
	raw := `{"name":"Plan","reset_date":"2026-03-16","reset_date_time":"2026-03-16T09:43:54Z","available":1467,"used":33,"total":1500}`
	var plan CIUsagePlan
	if err := json.Unmarshal([]byte(raw), &plan); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if plan.ResetDate != "2026-03-16" {
		t.Fatalf("expected reset_date %q, got %q", "2026-03-16", plan.ResetDate)
	}
	if plan.ResetDateTime != "2026-03-16T09:43:54Z" {
		t.Fatalf("expected reset_date_time %q, got %q", "2026-03-16T09:43:54Z", plan.ResetDateTime)
	}

	out, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	if !strings.Contains(string(out), `"reset_date":"2026-03-16"`) {
		t.Fatalf("expected reset_date in output, got %s", out)
	}
}

func TestNewCIClientSetsBaseURL(t *testing.T) {
	session := &AuthSession{Client: http.DefaultClient}
	client := NewCIClient(session)
	if !strings.HasSuffix(client.baseURL, "/ci/api") {
		t.Fatalf("expected base URL ending in /ci/api, got %q", client.baseURL)
	}
}

func TestListCIWorkflowsParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/products/prod-1/workflows-v15") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("limit") != "100" {
			t.Fatalf("expected limit=100, got %q", r.URL.Query().Get("limit"))
		}
		if r.URL.Query().Get("include_deleted") != "false" {
			t.Fatalf("expected include_deleted=false, got %q", r.URL.Query().Get("include_deleted"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items": [
				{"id":"wf-1","content":{"name":"TestFlight Deploy","description":"Build on main"}},
				{"id":"wf-2","content":{"name":"PR Check"}}
			]
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	result, err := client.ListCIWorkflows(context.Background(), "team-uuid", "prod-1")
	if err != nil {
		t.Fatalf("ListCIWorkflows() error = %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 workflows, got %d", len(result.Items))
	}
	if result.Items[0].ID != "wf-1" || result.Items[0].Content.Name != "TestFlight Deploy" {
		t.Fatalf("unexpected first workflow: %+v", result.Items[0])
	}
	if result.Items[1].Content.Name != "PR Check" {
		t.Fatalf("unexpected second workflow name: %q", result.Items[1].Content.Name)
	}
}

func TestListCIWorkflowsRejectsEmptyInputs(t *testing.T) {
	client := &Client{httpClient: http.DefaultClient, baseURL: "http://localhost"}
	tests := []struct {
		name      string
		teamID    string
		productID string
		wantErr   string
	}{
		{"empty team", "", "prod", "team id is required"},
		{"empty product", "team", "", "product id is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.ListCIWorkflows(context.Background(), tt.teamID, tt.productID)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}
