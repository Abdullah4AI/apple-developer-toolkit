package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/handlertest"
)

func TestNormalizeAppAvailabilityCreateAttributes(t *testing.T) {
	t.Run("normalizes and deduplicates territory ids", func(t *testing.T) {
		attrs, err := normalizeAppAvailabilityCreateAttributes(AppAvailabilityCreateAttributes{
			AppID:                " app-123 ",
			AvailableTerritories: []string{" usa ", "gbr", "USA", ""},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if attrs.AppID != "app-123" {
			t.Fatalf("expected trimmed app id, got %q", attrs.AppID)
		}
		if got := strings.Join(attrs.AvailableTerritories, ","); got != "GBR,USA" {
			t.Fatalf("expected normalized territories GBR,USA, got %q", got)
		}
	})

	t.Run("requires app id", func(t *testing.T) {
		_, err := normalizeAppAvailabilityCreateAttributes(AppAvailabilityCreateAttributes{
			AvailableTerritories: []string{"USA"},
		})
		if err == nil || !strings.Contains(err.Error(), "app id is required") {
			t.Fatalf("expected missing app id error, got %v", err)
		}
	})

	t.Run("requires at least one territory", func(t *testing.T) {
		_, err := normalizeAppAvailabilityCreateAttributes(AppAvailabilityCreateAttributes{
			AppID:                "app-123",
			AvailableTerritories: []string{"", "   "},
		})
		if err == nil || !strings.Contains(err.Error(), "at least one available territory is required") {
			t.Fatalf("expected missing territory error, got %v", err)
		}
	})
}

func TestCreateAppAvailabilityBuildsExpectedRequest(t *testing.T) {
	catalogPages, creates := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && r.URL.Path == "/territories" {
			catalogPages++
			if r.URL.Query().Get("cursor") == "next" {
				_, _ = w.Write([]byte(`{"data":[{"type":"territories","id":"CAN"}],"links":{"self":"/territories?cursor=next"}}`))
			} else {
				_, _ = w.Write([]byte(`{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"GBR"}],"links":{"self":"/territories","next":"/territories?cursor=next"}}`))
			}
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/iris/v2/appAvailabilities" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		creates++
		for name, want := range map[string]string{"Origin": appStoreBaseURL, "Referer": appStoreBaseURL + "/", "X-Requested-With": "XMLHttpRequest"} {
			if got := r.Header.Get(name); got != want {
				t.Errorf("%s = %q, want %q", name, got, want)
			}
		}
		var body struct {
			Data struct {
				Attributes    map[string]bool `json:"attributes"`
				Relationships map[string]struct {
					Data json.RawMessage `json:"data"`
				} `json:"relationships"`
			} `json:"data"`
			Included []struct {
				ID            string          `json:"id"`
				Type          string          `json:"type"`
				Attributes    map[string]bool `json:"attributes"`
				Relationships map[string]struct {
					Data struct {
						ID   string `json:"id"`
						Type string `json:"type"`
					} `json:"data"`
				} `json:"relationships"`
			} `json:"included"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		if body.Data.Attributes["availableInNewTerritories"] {
			t.Error("expected future territories disabled")
		}
		if _, legacy := body.Data.Relationships["availableTerritories"]; legacy {
			t.Error("legacy relationship must not be sent")
		}
		var refs []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		}
		if err := json.Unmarshal(body.Data.Relationships["territoryAvailabilities"].Data, &refs); err != nil {
			t.Error(err)
			return
		}
		if len(refs) != 3 || len(body.Included) != 3 {
			t.Errorf("expected all three territories: refs=%d included=%d", len(refs), len(body.Included))
			return
		}
		found := map[string]bool{}
		for i, resource := range body.Included {
			territory := resource.Relationships["territory"].Data.ID
			found[territory] = resource.Attributes["available"]
			if resource.Type != "territoryAvailabilities" || refs[i].Type != resource.Type || refs[i].ID != resource.ID {
				t.Errorf("unmatched resource: %#v", resource)
			}
		}
		if !found["USA"] || !found["GBR"] || found["CAN"] {
			t.Errorf("selection changed: %v", found)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"type":"appAvailabilities","id":"avail-123","attributes":{"availableInNewTerritories":false},"relationships":{"territoryAvailabilities":{"data":[]}}}}`))
	}))
	defer server.Close()
	created, err := testWebClient(server).CreateAppAvailability(context.Background(), AppAvailabilityCreateAttributes{AppID: "app-123", AvailableTerritories: []string{"usa", "gbr"}})
	if err != nil {
		t.Fatalf("CreateAppAvailability: %v", err)
	}
	if catalogPages != 2 || creates != 1 {
		t.Fatalf("pages=%d creates=%d", catalogPages, creates)
	}
	if created.ID != "avail-123" || created.AvailableInNewTerritories || strings.Join(created.AvailableTerritories, ",") != "GBR,USA" {
		t.Fatalf("unexpected receipt: %#v", created)
	}
}

func TestGetAppAvailabilityBuildsExpectedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/app-123/appAvailabilityV2" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"id": "avail-123",
				"type": "appAvailabilities",
				"attributes": {"availableInNewTerritories": true},
				"relationships": {
					"availableTerritories": {
						"data": [
							{"type": "territories", "id": "USA"},
							{"type": "territories", "id": "GBR"}
						]
					}
				}
			}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	got, err := client.GetAppAvailability(context.Background(), "app-123")
	if err != nil {
		t.Fatalf("GetAppAvailability() error = %v", err)
	}
	if got == nil {
		t.Fatal("expected app availability")
		return
	}
	if got.ID != "avail-123" || !got.AvailableInNewTerritories {
		t.Fatalf("unexpected app availability payload: %#v", got)
	}
	if joined := strings.Join(got.AvailableTerritories, ","); joined != "GBR,USA" {
		t.Fatalf("expected sorted territories GBR,USA, got %q", joined)
	}
	if !got.AvailableTerritoriesLoaded {
		t.Fatal("expected availableTerritories relationship to be marked loaded")
	}
}

func TestGetAppAvailabilityTreatsNullDataAsAbsent(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/apps/app-123/appAvailabilityV2" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":null}`))
	}))
	defer server.Close()

	got, err := testWebClient(server).GetAppAvailability(context.Background(), "app-123")
	if err != nil {
		t.Fatalf("GetAppAvailability() error = %v", err)
	}
	if got != nil {
		t.Fatalf("expected an absent availability, got %#v", got)
	}
	if requests != 1 {
		t.Fatalf("expected one request without a retry or fallback, got %d", requests)
	}
}

func TestGetAppAvailabilityRejectsNullDataWithErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{name: "non-empty errors", body: `{"data":null,"errors":[{"code":"PORTAL_FAILURE"}]}`, want: "contained errors"},
		{name: "empty errors", body: `{"data":null,"errors":[]}`, want: "data:null cannot be combined with errors"},
		{name: "null errors", body: `{"data":null,"errors":null}`, want: "malformed errors"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			got, err := testWebClient(server).GetAppAvailability(context.Background(), "app-123")
			if got != nil {
				t.Fatalf("expected no availability on mixed data/errors response, got %#v", got)
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %s error, got %v", tc.name, err)
			}
			if requests != 1 {
				t.Fatalf("expected one request without retry, got %d", requests)
			}
		})
	}
}

func TestGetAppAvailabilityRejectsUnexpectedResourceType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"type":"apps","id":"app-123","attributes":{"availableInNewTerritories":false},"relationships":{"availableTerritories":{"data":[]}}}}`))
	}))
	defer server.Close()

	got, err := testWebClient(server).GetAppAvailability(context.Background(), "app-123")
	if got != nil {
		t.Fatalf("expected no availability on wrong resource type, got %#v", got)
	}
	if err == nil || !strings.Contains(err.Error(), `unexpected resource type "apps"`) {
		t.Fatalf("expected wrong-type error, got %v", err)
	}
}

func TestGetAppAvailabilityPreservesUnauthorizedFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/app-123/appAvailabilityV2" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":[{"status":"403","code":"FORBIDDEN","title":"not permitted"}]}`))
	}))
	defer server.Close()

	got, err := testWebClient(server).GetAppAvailability(context.Background(), "app-123")
	if got != nil {
		t.Fatalf("expected no availability on unauthorized response, got %#v", got)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusForbidden {
		t.Fatalf("expected forbidden APIError, got %v", err)
	}
	if IsNotFound(err) {
		t.Fatal("forbidden availability read must not be classified as absent")
	}
}

func TestGetAppAvailabilityDoesNotRetryHTTPFailures(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"errors":[{"code":"REQUEST_FAILED"}]}`))
			}))
			defer server.Close()

			got, err := testWebClient(server).GetAppAvailability(context.Background(), "app-123")
			if got != nil {
				t.Fatalf("expected no availability on HTTP %d, got %#v", status, got)
			}
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.Status != status {
				t.Fatalf("expected HTTP %d APIError, got %v", status, err)
			}
			if requests != 1 {
				t.Fatalf("expected one request for deterministic HTTP %d, got %d", status, requests)
			}
		})
	}
}

