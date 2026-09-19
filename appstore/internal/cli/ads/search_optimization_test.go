package ads

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/appleads"
)

type searchOptimizationRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn searchOptimizationRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestFetchSearchOptimizationDataUsesOfficialEndpointsAndPreservesPartialFailures(t *testing.T) {
	var mu sync.Mutex
	requests := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s for %s, want POST", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-AP-Context"); got != "adAccountId=account-1;" {
			t.Errorf("X-AP-Context = %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode %s body: %v", r.URL.Path, err)
		}
		mu.Lock()
		requests[r.URL.Path]++
		mu.Unlock()
		assertOptimizationPaginationShape(t, r.URL.Path, body)

		switch r.URL.Path {
		case "/v1/campaigns/query":
			assertFilter(t, body, "promotedObjectType", "APPSTORE_APP")
			assertFilter(t, body, "promotedObjectId", "123456789")
			writeJSON(t, w, `{"result":[{"id":44,"name":"US Search","status":"ENABLED","displayStatus":"RUNNING","promotedObjectId":"123456789","targeting":{"countryOrRegion":{"include":["US"]}}}],"pagination":{"offset":0,"pageSize":1000,"totalCount":1}}`)
		case "/v1/suggestions/keywords/query":
			assertFilter(t, body, "promotedObjectId", "123456789")
			assertFilter(t, body, "promotedObjectType", "APPSTORE_APP")
			assertFilter(t, body, "countriesOrRegions", "US")
			writeJSON(t, w, `{"result":[{"text":"habit tracker","popularity":80}],"pagination":{"offset":0,"pageSize":1000,"totalCount":1}}`)
		case "/v1/suggestions/phrases/query":
			if hasOptimizationFilter(body, "countriesOrRegions") {
				t.Errorf("phrase suggestion request unexpectedly includes countriesOrRegions: %#v", body)
			}
			assertFilter(t, body, "queryType", "SUGGESTION")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"code":"INVALID_VALUE","message":"One or more validation errors occurred.","details":[{"code":"INVALID_FILTER","message":"Unsupported filter value","info":{"field":"filters[2].value"}}]}`))
		case "/v1/suggestions/target-cpas/query":
			assertFilter(t, body, "promotedObjectId", "123456789")
			assertFilter(t, body, "promotedObjectType", "APPSTORE_APP")
			assertFilter(t, body, "countryOrRegion", "US")
			writeJSON(t, w, `{"result":{"promotedObjectId":"123456789","suggestedTargetCPA":{"amount":"1.20","currency":"USD"},"countryOrRegion":["US"],"appCategory":"Productivity"}}`)
		case "/v1/insights/apps/search-term-popularity/query":
			assertFilter(t, body, "countryOrRegion", "US")
			assertFilter(t, body, "genre", "PRODUCTIVITY_UTILITIES")
			assertNestedValue(t, body, "timeRange", "start", "2026-08-09")
			assertNestedValue(t, body, "timeRange", "end", "2026-08-15")
			assertNestedValue(t, body, "timeRange", "timeZone", "UTC")
			assertNestedValue(t, body, "timeRange", "granularity", "WEEKLY_SUN_SAT")
			assertSorting(t, body, "rankInGenre", "sortOrder", "ASC")
			writeJSON(t, w, `{"result":{"rows":[{"week":"2026-08-09","countryOrRegion":"US","genre":"PRODUCTIVITY_UTILITIES","searchTerm":"habit tracker","rankInGenre":2,"searchPopularity1to100":88,"searchPopularity1to5":5}]},"pagination":{"offset":0,"pageSize":5000,"totalCount":1}}`)
		case "/v1/insights/apps/impression-share/query":
			assertFilter(t, body, "promotedObjectId", "123456789")
			assertFilter(t, body, "countryOrRegion", "US")
			assertNestedValue(t, body, "options", "impressionShareReportType", "ALL_SLOTS")
			assertNestedValue(t, body, "timeRange", "timeZone", "UTC")
			assertNestedValue(t, body, "timeRange", "granularity", "DAILY")
			writeJSON(t, w, `{"result":{"rows":[{"day":"2026-08-17","promotedObjectId":"123456789","countryOrRegion":"US","searchTerm":"habit tracker","lowImpressionShare":0.07,"highImpressionShare":0.07,"rank":6,"searchPopularity1to5":4}]},"pagination":{"offset":0,"pageSize":5000,"totalCount":1}}`)
		case "/v1/eligibilities/apps/query":
			assertFilter(t, body, "adamId", float64(123456789))
			writeJSON(t, w, `{"result":[{"adamId":123456789,"supplyPlacement":"APPSTORE_SEARCH_RESULTS","supplySource":"APPSTORE","state":"ELIGIBLE","countryOrRegion":"US","deviceClass":"IPHONE"}],"pagination":{"offset":0,"pageSize":1000,"totalCount":1}}`)
		case "/v1/recommendations/daily-budgets/query":
			assertFilter(t, body, "promotedObjectId", "123456789")
			assertFilter(t, body, "promotedObjectType", "APPSTORE_APP")
			assertFilter(t, body, "state", "AVAILABLE")
			writeJSON(t, w, `{"result":[{"id":"budget-1","campaignId":44,"state":"AVAILABLE"}],"pagination":{"offset":0,"pageSize":1000,"totalCount":1}}`)
		case "/v1/recommendations/target-cpas/query":
			assertFilter(t, body, "promotedObjectId", "123456789")
			assertFilter(t, body, "promotedObjectType", "APPSTORE_APP")
			assertFilter(t, body, "state", "AVAILABLE")
			writeJSON(t, w, `{"result":[],"pagination":{"offset":0,"pageSize":1000,"totalCount":0}}`)
		case "/v1/keywords/query":
			assertFilter(t, body, "campaignId", float64(44))
			writeJSON(t, w, `{"result":[{"id":88,"campaignId":44,"adGroupId":55,"text":"habits","matchType":"BROAD","status":"ENABLED"}],"pagination":{"offset":0,"pageSize":1000,"totalCount":1}}`)
		case "/v1/negative-keywords/query":
			writeJSON(t, w, `{"result":[],"pagination":{"offset":0,"pageSize":1000,"totalCount":0}}`)
		case "/v1/reports/apps/searchterms/query":
			assertFilter(t, body, "campaignId", float64(44))
			assertNestedValue(t, body, "timeRange", "start", "2026-07-19")
			assertNestedValue(t, body, "timeRange", "end", "2026-08-17")
			assertNestedValue(t, body, "timeRange", "timeZone", "ORTZ")
			assertNestedValue(t, body, "timeRange", "granularity", "DAILY")
			assertArrayContains(t, body, "fields", "totalInstalls")
			assertArrayContains(t, body, "groupBy", "countryOrRegion")
			writeJSON(t, w, `{"result":{"rows":[{"metadata":{"searchTermText":"habit tracker","keyword":{"id":88,"text":"habits","matchType":"BROAD"},"campaignId":44,"adGroupId":55,"countryOrRegion":"US"},"totalMetrics":{"localSpend":{"amount":"36.58","currency":"USD"},"impressions":1000,"taps":70,"tapInstalls":28,"totalInstalls":31}}]},"pagination":{"offset":0,"pageSize":5000,"totalCount":1}}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := appleads.NewClient(
		appleads.Credentials{AccessToken: "token", AdAccountID: "account-1"},
		appleads.WithPlatformBaseURL(server.URL+"/v1/"),
	)
	if err != nil {
		t.Fatal(err)
	}

	data, err := fetchSearchOptimizationData(context.Background(), client, SearchOptimizationRequest{
		AppID:           "123456789",
		Country:         "US",
		Genre:           "PRODUCTIVITY_UTILITIES",
		Start:           "2026-07-19",
		End:             "2026-08-17",
		PopularityStart: "2026-08-09",
		PopularityEnd:   "2026-08-15",
	})
	if err != nil {
		t.Fatalf("fetchSearchOptimizationData() error = %v", err)
	}
	if len(data.Suggestions) != 1 || data.Suggestions[0].Text != "habit tracker" {
		t.Fatalf("suggestions = %+v", data.Suggestions)
	}
	if len(data.Popularities) != 1 || data.Popularities[0].Term != "habit tracker" || data.Popularities[0].Popularity5 == nil || *data.Popularities[0].Popularity5 != 5 {
		t.Fatalf("popularities = %+v", data.Popularities)
	}
	if len(data.SearchTerms) != 1 || data.SearchTerms[0].TotalInstalls != 31 || data.SearchTerms[0].AdGroupID != 55 {
		t.Fatalf("search terms = %+v", data.SearchTerms)
	}
	if data.DailyBudgetRecommendations != 1 || data.TargetCPARecommendations != 0 {
		t.Fatalf("recommendations = (%d, %d)", data.DailyBudgetRecommendations, data.TargetCPARecommendations)
	}
	if len(data.TargetCPASuggestion) == 0 || !strings.Contains(string(data.TargetCPASuggestion), `"amount":"1.20"`) {
		t.Fatalf("target CPA suggestion = %s", data.TargetCPASuggestion)
	}
	if len(data.DailyBudgetRecommendationItems) != 1 || !strings.Contains(string(data.DailyBudgetRecommendationItems[0]), "budget-1") {
		t.Fatalf("daily budget recommendation items = %s", data.DailyBudgetRecommendationItems)
	}
	phraseSource := findOptimizationSource(t, data.Sources, "phrase_suggestions")
	if phraseSource.Status != "unavailable" || !strings.Contains(phraseSource.Error, "INVALID_FILTER") || !strings.Contains(phraseSource.Error, `"field":"filters[2].value"`) {
		t.Fatalf("phrase source = %+v", phraseSource)
	}
	if requests["/v1/negative-keywords/query"] != 2 {
		t.Fatalf("negative keyword requests = %d, want campaign and ad-group scopes", requests["/v1/negative-keywords/query"])
	}
}

