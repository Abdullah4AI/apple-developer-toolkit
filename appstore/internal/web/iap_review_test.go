package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFindReviewIAPReturnsFirstMatchingAppScopedIAP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/app-123/inAppPurchases" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if got := r.URL.Query().Get("limit"); got != "300" {
			t.Fatalf("expected limit=300, got %q", got)
		}
		fields := r.URL.Query().Get("fields[inAppPurchases]")
		fieldTokens := map[string]bool{}
		for _, field := range strings.Split(fields, ",") {
			fieldTokens[strings.TrimSpace(field)] = true
		}
		for _, want := range []string{"productId", "referenceName", "state"} {
			if !fieldTokens[want] {
				t.Fatalf("expected fields to contain %q, got %q", want, fields)
			}
		}
		// Apple's iris flavor rejects these names with PARAMETER_ERROR.INVALID
		// (verified against /apps/{APP_ID}/inAppPurchases) — guard against
		// re-adding them in a future patch.
		for _, forbidden := range []string{"name", "inAppPurchaseType", "isAppStoreReviewInProgress", "submitWithNextAppStoreVersion"} {
			if fieldTokens[forbidden] {
				t.Fatalf("fields must not include %q (iris rejects it): %q", forbidden, fields)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{
					"id": "iap-1",
					"type": "inAppPurchases",
					"attributes": {
						"productId": "com.example.removeads",
						"referenceName": "Remove Ads",
						"state": "READY_TO_SUBMIT"
					}
				}
			],
			"links": {
				"next": ""
			}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	got, found, err := client.FindReviewIAP(context.Background(), "app-123", "iap-1")
	if err != nil {
		t.Fatalf("FindReviewIAP() error = %v", err)
	}
	if !found {
		t.Fatal("expected IAP to be found")
	}
	if got.ID != "iap-1" || got.ProductID != "com.example.removeads" || got.ReferenceName != "Remove Ads" || got.State != "READY_TO_SUBMIT" {
		t.Fatalf("unexpected IAP payload: %#v", got)
	}
}

func TestFindReviewIAPMatchesByProductID(t *testing.T) {
	// The iris listing returns a UUID-shaped resource ID that does not match
	// the numeric public-REST-API IAP ID. Callers commonly know the product
	// ID instead, so FindReviewIAP also matches on `productId`.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{
					"id": "ae6d89d7-15c5-4a3d-9041-663a4d40638e",
					"type": "inAppPurchases",
					"attributes": {
						"productId": "com.example.lifetime",
						"referenceName": "Lifetime",
						"state": "READY_TO_SUBMIT"
					}
				}
			],
			"links": {"next": ""}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	got, found, err := client.FindReviewIAP(context.Background(), "app-123", "com.example.lifetime")
	if err != nil {
		t.Fatalf("FindReviewIAP() error = %v", err)
	}
	if !found {
		t.Fatal("expected IAP to be found by product id")
	}
	if got.ID != "ae6d89d7-15c5-4a3d-9041-663a4d40638e" || got.ProductID != "com.example.lifetime" {
		t.Fatalf("unexpected IAP payload: %#v", got)
	}
}

func TestFindReviewIAPMatchesByReferenceName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [{
				"id": "iap-reference-name",
				"type": "inAppPurchases",
				"attributes": {
					"productId": "com.example.lifetime",
					"referenceName": "Lifetime Access",
					"state": "READY_TO_SUBMIT"
				}
			}],
			"links": {"next": ""}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	got, found, err := client.FindReviewIAP(context.Background(), "app-123", "lifetime access")
	if err != nil {
		t.Fatalf("FindReviewIAP() error = %v", err)
	}
	if !found {
		t.Fatal("expected IAP to be found by reference name")
	}
	if got.ID != "iap-reference-name" || got.ReferenceName != "Lifetime Access" {
		t.Fatalf("unexpected IAP payload: %#v", got)
	}
}

func TestFindReviewIAPRejectsAmbiguousProductID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{"id":"iap-1","type":"inAppPurchases","attributes":{"productId":"com.example.duplicate","referenceName":"First"}},
				{"id":"iap-2","type":"inAppPurchases","attributes":{"productId":"com.example.duplicate","referenceName":"Second"}}
			],
			"links": {"next": ""}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	_, found, err := client.FindReviewIAP(context.Background(), "app-123", "com.example.duplicate")
	if err == nil {
		t.Fatal("expected ambiguous product ID to fail")
	}
	if found {
		t.Fatal("ambiguous product ID must not report a match")
	}
	if !strings.Contains(err.Error(), "matches 2 in-app purchases by product ID") {
		t.Fatalf("expected ambiguity diagnostic, got %q", err)
	}
}