func TestGetAppAvailabilityPreservesMalformedResponseFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/app-123/appAvailabilityV2" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"type":"appAvailabilities"}}`))
	}))
	defer server.Close()

	got, err := testWebClient(server).GetAppAvailability(context.Background(), "app-123")
	if got != nil {
		t.Fatalf("expected no availability on malformed response, got %#v", got)
	}
	if err == nil || !strings.Contains(err.Error(), "app availability id missing from response") {
		t.Fatalf("expected malformed response error, got %v", err)
	}
}

func TestGetAppAvailabilityRejectsMissingOrMalformedData(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{name: "missing data", body: `{}`, want: "app availability response missing data"},
		{name: "array data", body: `{"data":[]}`, want: "failed to parse app availability resource"},
		{name: "numeric id", body: `{"data":{"type":"appAvailabilities","id":123}}`, want: "failed to parse app availability resource"},
		{name: "null id", body: `{"data":{"type":"appAvailabilities","id":null}}`, want: "app availability id missing from response"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			got, err := testWebClient(server).GetAppAvailability(context.Background(), "app-123")
			if got != nil {
				t.Fatalf("expected no availability on %s, got %#v", tc.name, got)
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %s error, got %v", tc.name, err)
			}
		})
	}
}

func TestGetAppAvailabilityMarksEmptyTerritoriesLoaded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"id": "avail-123",
				"type": "appAvailabilities",
				"attributes": {"availableInNewTerritories": false},
				"relationships": {
					"availableTerritories": {"data": []}
				}
			}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	got, err := client.GetAppAvailability(context.Background(), "app-123")
	if err != nil {
		t.Fatalf("GetAppAvailability() error = %v", err)
	}
	if !got.AvailableTerritoriesLoaded {
		t.Fatal("expected empty availableTerritories.data to count as loaded")
	}
	if !got.AvailableInNewTerritoriesKnown {
		t.Fatal("expected availableInNewTerritories=false to count as known")
	}
	if len(got.AvailableTerritories) != 0 {
		t.Fatalf("expected no territories, got %#v", got.AvailableTerritories)
	}
}

func TestGetAppAvailabilityLeavesNewTerritoriesUnknownWhenAttributeOmitted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"id": "avail-123",
				"type": "appAvailabilities",
				"attributes": {},
				"relationships": {
					"availableTerritories": {"data": []}
				}
			}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	got, err := client.GetAppAvailability(context.Background(), "app-123")
	if err != nil {
		t.Fatalf("GetAppAvailability() error = %v", err)
	}
	if !got.AvailableTerritoriesLoaded {
		t.Fatal("expected empty availableTerritories.data to count as loaded")
	}
	if got.AvailableInNewTerritoriesKnown {
		t.Fatal("omitted availableInNewTerritories must not count as known")
	}
}