func TestFetchOptimizationPopularityDoesNotRequireAppScope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/insights/apps/search-term-popularity/query" {
			t.Errorf("path = %q", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode popularity request: %v", err)
			http.Error(w, "invalid test request", http.StatusBadRequest)
			return
		}
		if hasOptimizationFilter(body, "promotedObjectId") {
			t.Errorf("popularity request unexpectedly contains app scope: %#v", body)
		}
		assertFilter(t, body, "countryOrRegion", "US")
		assertFilter(t, body, "genre", "PRODUCTIVITY_UTILITIES")
		writeJSON(t, w, `{"result":{"rows":[{"week":"2026-08-09","countryOrRegion":"US","genre":"PRODUCTIVITY_UTILITIES","searchTerm":"habit tracker","searchPopularity1to5":5}]},"pagination":{"offset":0,"pageSize":5000,"totalCount":1}}`)
	}))
	defer server.Close()

	client, err := appleads.NewClient(
		appleads.Credentials{AccessToken: "token", AdAccountID: "account-1"},
		appleads.WithPlatformBaseURL(server.URL+"/v1/"),
	)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := fetchOptimizationPopularity(context.Background(), client, SearchOptimizationRequest{
		Country:         "US",
		Genre:           "PRODUCTIVITY_UTILITIES",
		PopularityStart: "2026-08-09",
		PopularityEnd:   "2026-08-15",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Term != "habit tracker" {
		t.Fatalf("popularity rows = %+v", rows)
	}
}

func TestQueryOptimizationListPaginatesRequestBody(t *testing.T) {
	var offsets []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Pagination map[string]any `json:"pagination"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode pagination request: %v", err)
			http.Error(w, "invalid test request", http.StatusBadRequest)
			return
		}
		if _, present := body.Pagination["fetchTotalCount"]; present {
			t.Errorf("recommendation-style pagination contains unsupported fetchTotalCount: %#v", body.Pagination)
		}
		offset, _ := body.Pagination["offset"].(float64)
		offsets = append(offsets, int(offset))
		if offset == 0 {
			writeJSON(t, w, `{"result":[{"text":"one","popularity":1}],"pagination":{"offset":0,"pageSize":1,"totalCount":2}}`)
			return
		}
		writeJSON(t, w, `{"result":[{"text":"two","popularity":2}],"pagination":{"offset":1,"pageSize":1,"totalCount":2}}`)
	}))
	defer server.Close()
	client, err := appleads.NewClient(appleads.Credentials{AccessToken: "token", AdAccountID: "account-1"}, appleads.WithPlatformBaseURL(server.URL+"/v1/"))
	if err != nil {
		t.Fatal(err)
	}
	spec, ok := appleads.PlatformEndpointByCommandPath("suggestions", "keywords", "find")
	if !ok {
		t.Fatal("missing endpoint spec")
	}
	items, err := queryOptimizationList[SearchSuggestion](context.Background(), client, spec, map[string]any{}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || len(offsets) != 2 || offsets[0] != 0 || offsets[1] != 1 {
		t.Fatalf("items=%+v offsets=%v", items, offsets)
	}
}

func TestQueryOptimizationListCapsUnboundedPages(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		writeJSON(t, w, `{"result":[{"text":"one","popularity":1}],"pagination":{"offset":0,"pageSize":1}}`)
	}))
	defer server.Close()
	client, err := appleads.NewClient(appleads.Credentials{AccessToken: "token", AdAccountID: "account-1"}, appleads.WithPlatformBaseURL(server.URL+"/v1/"))
	if err != nil {
		t.Fatal(err)
	}
	spec, ok := appleads.PlatformEndpointByCommandPath("suggestions", "keywords", "find")
	if !ok {
		t.Fatal("missing endpoint spec")
	}

	items, err := queryOptimizationList[SearchSuggestion](context.Background(), client, spec, map[string]any{}, 1)
	if err == nil {
		t.Fatal("queryOptimizationList() unexpectedly succeeded for an unbounded result")
	}
	if got, want := requests, appleads.MaxPlatformPaginationPages; got != want {
		t.Fatalf("request count = %d, want %d", got, want)
	}
	if got, want := len(items), appleads.MaxPlatformPaginationPages; got != want {
		t.Fatalf("accumulated items = %d, want the %d fetched pages returned alongside the error", got, want)
	}
	for _, want := range []string{"1000-page safety limit", "suggestions keywords find", "narrow"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want it to contain %q", err, want)
		}
	}
}

func TestQueryOptimizationRowsCapsUnboundedPages(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		writeJSON(t, w, `{"result":{"rows":[{"searchTerm":"habit tracker"}]},"pagination":{"offset":0,"pageSize":1}}`)
	}))
	defer server.Close()
	client, err := appleads.NewClient(appleads.Credentials{AccessToken: "token", AdAccountID: "account-1"}, appleads.WithPlatformBaseURL(server.URL+"/v1/"))
	if err != nil {
		t.Fatal(err)
	}
	spec, ok := appleads.PlatformEndpointByCommandPath("insights", "search-term-popularity", "find")
	if !ok {
		t.Fatal("missing endpoint spec")
	}

	items, err := queryOptimizationRows[popularityResponse](context.Background(), client, spec, map[string]any{}, 1)
	if err == nil {
		t.Fatal("queryOptimizationRows() unexpectedly succeeded for an unbounded result")
	}
	if got, want := requests, appleads.MaxPlatformPaginationPages; got != want {
		t.Fatalf("request count = %d, want %d", got, want)
	}
	if got, want := len(items), appleads.MaxPlatformPaginationPages; got != want {
		t.Fatalf("accumulated rows = %d, want the %d fetched pages returned alongside the error", got, want)
	}
	for _, want := range []string{"1000-page safety limit", "insights search-term-popularity find", "narrow"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want it to contain %q", err, want)
		}
	}
}

func TestFetchOptimizationPhraseSuggestionsReadsPhraseField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode phrase-suggestion request: %v", err)
			http.Error(w, "invalid test request", http.StatusBadRequest)
			return
		}
		if hasOptimizationFilter(body, "countriesOrRegions") {
			t.Errorf("phrase suggestion request unexpectedly includes countriesOrRegions: %#v", body)
		}
		writeJSON(t, w, `{"result":[{"phrase":"best habit tracker","popularity":82}],"pagination":{"offset":0,"pageSize":1000,"totalCount":1}}`)
	}))
	defer server.Close()
	client, err := appleads.NewClient(
		appleads.Credentials{AccessToken: "token", AdAccountID: "account-1"},
		appleads.WithPlatformBaseURL(server.URL+"/v1/"),
	)
	if err != nil {
		t.Fatal(err)
	}

	items, err := fetchOptimizationSuggestions(context.Background(), client, SearchOptimizationRequest{
		AppID: "123456789", Country: "US",
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Text != "best habit tracker" || items[0].Kind != "phrase" || items[0].Popularity == nil || *items[0].Popularity != 82 {
		t.Fatalf("phrase suggestions = %+v", items)
	}
}

func TestFetchSearchSuggestionsOmitsSortingAndHonorsLimit(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path)
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode suggestion request: %v", err)
			http.Error(w, "invalid test request", http.StatusBadRequest)
			return
		}
		if _, hasSorting := body["sorting"]; hasSorting {
			t.Errorf("sorting = %#v, want omitted (Apple rejects popularity sort on suggestions)", body["sorting"])
		}
		pagination, ok := body["pagination"].(map[string]any)
		if !ok || pagination["offset"] != float64(0) || pagination["pageSize"] != float64(2) {
			t.Errorf("pagination = %#v, want offset 0/pageSize 2", body["pagination"])
		}
		if r.URL.Path == "/v1/suggestions/keywords/query" {
			writeJSON(t, w, `{"result":[{"text":"keyword one","popularity":90},{"text":"keyword two","popularity":80}],"pagination":{"offset":0,"pageSize":2,"totalCount":10}}`)
			return
		}
		if r.URL.Path == "/v1/suggestions/phrases/query" {
			writeJSON(t, w, `{"result":[{"phrase":"phrase one","popularity":70},{"phrase":"phrase two","popularity":60}],"pagination":{"offset":0,"pageSize":2,"totalCount":10}}`)
			return
		}
		t.Errorf("unexpected path %s", r.URL.Path)
		http.NotFound(w, r)
	}))
	defer server.Close()
	client, err := appleads.NewClient(
		appleads.Credentials{AccessToken: "token", AdAccountID: "account-1"},
		appleads.WithPlatformBaseURL(server.URL+"/v1/"),
	)
	if err != nil {
		t.Fatal(err)
	}

	data, err := fetchSearchSuggestions(context.Background(), client, SearchOptimizationRequest{
		AppID: "123456789", Country: "US", Limit: 2,
	})
	if err != nil {
		t.Fatalf("fetchSearchSuggestions() error = %v", err)
	}
	if len(requests) != 2 || requests[0] != "/v1/suggestions/keywords/query" || requests[1] != "/v1/suggestions/phrases/query" {
		t.Fatalf("suggestion requests = %v, want one keyword and one phrase request", requests)
	}
	if len(data.Suggestions) != 4 {
		t.Fatalf("suggestions = %+v, want two results from each endpoint", data.Suggestions)
	}
	if !data.SuggestionsTruncated {
		t.Fatal("suggestions should report that both endpoints have more results beyond the bounded prefix")
	}
}

func TestFetchSearchSuggestionsDetectsMoreWhenTotalCountIsOmitted(t *testing.T) {
	requests := make(map[string][][2]int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode suggestion request: %v", err)
			http.Error(w, "invalid test request", http.StatusBadRequest)
			return
		}
		pagination, ok := body["pagination"].(map[string]any)
		if !ok {
			t.Errorf("pagination = %#v, want an object", body["pagination"])
			http.Error(w, "missing pagination", http.StatusBadRequest)
			return
		}
		offset := int(pagination["offset"].(float64))
		pageSize := int(pagination["pageSize"].(float64))
		requests[r.URL.Path] = append(requests[r.URL.Path], [2]int{offset, pageSize})
		if offset == 0 {
			if r.URL.Path == "/v1/suggestions/keywords/query" {
				writeJSON(t, w, `{"result":[{"text":"keyword one","popularity":90},{"text":"keyword two","popularity":80}],"pagination":{"offset":0,"pageSize":2}}`)
				return
			}
			writeJSON(t, w, `{"result":[{"phrase":"phrase one","popularity":70},{"phrase":"phrase two","popularity":60}],"pagination":{"offset":0,"pageSize":2}}`)
			return
		}
		if offset == 2 && pageSize == 1 {
			if r.URL.Path == "/v1/suggestions/keywords/query" {
				writeJSON(t, w, `{"result":[{"text":"keyword three","popularity":50}],"pagination":{"offset":2,"pageSize":1}}`)
				return
			}
			writeJSON(t, w, `{"result":[{"phrase":"phrase three","popularity":40}],"pagination":{"offset":2,"pageSize":1}}`)
			return
		}
		t.Errorf("unexpected %s pagination offset=%d pageSize=%d", r.URL.Path, offset, pageSize)
		http.Error(w, "unexpected pagination", http.StatusBadRequest)
	}))
	defer server.Close()
	client, err := appleads.NewClient(
		appleads.Credentials{AccessToken: "token", AdAccountID: "account-1"},
		appleads.WithPlatformBaseURL(server.URL+"/v1/"),
	)
	if err != nil {
		t.Fatal(err)
	}

	data, err := fetchSearchSuggestions(context.Background(), client, SearchOptimizationRequest{
		AppID: "123456789", Country: "US", Limit: 2,
	})
	if err != nil {
		t.Fatalf("fetchSearchSuggestions() error = %v", err)
	}
	if !data.SuggestionsTruncated {
		t.Fatal("suggestions should report more results when the probe finds another record")
	}
	if len(data.Suggestions) != 4 {
		t.Fatalf("suggestions = %+v, want bounded first pages only", data.Suggestions)
	}
	for _, path := range []string{"/v1/suggestions/keywords/query", "/v1/suggestions/phrases/query"} {
		if got := requests[path]; len(got) != 2 || got[0] != [2]int{0, 2} || got[1] != [2]int{2, 1} {
			t.Fatalf("%s requests = %v, want initial page and one-record probe", path, got)
		}
	}
}

func TestFetchSearchSuggestionsPreservesPrefixWhenTruncationProbeFails(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Pagination struct {
				Offset   int `json:"offset"`
				PageSize int `json:"pageSize"`
			} `json:"pagination"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode suggestion request: %v", err)
			http.Error(w, "invalid test request", http.StatusBadRequest)
			return
		}
		requests = append(requests, fmt.Sprintf("%s offset=%d pageSize=%d", r.URL.Path, body.Pagination.Offset, body.Pagination.PageSize))

		switch {
		case r.URL.Path == "/v1/suggestions/keywords/query" && body.Pagination.Offset == 0:
			writeJSON(t, w, `{"result":[{"text":"keyword one","popularity":90},{"text":"keyword two","popularity":80}],"pagination":{"offset":0,"pageSize":2}}`)
		case r.URL.Path == "/v1/suggestions/keywords/query" && body.Pagination.Offset == 2:
			http.Error(w, "probe unavailable", http.StatusBadGateway)
		case r.URL.Path == "/v1/suggestions/phrases/query":
			writeJSON(t, w, `{"result":[{"phrase":"phrase one","popularity":70}],"pagination":{"offset":0,"pageSize":2,"totalCount":1}}`)
		default:
			t.Errorf("unexpected suggestion request: %s", requests[len(requests)-1])
			http.Error(w, "unexpected pagination", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	client, err := appleads.NewClient(
		appleads.Credentials{AccessToken: "token", AdAccountID: "account-1"},
		appleads.WithPlatformBaseURL(server.URL+"/v1/"),
	)
	if err != nil {
		t.Fatal(err)
	}

	data, err := fetchSearchSuggestions(context.Background(), client, SearchOptimizationRequest{
		AppID: "123456789", Country: "US", Limit: 2,
	})
	if err != nil {
		t.Fatalf("fetchSearchSuggestions() error = %v", err)
	}
	if len(data.Suggestions) != 3 {
		t.Fatalf("suggestions = %+v, want the bounded prefix plus the phrase result", data.Suggestions)
	}
	if !data.SuggestionsTruncated {
		t.Fatal("suggestions should be conservatively marked truncated when the probe fails")
	}
	keywordSource := findOptimizationSource(t, data.Sources, "keyword_suggestions")
	if keywordSource.Status != "unavailable" || keywordSource.Count != 2 {
		t.Fatalf("keyword source = %+v, want unavailable with preserved prefix count", keywordSource)
	}
	if !strings.Contains(keywordSource.Error, "pagination truncation probe") || !strings.Contains(keywordSource.Error, "probe unavailable") {
		t.Fatalf("keyword source diagnostic = %q, want the probe failure", keywordSource.Error)
	}
}

func TestFetchSearchSuggestionsMarksPrefixTruncatedWhenPaginationFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Pagination struct {
				Offset   int `json:"offset"`
				PageSize int `json:"pageSize"`
			} `json:"pagination"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode suggestion request: %v", err)
			http.Error(w, "invalid test request", http.StatusBadRequest)
			return
		}
		if r.URL.Path == "/v1/suggestions/phrases/query" {
			writeJSON(t, w, `{"result":[],"pagination":{"offset":0,"pageSize":1000,"totalCount":0}}`)
			return
		}
		if body.Pagination.Offset == 1000 {
			http.Error(w, "page unavailable", http.StatusBadGateway)
			return
		}
		result := make([]map[string]any, 1000)
		for index := range result {
			result[index] = map[string]any{"text": fmt.Sprintf("keyword-%d", index), "popularity": 1000 - index}
		}
		payload := map[string]any{
			"result": result,
			"pagination": map[string]any{
				"offset":     body.Pagination.Offset,
				"pageSize":   body.Pagination.PageSize,
				"totalCount": 1800,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			t.Errorf("encode suggestion response: %v", err)
		}
	}))
	defer server.Close()
	client, err := appleads.NewClient(
		appleads.Credentials{AccessToken: "token", AdAccountID: "account-1"},
		appleads.WithPlatformBaseURL(server.URL+"/v1/"),
	)
	if err != nil {
		t.Fatal(err)
	}

	data, err := fetchSearchSuggestions(context.Background(), client, SearchOptimizationRequest{
		AppID: "123456789", Country: "US", Limit: 1500,
	})
	if err != nil {
		t.Fatalf("fetchSearchSuggestions() error = %v", err)
	}
	if len(data.Suggestions) != 1000 {
		t.Fatalf("suggestions = %d, want the successfully fetched prefix", len(data.Suggestions))
	}
	if !data.SuggestionsTruncated {
		t.Fatal("suggestions should be conservatively marked truncated after a later page fails")
	}
	keywordSource := findOptimizationSource(t, data.Sources, "keyword_suggestions")
	if keywordSource.Status != "unavailable" || keywordSource.Count != 1000 {
		t.Fatalf("keyword source = %+v, want unavailable with the preserved prefix", keywordSource)
	}
}

func TestFetchSearchSuggestionsMarksOvershotBoundedPageAsTruncated(t *testing.T) {
	requests := make(map[string][][2]int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Pagination struct {
				Offset   int `json:"offset"`
				PageSize int `json:"pageSize"`
			} `json:"pagination"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode suggestion request: %v", err)
			http.Error(w, "invalid test request", http.StatusBadRequest)
			return
		}
		requests[r.URL.Path] = append(requests[r.URL.Path], [2]int{body.Pagination.Offset, body.Pagination.PageSize})

		count := 0
		switch {
		case r.URL.Path == "/v1/suggestions/keywords/query" && body.Pagination.Offset == 0:
			count = 1000
		case r.URL.Path == "/v1/suggestions/keywords/query" && body.Pagination.Offset == 1000:
			count = 800
		case r.URL.Path == "/v1/suggestions/phrases/query":
			writeJSON(t, w, `{"result":[],"pagination":{"offset":0,"pageSize":1000,"totalCount":0}}`)
			return
		default:
			t.Errorf("unexpected %s pagination %+v", r.URL.Path, body.Pagination)
			http.Error(w, "unexpected pagination", http.StatusBadRequest)
			return
		}

		result := make([]map[string]any, count)
		for index := range result {
			result[index] = map[string]any{
				"text":       fmt.Sprintf("keyword-%d", body.Pagination.Offset+index),
				"popularity": count - index,
			}
		}
		payload := map[string]any{
			"result": result,
			"pagination": map[string]any{
				"offset":     body.Pagination.Offset,
				"pageSize":   body.Pagination.PageSize,
				"totalCount": 1800,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			t.Errorf("encode suggestion response: %v", err)
		}
	}))
	defer server.Close()
	client, err := appleads.NewClient(
		appleads.Credentials{AccessToken: "token", AdAccountID: "account-1"},
		appleads.WithPlatformBaseURL(server.URL+"/v1/"),
	)
	if err != nil {
		t.Fatal(err)
	}

	data, err := fetchSearchSuggestions(context.Background(), client, SearchOptimizationRequest{
		AppID: "123456789", Country: "US", Limit: 1500,
	})
	if err != nil {
		t.Fatalf("fetchSearchSuggestions() error = %v", err)
	}
	if !data.SuggestionsTruncated {
		t.Fatal("suggestions should report truncation when an overshot page contains results beyond the limit")
	}
	if len(data.Suggestions) != 1500 {
		t.Fatalf("suggestions = %d, want the requested limit", len(data.Suggestions))
	}
	if got := requests["/v1/suggestions/keywords/query"]; len(got) != 2 || got[0] != [2]int{0, 1000} || got[1] != [2]int{1000, 1000} {
		t.Fatalf("keyword requests = %v, want two bounded pages", got)
	}
}