func TestFindReviewIAPRejectsAmbiguousReferenceName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{"id":"iap-1","type":"inAppPurchases","attributes":{"productId":"com.example.first","referenceName":"Duplicate Name"}},
				{"id":"iap-2","type":"inAppPurchases","attributes":{"productId":"com.example.second","referenceName":"duplicate name"}}
			],
			"links": {"next": ""}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	_, found, err := client.FindReviewIAP(context.Background(), "app-123", "duplicate name")
	if err == nil {
		t.Fatal("expected ambiguous reference name to fail")
	}
	if found {
		t.Fatal("ambiguous reference name must not report a match")
	}
	if !strings.Contains(err.Error(), "matches 2 in-app purchases by reference name") {
		t.Fatalf("expected ambiguity diagnostic, got %q", err)
	}
}

func TestFindReviewIAPPrefersExactResourceIDOverProductIDFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{
					"id": "resource-from-product-fallback",
					"type": "inAppPurchases",
					"attributes": {
						"productId": "iap-2",
						"referenceName": "Collision Product",
						"state": "READY_TO_SUBMIT"
					}
				},
				{
					"id": "iap-2",
					"type": "inAppPurchases",
					"attributes": {
						"productId": "com.example.target",
						"referenceName": "Exact Resource",
						"state": "READY_TO_SUBMIT"
					}
				}
			],
			"links": {"next": ""}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	got, found, err := client.FindReviewIAP(context.Background(), "app-123", "iap-2")
	if err != nil {
		t.Fatalf("FindReviewIAP() error = %v", err)
	}
	if !found {
		t.Fatal("expected IAP to be found")
	}
	if got.ID != "iap-2" || got.ProductID != "com.example.target" {
		t.Fatalf("expected exact resource-id match to win, got %#v", got)
	}
}

func TestFindReviewIAPPrefersExactResourceIDOnLaterPage(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		switch calls {
		case 1:
			_, _ = w.Write([]byte(`{
				"data": [{
					"id": "resource-from-product-fallback",
					"type": "inAppPurchases",
					"attributes": {"productId": "iap-2"}
				}],
				"links": {"next": "/apps/app-123/inAppPurchases?page=2"}
			}`))
		case 2:
			_, _ = w.Write([]byte(`{
				"data": [{
					"id": "iap-2",
					"type": "inAppPurchases",
					"attributes": {"productId": "com.example.target"}
				}],
				"links": {"next": ""}
			}`))
		default:
			t.Fatalf("unexpected extra request %d: %s", calls, r.URL.String())
		}
	}))
	defer server.Close()

	client := testWebClient(server)
	got, found, err := client.FindReviewIAP(context.Background(), "app-123", "iap-2")
	if err != nil {
		t.Fatalf("FindReviewIAP() error = %v", err)
	}
	if !found {
		t.Fatal("expected IAP to be found")
	}
	if got.ID != "iap-2" || got.ProductID != "com.example.target" {
		t.Fatalf("expected later exact resource-id match to win, got %#v", got)
	}
	if calls != 2 {
		t.Fatalf("expected two paginated requests, got %d", calls)
	}
}

func TestFindReviewIAPRejectsAmbiguousProductIDAcrossPages(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		switch calls {
		case 1:
			_, _ = w.Write([]byte(`{
				"data": [{
					"id": "iap-1",
					"type": "inAppPurchases",
					"attributes": {"productId": "com.example.duplicate", "referenceName": "First"}
				}],
				"links": {"next": "/apps/app-123/inAppPurchases?page=2"}
			}`))
		case 2:
			_, _ = w.Write([]byte(`{
				"data": [{
					"id": "iap-2",
					"type": "inAppPurchases",
					"attributes": {"productId": "com.example.duplicate", "referenceName": "Second"}
				}],
				"links": {"next": ""}
			}`))
		default:
			t.Fatalf("unexpected extra request %d: %s", calls, r.URL.String())
		}
	}))
	defer server.Close()

	_, found, err := testWebClient(server).FindReviewIAP(context.Background(), "app-123", "com.example.duplicate")
	if err == nil || !strings.Contains(err.Error(), "matches 2 in-app purchases by product ID") {
		t.Fatalf("expected cross-page ambiguity error, got found=%t err=%v", found, err)
	}
	if found {
		t.Fatal("ambiguous cross-page product ID must not report a match")
	}
	if calls != 2 {
		t.Fatalf("expected both pages to be scanned, got %d requests", calls)
	}
}