func TestGetAppAvailabilityReadsPaginatedTerritoryAvailabilities(t *testing.T) {
	fixture := handlertest.New(t)
	const appID = "6759231657"
	availabilityPath := "/apps/" + appID + "/appAvailabilityV2"
	territoriesPath := "/iris/v2/appAvailabilities/" + appID + "/territoryAvailabilities"

	nextLink := ""
	var territoryPages []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case availabilityPath:
			if r.Method != http.MethodGet {
				fixture.Respond(w, "unexpected method: %s", r.Method)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"data": {
					"type": "appAvailabilities",
					"id": "6759231657",
					"attributes": {"availableInNewTerritories": false},
					"relationships": {
						"territoryAvailabilities": {
							"links": {
								"self": "https://appstoreconnect.apple.com/iris/v2/appAvailabilities/6759231657/relationships/territoryAvailabilities",
								"related": "https://appstoreconnect.apple.com/iris/v2/appAvailabilities/6759231657/territoryAvailabilities"
							}
						}
					}
				}
			}`))
		case territoriesPath:
			if r.Method != http.MethodGet {
				fixture.Respond(w, "unexpected method: %s", r.Method)
				return
			}
			query := r.URL.Query()
			if query.Get("cursor") == "" {
				if got := query.Get("include"); got != "territory" {
					fixture.Respond(w, "expected include=territory, got %q", got)
					return
				}
				if got := query.Get("limit"); got != "200" {
					fixture.Respond(w, "expected limit=200, got %q", got)
					return
				}
			}
			territoryPages = append(territoryPages, r.URL.RawQuery)
			w.Header().Set("Content-Type", "application/json")
			if query.Get("cursor") == "BQ" {
				_, _ = w.Write([]byte(`{
					"data": [
						{
							"type": "territoryAvailabilities",
							"id": "eyJzIjoiNjc1OTIzMTY1NyIsInQiOiJHQlIifQ",
							"attributes": {"available": true},
							"relationships": {"territory": {"data": {"type": "territories", "id": "GBR"}}}
						},
						{
							"type": "territoryAvailabilities",
							"id": "eyJzIjoiNjc1OTIzMTY1NyIsInQiOiJDQU4ifQ",
							"attributes": {"available": false},
							"relationships": {"territory": {"data": {"type": "territories", "id": "CAN"}}}
						}
					],
					"links": {
						"self": "https://appstoreconnect.apple.com/iris/v2/appAvailabilities/6759231657/territoryAvailabilities?cursor=BQ&include=territory&limit=200"
					}
				}`))
				return
			}
			_, _ = w.Write([]byte(`{
				"data": [
					{
						"type": "territoryAvailabilities",
						"id": "eyJzIjoiNjc1OTIzMTY1NyIsInQiOiJVU0EifQ",
						"attributes": {"available": true},
						"relationships": {"territory": {"data": {"type": "territories", "id": "USA"}}}
					},
					{
						"type": "territoryAvailabilities",
						"id": "eyJzIjoiNjc1OTIzMTY1NyIsInQiOiJGUkEifQ",
						"attributes": {"available": false},
						"relationships": {"territory": {"data": {"type": "territories", "id": "FRA"}}}
					}
				],
				"links": {
					"self": "https://appstoreconnect.apple.com/iris/v2/appAvailabilities/6759231657/territoryAvailabilities",
					"next": "` + nextLink + `"
				}
			}`))
		default:
			fixture.Respond(w, "unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	nextLink = server.URL + territoriesPath + "?cursor=BQ&include=territory&limit=200"

	client := testWebClient(server)
	got, err := client.GetAppAvailability(context.Background(), appID)
	if err != nil {
		t.Fatalf("GetAppAvailability() error = %v", err)
	}
	if got == nil {
		t.Fatal("expected app availability")
	}
	if !got.AvailableTerritoriesLoaded {
		t.Fatal("expected territoryAvailabilities pagination to mark territories loaded")
	}
	if !got.AvailableInNewTerritoriesKnown || got.AvailableInNewTerritories {
		t.Fatalf("expected availableInNewTerritories=false and known, got %#v", got)
	}
	if joined := strings.Join(got.AvailableTerritories, ","); joined != "GBR,USA" {
		t.Fatalf("expected sorted available territories GBR,USA, got %q", joined)
	}
	if len(territoryPages) != 2 {
		t.Fatalf("expected two territoryAvailabilities pages, got %d (%v)", len(territoryPages), territoryPages)
	}
}

func TestGetAppAvailabilityFailsWhenTerritoryAvailabilitiesUnreadable(t *testing.T) {
	fixture := handlertest.New(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apps/app-123/appAvailabilityV2":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"data": {
					"id": "app-123",
					"type": "appAvailabilities",
					"attributes": {"availableInNewTerritories": false},
					"relationships": {
						"territoryAvailabilities": {
							"links": {
								"related": "https://appstoreconnect.apple.com/iris/v2/appAvailabilities/app-123/territoryAvailabilities"
							}
						}
					}
				}
			}`))
		case "/iris/v2/appAvailabilities/app-123/territoryAvailabilities":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errors":[{"status":"500","code":"UNEXPECTED","title":"boom"}]}`))
		default:
			fixture.Respond(w, "unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	_, err := testWebClient(server).GetAppAvailability(context.Background(), "app-123")
	if err == nil {
		t.Fatal("expected territoryAvailabilities read to fail closed")
	}
	if !strings.Contains(err.Error(), "territoryAvailabilities") {
		t.Fatalf("expected error to name territoryAvailabilities, got %v", err)
	}
}