func hasOptimizationFilter(body map[string]any, field string) bool {
	filters, _ := body["filters"].([]any)
	for _, raw := range filters {
		filter, _ := raw.(map[string]any)
		if filter["field"] == field {
			return true
		}
	}
	return false
}

func TestExecuteOptimizationQueryAppliesRequestTimeout(t *testing.T) {
	deadlinePresent := false
	client, err := appleads.NewClient(
		appleads.Credentials{AccessToken: "token", AdAccountID: "account-1"},
		appleads.WithHTTPClient(&http.Client{Transport: searchOptimizationRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			_, deadlinePresent = request.Context().Deadline()
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"result":[],"pagination":{"totalCount":0}}`)),
			}, nil
		})}),
	)
	if err != nil {
		t.Fatal(err)
	}
	spec, ok := appleads.PlatformEndpointByCommandPath("suggestions", "keywords", "find")
	if !ok {
		t.Fatal("missing endpoint spec")
	}
	if _, err := executeOptimizationQuery(context.Background(), client, spec, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if !deadlinePresent {
		t.Fatal("optimization request context has no deadline")
	}
}

func TestFetchSearchOptimizationDataFailsWhenEveryIntelligenceSourceIsUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"errors":[{"message":"unavailable"}]}`, http.StatusBadRequest)
	}))
	defer server.Close()
	client, err := appleads.NewClient(
		appleads.Credentials{AccessToken: "token", AdAccountID: "account-1"},
		appleads.WithPlatformBaseURL(server.URL+"/v1/"),
	)
	if err != nil {
		t.Fatal(err)
	}

	data, err := fetchSearchOptimizationData(context.Background(), client, SearchOptimizationRequest{
		AppID: "123456789", Country: "US", Genre: "PRODUCTIVITY_UTILITIES",
		Start: "2026-07-19", End: "2026-08-17", PopularityStart: "2026-08-09", PopularityEnd: "2026-08-15",
	})
	if err == nil || !strings.Contains(err.Error(), "all official Apple Ads optimization sources are unavailable") {
		t.Fatalf("error = %v", err)
	}
	if source := findOptimizationSource(t, data.Sources, "search_term_performance"); source.Status != "unavailable" || !strings.Contains(source.Error, "campaign scope unavailable") {
		t.Fatalf("search term source = %+v", source)
	}
}