func TestFindReviewIAPMatchesMaxBinRedactedSchema(t *testing.T) {
	const (
		publicASCIAPID = "1234567890"
		irisResourceID = "ae6d89d7-15c5-4a3d-9041-663a4d40638e"
		productID      = "com.example.app.pro.lifetime"
	)

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/apps/app-123/inAppPurchases" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("fields[inAppPurchases]"); got != reviewIAPFields {
			t.Fatalf("expected iris-safe fields %q, got %q", reviewIAPFields, got)
		}
		if got := r.URL.Query().Get("sort"); got != "referenceName" {
			t.Fatalf("expected sort=referenceName, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [{
				"id": "` + irisResourceID + `",
				"type": "inAppPurchases",
				"attributes": {
					"productId": "` + productID + `",
					"referenceName": "Pro Lifetime",
					"state": "READY_TO_SUBMIT"
				}
			}],
			"links": {"next": ""}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	got, found, err := client.FindReviewIAP(context.Background(), "app-123", productID)
	if err != nil {
		t.Fatalf("FindReviewIAP(productID) error = %v", err)
	}
	if !found {
		t.Fatal("expected IAP to be found by bundle-style productId")
	}
	if got.ID != irisResourceID || got.ProductID != productID || got.ReferenceName != "Pro Lifetime" {
		t.Fatalf("unexpected IAP payload: %#v", got)
	}

	_, found, err = client.FindReviewIAP(context.Background(), "app-123", publicASCIAPID)
	if err != nil {
		t.Fatalf("FindReviewIAP(public numeric id) error = %v", err)
	}
	if found {
		t.Fatalf("public numeric ASC IAP id %q must not match Iris schema that only exposes id=%q and productId=%q", publicASCIAPID, irisResourceID, productID)
	}
	if requests != 2 {
		t.Fatalf("expected two lookup requests, got %d", requests)
	}
}

func TestFindReviewIAPStopsAfterMatchingPage(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls > 1 {
			t.Fatalf("expected lookup to stop after finding target, got request %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [{
				"id": "iap-1",
				"type": "inAppPurchases",
				"attributes": {"productId": "com.example.removeads"}
			}],
			"links": {
				"next": "https://example.com/should-not-fetch"
			}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	got, found, err := client.FindReviewIAP(context.Background(), "app-123", "iap-1")
	if err != nil {
		t.Fatalf("FindReviewIAP() error = %v", err)
	}
	if !found || got.ID != "iap-1" {
		t.Fatalf("expected target IAP, got found=%t payload=%#v", found, got)
	}
	if calls != 1 {
		t.Fatalf("expected one request, got %d", calls)
	}
}

func TestFindReviewIAPReturnsFalseWhenMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	_, found, err := client.FindReviewIAP(context.Background(), "app-123", "missing-iap")
	if err != nil {
		t.Fatalf("FindReviewIAP() error = %v", err)
	}
	if found {
		t.Fatal("expected missing IAP")
	}
}

func TestFindReviewIAPRejectsUnexpectedResourceType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"sub-1","type":"subscriptions","attributes":{"productId":"com.example.wrong","referenceName":"Wrong type"}}]}`))
	}))
	defer server.Close()

	_, found, err := testWebClient(server).FindReviewIAP(context.Background(), "app-123", "com.example.wrong")
	if err == nil || !strings.Contains(err.Error(), "unexpected resource type") {
		t.Fatalf("expected unexpected-type error, got found=%t err=%v", found, err)
	}
	if found {
		t.Fatal("unexpected resource type must not be reported as a match")
	}
}

func TestFindReviewIAPRejectsEmptyAppID(t *testing.T) {
	client := &Client{}
	_, found, err := client.FindReviewIAP(context.Background(), "   ", "iap-1")
	if err == nil {
		t.Fatal("expected error for empty app id, got nil")
	}
	if found {
		t.Fatal("expected empty app id to return found=false")
	}
}

func TestFindReviewIAPRejectsEmptyIAPID(t *testing.T) {
	client := &Client{}
	_, found, err := client.FindReviewIAP(context.Background(), "app-123", "   ")
	if err == nil {
		t.Fatal("expected error for empty iap id, got nil")
	}
	if found {
		t.Fatal("expected empty iap id to return found=false")
	}
}

func TestCreateInAppPurchaseSubmissionSendsHiddenAttachPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/inAppPurchaseSubmissions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		var payload struct {
			Data struct {
				Type          string `json:"type"`
				Attributes    map[string]bool
				Relationships map[string]struct {
					Data struct {
						Type string `json:"type"`
						ID   string `json:"id"`
					} `json:"data"`
				} `json:"relationships"`
			} `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		if payload.Data.Type != "inAppPurchaseSubmissions" {
			t.Fatalf("expected type inAppPurchaseSubmissions, got %q", payload.Data.Type)
		}
		if !payload.Data.Attributes["submitWithNextAppStoreVersion"] {
			t.Fatalf("expected hidden attach flag to be true, got %#v", payload.Data.Attributes)
		}
		relationship := payload.Data.Relationships["inAppPurchaseV2"].Data
		if relationship.Type != "inAppPurchases" || relationship.ID != "iap-1" {
			t.Fatalf("unexpected inAppPurchaseV2 relationship: %#v", relationship)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"id": "submission-1",
				"type": "inAppPurchaseSubmissions",
				"attributes": {"submitWithNextAppStoreVersion": true},
				"relationships": {
					"inAppPurchaseV2": {"data": {"type": "inAppPurchases", "id": "iap-1"}}
				}
			}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	got, err := client.CreateInAppPurchaseSubmission(context.Background(), "iap-1")
	if err != nil {
		t.Fatalf("CreateInAppPurchaseSubmission() error = %v", err)
	}
	if got.ID != "submission-1" || got.InAppPurchaseID != "iap-1" || !got.SubmitWithNextAppStoreVersion {
		t.Fatalf("unexpected submission payload: %#v", got)
	}
}