func TestGetAppAvailabilityFailsWhenTerritoryAvailabilityOmitsAvailable(t *testing.T) {
	fixture := handlertest.New(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apps/app-123/appAvailabilityV2":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"data": {
					"id": "app-123",
					"type": "appAvailabilities",
					"attributes": {"availableInNewTerritories": false}
				}
			}`))
		case "/iris/v2/appAvailabilities/app-123/territoryAvailabilities":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"data": [{
					"type": "territoryAvailabilities",
					"id": "ta-usa",
					"attributes": {},
					"relationships": {"territory": {"data": {"type": "territories", "id": "USA"}}}
				}],
				"links": {"self": "https://appstoreconnect.apple.com/iris/v2/appAvailabilities/app-123/territoryAvailabilities"}
			}`))
		default:
			fixture.Respond(w, "unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	_, err := testWebClient(server).GetAppAvailability(context.Background(), "app-123")
	if err == nil {
		t.Fatal("expected missing available boolean to fail closed")
	}
	if !strings.Contains(err.Error(), "available") {
		t.Fatalf("expected error to name the available attribute, got %v", err)
	}
}

func TestGetAppAvailabilityFailsWhenTerritoryAvailabilitiesDataMissing(t *testing.T) {
	tests := map[string]string{
		"field omitted": `{}`,
		"null":          `{"data":null}`,
	}
	for name, relatedResponse := range tests {
		t.Run(name, func(t *testing.T) {
			fixture := handlertest.New(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/apps/app-123/appAvailabilityV2":
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{
						"data": {
							"id": "avail-123",
							"type": "appAvailabilities",
							"attributes": {"availableInNewTerritories": false},
							"relationships": {
								"territoryAvailabilities": {
									"links": {"related": "https://appstoreconnect.apple.com/iris/v2/appAvailabilities/avail-123/territoryAvailabilities"}
								}
							}
						}
					}`))
				case "/iris/v2/appAvailabilities/avail-123/territoryAvailabilities":
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(relatedResponse))
				default:
					fixture.Respond(w, "unexpected path: %s", r.URL.Path)
				}
			}))
			defer server.Close()

			_, err := testWebClient(server).GetAppAvailability(context.Background(), "app-123")
			if err == nil {
				t.Fatal("expected missing territoryAvailabilities data to fail closed")
			}
			if !strings.Contains(err.Error(), "data") {
				t.Fatalf("expected error to name missing data, got %v", err)
			}
		})
	}
}

func TestGetAppAvailabilityFailsWhenTerritoryAvailabilitiesLinksMissing(t *testing.T) {
	tests := map[string]string{
		"field omitted": `{"data": [{"id":"ta-usa","type":"territoryAvailabilities","attributes":{"available":true},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}}]}`,
		"null":          `{"data": [{"id":"ta-usa","type":"territoryAvailabilities","attributes":{"available":true},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}}],"links":null}`,
		"self omitted":  `{"data": [{"id":"ta-usa","type":"territoryAvailabilities","attributes":{"available":true},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}}],"links":{}}`,
	}
	for name, relatedResponse := range tests {
		t.Run(name, func(t *testing.T) {
			fixture := handlertest.New(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/apps/app-123/appAvailabilityV2":
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{
						"data": {
							"id": "avail-123",
							"type": "appAvailabilities",
							"attributes": {"availableInNewTerritories": false},
							"relationships": {
								"territoryAvailabilities": {
									"links": {"related": "https://appstoreconnect.apple.com/iris/v2/appAvailabilities/avail-123/territoryAvailabilities"}
								}
							}
						}
					}`))
				case "/iris/v2/appAvailabilities/avail-123/territoryAvailabilities":
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(relatedResponse))
				default:
					fixture.Respond(w, "unexpected path: %s", r.URL.Path)
				}
			}))
			defer server.Close()

			_, err := testWebClient(server).GetAppAvailability(context.Background(), "app-123")
			if err == nil {
				t.Fatal("expected missing territoryAvailabilities links to fail closed")
			}
			if !strings.Contains(err.Error(), "links") {
				t.Fatalf("expected error to name missing links, got %v", err)
			}
		})
	}
}

