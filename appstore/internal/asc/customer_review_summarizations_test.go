package asc

import (
	"context"
	"net/http"
	"testing"
)

func TestGetCustomerReviewSummarizations(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/customerReviewSummarizations" {
			t.Fatalf("expected path /v1/apps/app-1/customerReviewSummarizations, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if got := values.Get("filter[platform]"); got != "IOS" {
			t.Fatalf("expected filter[platform]=IOS, got %q", got)
		}
		if got := values.Get("filter[territory]"); got != "US" {
			t.Fatalf("expected filter[territory]=US, got %q", got)
		}
		if got := values.Get("fields[customerReviewSummarizations]"); got != "locale,text" {
			t.Fatalf("expected fields[customerReviewSummarizations]=locale,text, got %q", got)
		}
		if got := values.Get("fields[territories]"); got != "currency" {
			t.Fatalf("expected fields[territories]=currency, got %q", got)
		}
		if got := values.Get("include"); got != "territory" {
			t.Fatalf("expected include=territory, got %q", got)
		}
		if got := values.Get("limit"); got != "25" {
			t.Fatalf("expected limit=25, got %q", got)
		}
		assertAuthorized(t, req)
	}, response)

	_, err := client.GetCustomerReviewSummarizations(
		context.Background(), "app-1",
		WithCustomerReviewSummarizationsPlatforms([]string{"IOS"}),
		WithCustomerReviewSummarizationsTerritories([]string{"US"}),
		WithCustomerReviewSummarizationsFields([]string{"locale", "text"}),
		WithCustomerReviewSummarizationsTerritoryFields([]string{"currency"}),
		WithCustomerReviewSummarizationsInclude([]string{"territory"}),
		WithCustomerReviewSummarizationsLimit(25),
	)
	if err != nil {
		t.Fatalf("GetCustomerReviewSummarizations() error: %v", err)
	}
}

func TestGetCustomerReviewSummarizations_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/apps/app-1/customerReviewSummarizations?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.String() != next {
			t.Fatalf("expected URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCustomerReviewSummarizations(context.Background(), "app-1", WithCustomerReviewSummarizationsNextURL(next)); err != nil {
		t.Fatalf("GetCustomerReviewSummarizations() error: %v", err)
	}
}