func assertFilter(t *testing.T, body map[string]any, field string, want any) {
	t.Helper()
	filters, _ := body["filters"].([]any)
	for _, raw := range filters {
		filter, _ := raw.(map[string]any)
		if filter["field"] != field {
			continue
		}
		value := filter["value"]
		switch typed := value.(type) {
		case []any:
			for _, item := range typed {
				if item == want {
					return
				}
			}
		default:
			if typed == want {
				return
			}
		}
	}
	t.Errorf("body filters do not contain %s=%v: %#v", field, want, body)
}

func assertOptimizationPaginationShape(t *testing.T, path string, body map[string]any) {
	t.Helper()
	if path == "/v1/suggestions/target-cpas/query" {
		if _, present := body["pagination"]; present {
			t.Errorf("%s body has unnecessary pagination: %#v", path, body)
		}
		return
	}
	pagination, ok := body["pagination"].(map[string]any)
	if !ok {
		t.Errorf("%s body has no pagination object: %#v", path, body)
		return
	}
	if _, ok := pagination["offset"]; !ok {
		t.Errorf("%s pagination has no offset: %#v", path, pagination)
	}
	if _, ok := pagination["pageSize"]; !ok {
		t.Errorf("%s pagination has no pageSize: %#v", path, pagination)
	}
	_, hasFetchTotalCount := pagination["fetchTotalCount"]
	expectsFetchTotalCount := path == "/v1/campaigns/query" || path == "/v1/keywords/query" || path == "/v1/negative-keywords/query"
	if hasFetchTotalCount != expectsFetchTotalCount {
		t.Errorf("%s pagination fetchTotalCount present = %t, want %t: %#v", path, hasFetchTotalCount, expectsFetchTotalCount, pagination)
	}
}