func TestGetAppAvailabilityDoesNotTreatRelatedNotFoundAsMissing(t *testing.T) {
	fixture := handlertest.New(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apps/app-123/appAvailabilityV2":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"data": {
					"id": "avail-123",
					"type": "appAvailabilities",
					"attributes": {"availableInNewTerritories": false},
					"relationships": {
						"territoryAvailabilities": {
							"links": {"related": "https://appstoreconnect.apple.com/iris/v2/appAvailabilities/avail-123/territoryAvailabilities"}
						}
					}
				}
			}`))
		case "/iris/v2/appAvailabilities/avail-123/territoryAvailabilities":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"status":"404","code":"NOT_FOUND","title":"not found"}]}`))
		default:
			fixture.Respond(w, "unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	_, err := testWebClient(server).GetAppAvailability(context.Background(), "app-123")
	if err == nil {
		t.Fatal("expected related collection read to fail")
	}
	if IsNotFound(err) {
		t.Fatalf("related collection 404 must not be treated as missing app availability: %v", err)
	}
	if IsAppAvailabilityNotFound(err) {
		t.Fatalf("related collection 404 must not be treated as missing primary availability: %v", err)
	}
}

func TestGetAppAvailabilityPrimaryNotFoundRemainsNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/app-123/appAvailabilityV2" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errors":[{"status":"404","code":"NOT_FOUND","title":"not found"}]}`))
	}))
	defer server.Close()

	_, err := testWebClient(server).GetAppAvailability(context.Background(), "app-123")
	if err == nil {
		t.Fatal("expected primary availability read to fail")
	}
	if !IsNotFound(err) {
		t.Fatalf("primary availability 404 must remain not found: %v", err)
	}
	if !IsAppAvailabilityNotFound(err) {
		t.Fatalf("JSON:API primary availability 404 must be classified as absent: %v", err)
	}
}

func TestIsAppAvailabilityNotFoundRejectsUnclassified404s(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		want404 bool
	}{
		{name: "status only", status: http.StatusNotFound},
		{name: "HTML portal response", status: http.StatusNotFound, body: "<html>not found</html>"},
		{name: "empty JSON API errors", status: http.StatusNotFound, body: `{"errors":[]}`},
		{name: "wrong error status", status: http.StatusNotFound, body: `{"errors":[{"status":"500","code":"PORTAL_FAILURE"}]}`},
		{name: "wrong error code", status: http.StatusNotFound, body: `{"errors":[{"status":"404","code":"PORTAL_FAILURE"}]}`},
		{name: "mixed error codes", status: http.StatusNotFound, body: `{"errors":[{"status":"404","code":"NOT_FOUND"},{"status":"404","code":"PORTAL_FAILURE"}]}`},
		{name: "mixed data and errors", status: http.StatusNotFound, body: `{"data":null,"errors":[{"status":"404","code":"NOT_FOUND"}]}`},
		{name: "JSON API not found", status: http.StatusNotFound, body: `{"errors":[{"status":"404","code":"NOT_FOUND"}]}`, want404: true},
		{name: "JSON API not found by code", status: http.StatusNotFound, body: `{"errors":[{"code":"NOT_FOUND"}]}`, want404: true},
		{name: "JSON API status without code", status: http.StatusNotFound, body: `{"errors":[{"status":"404"}]}`},
		{name: "server failure", status: http.StatusInternalServerError, body: `{"errors":[{"status":"500","code":"PORTAL_FAILURE"}]}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := &APIError{Status: tc.status, rawBody: []byte(tc.body)}
			if got := IsAppAvailabilityNotFound(err); got != tc.want404 {
				t.Fatalf("IsAppAvailabilityNotFound() = %v, want %v", got, tc.want404)
			}
		})
	}
}

func TestIsNotFound(t *testing.T) {
	if !IsNotFound(&APIError{Status: http.StatusNotFound}) {
		t.Fatal("expected 404 APIError to be treated as not found")
	}
	if IsNotFound(&APIError{Status: http.StatusConflict}) {
		t.Fatal("did not expect 409 APIError to be treated as not found")
	}
}

func TestCreateAppAvailabilityCatalogFailureDoesNotCreate(t *testing.T) {
	for _, tc := range []struct{ name, catalog, want string }{
		{"empty", `{"data":[],"links":{"self":"/territories"}}`, "territory catalog is empty"},
		{"missing selected", `{"data":[{"type":"territories","id":"CAN"}],"links":{"self":"/territories"}}`, "missing from Apple's territory catalog"},
		{"missing links", `{"data":[{"type":"territories","id":"USA"}]}`, "missing non-null links"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/territories" {
					posts++
					http.Error(w, "unexpected mutation", 500)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.catalog))
			}))
			defer server.Close()
			_, err := testWebClient(server).CreateAppAvailability(context.Background(), AppAvailabilityCreateAttributes{AppID: "app-1", AvailableTerritories: []string{"USA"}})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
			if posts != 0 {
				t.Fatalf("catalog failure caused %d mutations", posts)
			}
		})
	}
}