func TestCreateInAppPurchaseSubmissionFallsBackToRequestedIDWhenRelationshipMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"id": "submission-2",
				"type": "inAppPurchaseSubmissions",
				"attributes": {"submitWithNextAppStoreVersion": true}
			}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	got, err := client.CreateInAppPurchaseSubmission(context.Background(), "iap-fallback")
	if err != nil {
		t.Fatalf("CreateInAppPurchaseSubmission() error = %v", err)
	}
	if got.InAppPurchaseID != "iap-fallback" {
		t.Fatalf("expected fallback InAppPurchaseID, got %q", got.InAppPurchaseID)
	}
	if got.ID != "submission-2" || !got.SubmitWithNextAppStoreVersion {
		t.Fatalf("unexpected payload: %#v", got)
	}
}

func TestCreateInAppPurchaseSubmissionRejectsMissingSubmissionID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"type": "inAppPurchaseSubmissions",
				"attributes": {"submitWithNextAppStoreVersion": true}
			}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	if _, err := client.CreateInAppPurchaseSubmission(context.Background(), "iap-1"); err == nil {
		t.Fatal("expected error for missing submission id, got nil")
	}
}

func TestCreateInAppPurchaseSubmissionRejectsUnexpectedResourceType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"id": "submission-1",
				"type": "unexpectedResources",
				"attributes": {"submitWithNextAppStoreVersion": true}
			}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	if _, err := client.CreateInAppPurchaseSubmission(context.Background(), "iap-1"); err == nil {
		t.Fatal("expected error for unexpected resource type, got nil")
	}
}

func TestCreateInAppPurchaseSubmissionRejectsMissingOrEmptyResourceType(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "missing", body: `{"data":{"id":"submission-1"}}`},
		{name: "empty", body: `{"data":{"id":"submission-1","type":""}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			if _, err := testWebClient(server).CreateInAppPurchaseSubmission(context.Background(), "iap-1"); err == nil || !strings.Contains(err.Error(), "missing submission resource type") {
				t.Fatalf("expected missing-resource-type error, got %v", err)
			}
		})
	}
}

func TestCreateInAppPurchaseSubmissionRejectsMismatchedRelationship(t *testing.T) {
	cases := []struct {
		name string
		data string
	}{
		{name: "wrong type", data: `{"type":"subscriptions","id":"iap-1"}`},
		{name: "wrong id", data: `{"type":"inAppPurchases","id":"iap-2"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				body := `{"data":{"id":"submission-1","type":"inAppPurchaseSubmissions","relationships":{"inAppPurchaseV2":{"data":` + tc.data + `}}}}`
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()

			if _, err := testWebClient(server).CreateInAppPurchaseSubmission(context.Background(), "iap-1"); err == nil || !strings.Contains(err.Error(), "inAppPurchaseV2 relationship") {
				t.Fatalf("expected inAppPurchaseV2-relationship error, got %v", err)
			}
		})
	}
}

func TestCreateInAppPurchaseSubmissionRejectsEmptyID(t *testing.T) {
	client := &Client{}
	if _, err := client.CreateInAppPurchaseSubmission(context.Background(), "  "); err == nil {
		t.Fatal("expected error for empty iap id, got nil")
	}
}

func TestCreateInAppPurchaseSubmissionPreservesSanitizedPortalReason(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"errors":[{"code":"STATE_ERROR","title":"Attachment refused","detail":"The IAP is not ready\u001b[31m"}]}`))
	}))
	defer server.Close()

	_, err := testWebClient(server).CreateInAppPurchaseSubmission(context.Background(), "iap-1")
	if err == nil {
		t.Fatal("expected attachment refusal")
	}
	if !strings.Contains(err.Error(), "status 422") {
		t.Fatalf("expected portal status in error, got %q", err)
	}
	if !strings.Contains(err.Error(), "Attachment refused: The IAP is not ready[31m") {
		t.Fatalf("expected sanitized portal reason in error, got %q", err)
	}
	if strings.Contains(err.Error(), "\x1b") {
		t.Fatalf("portal reason contains terminal escape sequence: %q", err)
	}
}