func assertNestedValue(t *testing.T, body map[string]any, object, field string, want any) {
	t.Helper()
	nested, ok := body[object].(map[string]any)
	if !ok || nested[field] != want {
		t.Errorf("%s.%s = %#v, want %#v in body %#v", object, field, nested[field], want, body)
	}
}

func assertSorting(t *testing.T, body map[string]any, field, orderKey, order string) {
	t.Helper()
	sorting, ok := body["sorting"].([]any)
	if !ok || len(sorting) != 1 {
		t.Errorf("sorting = %#v, want one entry", body["sorting"])
		return
	}
	entry, ok := sorting[0].(map[string]any)
	if !ok || entry["field"] != field || entry[orderKey] != order {
		t.Errorf("sorting = %#v, want %s %s=%s", sorting, field, orderKey, order)
	}
}

func assertArrayContains(t *testing.T, body map[string]any, field, want string) {
	t.Helper()
	items, ok := body[field].([]any)
	if !ok {
		t.Errorf("%s = %#v, want array containing %q", field, body[field], want)
		return
	}
	for _, item := range items {
		if item == want {
			return
		}
	}
	t.Errorf("%s = %#v, want array containing %q", field, items, want)
}

func writeJSON(t *testing.T, w http.ResponseWriter, payload string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(payload))
}

func findOptimizationSource(t *testing.T, sources []SearchOptimizationSourceStatus, name string) SearchOptimizationSourceStatus {
	t.Helper()
	for _, source := range sources {
		if source.Name == name {
			return source
		}
	}
	t.Fatalf("missing source %q in %+v", name, sources)
	return SearchOptimizationSourceStatus{}
}
