package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func TestNormalizeMedicalDeviceDeclarationRegionsRejectsUnsupportedRegion(t *testing.T) {
	_, err := NormalizeMedicalDeviceDeclarationRegions([]string{"USA", "CAN"})
	if err == nil || !strings.Contains(err.Error(), "unsupported medical device country/region") {
		t.Fatalf("expected unsupported region error, got %v", err)
	}
}

func TestSetMedicalDeviceDeclarationPostsExpectedRequest(t *testing.T) {
	requirementsCalls := 0
	formCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements":
			requirementsCalls++
			if got := r.URL.Query().Get("contentId"); got != "app-123" {
				t.Fatalf("expected contentId app-123, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			if requirementsCalls == 1 {
				_, _ = w.Write([]byte(`{
					"accountId":"account-123",
					"requirementData":[{
						"contentId":"app-123",
						"requirements":[{
							"id":"req-123",
							"name":"MEDICAL_DEVICE",
							"status":"PENDING_COLLECTION"
						}]
					}]
				}`))
				return
			}
			_, _ = w.Write([]byte(`{
				"accountId":"account-123",
				"requirementData":[{
					"contentId":"app-123",
					"requirements":[{
						"id":"req-123",
						"name":"MEDICAL_DEVICE",
						"status":"COLLECTED",
						"formId":"form-123"
					}]
				}]
			}`))
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements/req-123/forms":
			formCalls++
			if got := r.URL.Query().Get("contentId"); got != "app-123" {
				t.Fatalf("expected contentId app-123, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			if formCalls == 1 {
				_, _ = w.Write([]byte(`{
					"data":{
						"accountId":"account-123",
						"contentId":"app-123",
						"requirementId":"req-123",
						"requirementName":"MEDICAL_DEVICE",
						"medicalDeviceData":{}
					},
					"constraints":{
						"$[*].countriesOrRegions":{
							"attributeName":"countriesOrRegions",
							"options":[
								{"value":"USA"},
								{"value":"GBR"},
								{"value":"EU"}
							]
						},
						"$[*].medicalDeviceData.contactInformation[0].countriesOrRegions":{
							"attributeName":"countriesOrRegions",
							"options":[
								{"listValues":["USA","GBR","EEA"]}
							]
						}
					}
				}`))
			} else {
				_, _ = w.Write([]byte(`{
					"data":{"medicalDeviceData":{"declaration":"no"},"countriesOrRegions":["EEA","GBR","USA"]}
				}`))
			}
		case r.Method == http.MethodPost && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/contents/app-123/requirements/req-123/forms":
			if got := r.Header.Get("X-Csrf-Itc"); got != "itc" {
				t.Fatalf("expected X-Csrf-Itc itc, got %q", got)
			}
			var body struct {
				AccountID          string   `json:"accountId"`
				ContentID          string   `json:"contentId"`
				RequirementID      string   `json:"requirementId"`
				RequirementName    string   `json:"requirementName"`
				CountriesOrRegions []string `json:"countriesOrRegions"`
				MedicalDeviceData  struct {
					Declaration string `json:"declaration"`
				} `json:"medicalDeviceData"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body.AccountID != "account-123" || body.ContentID != "app-123" || body.RequirementID != "req-123" {
				t.Fatalf("unexpected identifiers in body: %#v", body)
			}
			if body.RequirementName != "MEDICAL_DEVICE" {
				t.Fatalf("expected requirement name MEDICAL_DEVICE, got %q", body.RequirementName)
			}
			if body.MedicalDeviceData.Declaration != "no" {
				t.Fatalf("expected declaration no, got %q", body.MedicalDeviceData.Declaration)
			}
			if got := strings.Join(body.CountriesOrRegions, ","); got != "EEA,GBR,USA" {
				t.Fatalf("expected normalized countries EEA,GBR,USA, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	client := testWebClient(server)
	got, err := client.SetMedicalDeviceDeclaration(context.Background(), "account-123", "app-123", false)
	if err != nil {
		t.Fatalf("SetMedicalDeviceDeclaration() error = %v", err)
	}
	if got == nil {
		t.Fatal("expected result")
		return
	}
	if got.AppID != "app-123" {
		t.Fatalf("expected app id app-123, got %q", got.AppID)
	}
	if got.RequirementID != "req-123" || got.RequirementName != "MEDICAL_DEVICE" {
		t.Fatalf("unexpected requirement metadata: %#v", got)
	}
	if got.Status != "COLLECTED" {
		t.Fatalf("expected collected status, got %q", got.Status)
	}
	if got.Declared {
		t.Fatalf("expected declared false, got true")
	}
	if got := strings.Join(got.CountriesOrRegions, ","); got != "EEA,GBR,USA" {
		t.Fatalf("expected countries EEA,GBR,USA, got %q", got)
	}
	if formCalls != 2 {
		t.Fatalf("form calls = %d, want 2 for before and after verification", formCalls)
	}
}

func TestSetMedicalDeviceDeclarationPrefersExactContentIDRequirements(t *testing.T) {
	requirementsCalls := 0
	formCalls := 0
	var requestRegions []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements":
			requirementsCalls++
			w.Header().Set("Content-Type", "application/json")
			if requirementsCalls == 1 {
				_, _ = w.Write([]byte(`{
					"accountId":"account-123",
					"requirementData":[
						{
							"contentId":"",
							"requirements":[{
								"id":"req-generic",
								"name":"OTHER_REQUIREMENT",
								"status":"PENDING_COLLECTION"
							}]
						},
						{
							"contentId":"app-123",
							"requirements":[{
								"id":"req-app",
								"name":"MEDICAL_DEVICE",
								"status":"PENDING_COLLECTION"
							}]
						}
					]
				}`))
				return
			}
			_, _ = w.Write([]byte(`{
				"accountId":"account-123",
				"requirementData":[
					{
						"contentId":"",
						"requirements":[{
							"id":"req-generic",
							"name":"OTHER_REQUIREMENT",
							"status":"PENDING_COLLECTION"
						}]
					},
					{
						"contentId":"app-123",
						"requirements":[{
							"id":"req-app",
							"name":"MEDICAL_DEVICE",
							"status":"COLLECTED",
							"formId":"form-app"
						}]
					}
				]
			}`))
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements/req-app/forms":
			formCalls++
			w.Header().Set("Content-Type", "application/json")
			if formCalls == 1 {
				_, _ = w.Write([]byte(`{
					"constraints":{
						"$[*].countriesOrRegions":{
							"attributeName":"countriesOrRegions",
							"options":[
								{"value":"USA"}
							]
						}
					}
				}`))
			} else {
				_, _ = w.Write([]byte(`{"data":{"medicalDeviceData":{"declaration":"no"},"countriesOrRegions":["USA"]}}`))
			}
		case r.Method == http.MethodPost && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/contents/app-123/requirements/req-app/forms":
			var body struct {
				CountriesOrRegions []string `json:"countriesOrRegions"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			requestRegions = body.CountriesOrRegions
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	client := testWebClient(server)
	got, err := client.SetMedicalDeviceDeclaration(context.Background(), "account-123", "app-123", false)
	if err != nil {
		t.Fatalf("SetMedicalDeviceDeclaration() error = %v", err)
	}
	if got == nil {
		t.Fatal("expected result")
		return
	}
	if got.RequirementID != "req-app" {
		t.Fatalf("expected exact app requirement id req-app, got %q", got.RequirementID)
	}
	if got.Status != "COLLECTED" {
		t.Fatalf("expected collected status, got %q", got.Status)
	}
	if formCalls != 2 {
		t.Fatalf("form calls = %d, want 2 for before and after verification", formCalls)
	}
	if !slices.Equal(requestRegions, []string{"USA"}) {
		t.Fatalf("countriesOrRegions = %v, want form-constrained USA", requestRegions)
	}
}

func TestSetMedicalDeviceDeclarationRejectsUnverifiedReadback(t *testing.T) {
	var requirementsCalls int
	var formCalls int
	var postCalls int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements":
			requirementsCalls++
			w.Header().Set("Content-Type", "application/json")
			status := "PENDING_COLLECTION"
			if requirementsCalls > 1 {
				status = "COLLECTED"
			}
			_, _ = fmt.Fprintf(w, `{
				"accountId":"account-123",
				"requirementData":[{"contentId":"app-123","requirements":[{
					"id":"req-123","name":"MEDICAL_DEVICE","status":%q,"formId":"form-123"
				}]}]
			}`, status)
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements/req-123/forms":
			formCalls++
			w.Header().Set("Content-Type", "application/json")
			if formCalls == 1 {
				_, _ = w.Write([]byte(`{
					"data":{"medicalDeviceData":{}},
					"constraints":{"$[*].countriesOrRegions":{"attributeName":"countriesOrRegions","options":[{"value":"USA"}]}}
				}`))
			} else {
				// The write succeeds, but the subsequent read still says "yes". A
				// successful POST alone must not be reported as a verified "no".
				_, _ = w.Write([]byte(`{
					"data":{"medicalDeviceData":{"declaration":"yes"}},
					"constraints":{"$[*].countriesOrRegions":{"attributeName":"countriesOrRegions","options":[{"value":"USA"}]}}
				}`))
			}
		case r.Method == http.MethodPost && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/contents/app-123/requirements/req-123/forms":
			postCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	_, err := testWebClient(server).SetMedicalDeviceDeclaration(context.Background(), "account-123", "app-123", false)
	if err == nil || !strings.Contains(err.Error(), "verification") {
		t.Fatalf("expected verification error, got %v", err)
	}
	if postCalls != 1 {
		t.Fatalf("POST calls = %d, want 1", postCalls)
	}
	if requirementsCalls != 2 {
		t.Fatalf("requirements GET calls = %d, want 2", requirementsCalls)
	}
	if formCalls != 2 {
		t.Fatalf("form GET calls = %d, want 2 for before and after verification", formCalls)
	}
}

func TestSetMedicalDeviceDeclarationYesPostsSelectedRegionsAndPreservesFormData(t *testing.T) {
	requirementsCalls := 0
	formCalls := 0
	var requestMethod string
	var requestBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements":
			requirementsCalls++
			w.Header().Set("Content-Type", "application/json")
			status := "PENDING_COLLECTION"
			if requirementsCalls > 1 {
				status = "COLLECTED"
			}
			_, _ = fmt.Fprintf(w, `{"requirementData":[{"contentId":"app-123","requirements":[{"id":"req-123","name":"MEDICAL_DEVICE","status":%q}]}]}`, status)
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements/req-123/forms":
			formCalls++
			w.Header().Set("Content-Type", "application/json")
			if formCalls == 1 {
				_, _ = w.Write([]byte(`{
					"data":{
						"accountId":"account-123",
						"contentId":"app-123",
						"requirementId":"req-123",
						"requirementName":"MEDICAL_DEVICE",
						"opaqueField":{"keep":true},
						"countriesOrRegions":["USA"],
						"medicalDeviceData":{"existingField":"preserve"}
					}
				}`))
			} else {
				_, _ = w.Write([]byte(`{"data":{"medicalDeviceData":{"declaration":"yes"},"countriesOrRegions":["EEA","GBR"]}}`))
			}
		case r.Method == http.MethodPost && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/contents/app-123/requirements/req-123/forms":
			requestMethod = r.Method
			if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	got, err := testWebClient(server).SetMedicalDeviceDeclarationWithOptions(context.Background(), "account-123", "app-123", true, MedicalDeviceDeclarationOptions{CountriesOrRegions: []string{"GBR", "EEA"}})
	if err != nil {
		t.Fatalf("SetMedicalDeviceDeclarationWithOptions() error = %v", err)
	}
	if requestMethod != http.MethodPost {
		t.Fatalf("request method = %q, want POST", requestMethod)
	}
	if got == nil || !got.Declared || !got.Changed {
		t.Fatalf("unexpected result: %#v", got)
	}
	if got.Status != "COLLECTED" || !slices.Equal(got.CountriesOrRegions, []string{"EEA", "GBR"}) {
		t.Fatalf("unexpected result metadata: %#v", got)
	}
	if requestBody["opaqueField"].(map[string]any)["keep"] != true {
		t.Fatalf("opaque form field was not preserved: %#v", requestBody)
	}
	if got := requestBody["countriesOrRegions"].([]any); !slices.Equal(got, []any{"EEA", "GBR"}) {
		t.Fatalf("countriesOrRegions = %#v, want EEA,GBR", got)
	}
	medicalData, ok := requestBody["medicalDeviceData"].(map[string]any)
	if !ok || medicalData["declaration"] != "yes" || medicalData["existingField"] != "preserve" {
		t.Fatalf("unexpected medicalDeviceData: %#v", requestBody["medicalDeviceData"])
	}
	if _, ok := medicalData["registrationInfo"]; ok {
		t.Fatalf("initial affirmative payload must not invent registrationInfo: %#v", medicalData)
	}
}

func TestSetMedicalDeviceDeclarationExistingUsesPutAndRebuildsRegionRows(t *testing.T) {
	requirementsCalls := 0
	formCalls := 0
	var requestBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements":
			requirementsCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"requirementData":[{"contentId":"app-123","requirements":[{"id":"req-123","name":"MEDICAL_DEVICE","status":"COLLECTED"}]}]}`)
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements/req-123/forms":
			formCalls++
			w.Header().Set("Content-Type", "application/json")
			if formCalls == 1 {
				_, _ = w.Write([]byte(`{
					"data":{
						"countriesOrRegions":["USA"],
						"opaque":"keep",
						"medicalDeviceData":{
							"declaration":"no",
							"other":"preserve",
							"registrationInfo":[
								{"countriesOrRegions":["USA"],"declaration":"no","registrationNumber":"123","unknown":"keep"},
								{"countriesOrRegions":["CAN"],"declaration":"yes","outside":"drop"}
							]
						}
					}
				}`))
			} else {
				_, _ = w.Write([]byte(`{
					"data":{
						"medicalDeviceData":{
							"declaration":"yes",
							"registrationInfo":[
								{"countriesOrRegions":["EEA"],"declaration":"yes"},
								{"countriesOrRegions":["GBR"],"declaration":"no"},
								{"countriesOrRegions":["USA"],"declaration":"yes"}
							]
						},
						"countriesOrRegions":["USA"]
					}
				}`))
			}
		case r.Method == http.MethodPut && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/contents/app-123/requirements/req-123/forms":
			if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	got, err := testWebClient(server).SetMedicalDeviceDeclarationWithOptions(context.Background(), "account-123", "app-123", true, MedicalDeviceDeclarationOptions{CountriesOrRegions: []string{"EEA", "USA"}})
	if err != nil {
		t.Fatalf("SetMedicalDeviceDeclarationWithOptions() error = %v", err)
	}
	if got == nil || !got.Declared || got.Status != "COLLECTED" {
		t.Fatalf("unexpected result: %#v", got)
	}
	if requirementsCalls != 2 || formCalls != 2 {
		t.Fatalf("verification calls = requirements %d, forms %d; want 2 each", requirementsCalls, formCalls)
	}
	if requestBody["opaque"] != "keep" {
		t.Fatalf("opaque form field was not preserved: %#v", requestBody)
	}
	medicalData := requestBody["medicalDeviceData"].(map[string]any)
	if medicalData["other"] != "preserve" || medicalData["declaration"] != "yes" {
		t.Fatalf("unexpected medicalDeviceData: %#v", medicalData)
	}
	rows := medicalData["registrationInfo"].([]any)
	if len(rows) != 3 {
		t.Fatalf("registrationInfo rows = %d, want 3: %#v", len(rows), rows)
	}
	rowsByRegion := make(map[string]map[string]any, len(rows))
	for _, raw := range rows {
		row := raw.(map[string]any)
		region := row["countriesOrRegions"].([]any)[0].(string)
		rowsByRegion[region] = row
	}
	if rowsByRegion["USA"]["declaration"] != "yes" || rowsByRegion["USA"]["registrationNumber"] != "123" || rowsByRegion["USA"]["unknown"] != "keep" {
		t.Fatalf("USA row was not preserved and updated: %#v", rowsByRegion["USA"])
	}
	if rowsByRegion["EEA"]["declaration"] != "yes" || rowsByRegion["GBR"]["declaration"] != "no" {
		t.Fatalf("unexpected region declarations: %#v", rowsByRegion)
	}
	if _, ok := rowsByRegion["CAN"]; ok {
		t.Fatalf("unsupported existing region should not be copied: %#v", rowsByRegion)
	}
}

func TestSetMedicalDeviceDeclarationPreservesTopLevelFormFieldsOnPut(t *testing.T) {
	formCalls := 0
	var requestBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"requirementData":[{"contentId":"app-123","requirements":[{"id":"req-123","name":"MEDICAL_DEVICE","status":"COLLECTED"}]}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements/req-123/forms":
			formCalls++
			w.Header().Set("Content-Type", "application/json")
			if formCalls == 1 {
				_, _ = w.Write([]byte(`{
					"countriesOrRegions":["USA"],
					"opaque":"keep",
					"medicalDeviceData":{
						"declaration":"no",
						"other":"preserve",
						"registrationInfo":[
							{"countriesOrRegions":["USA"],"declaration":"no","registrationNumber":"123","unknown":"keep"},
							{"countriesOrRegions":["CAN"],"declaration":"yes","outside":"drop"}
						],
						"supportInfo":[{"instruction":"keep"}],
						"contactInformation":[{"name":"keep"}]
					},
					"constraints":{}
				}`))
				return
			}
			_, _ = w.Write([]byte(`{
				"countriesOrRegions":["EEA","USA"],
				"medicalDeviceData":{
					"declaration":"yes",
					"registrationInfo":[
						{"countriesOrRegions":["EEA"],"declaration":"yes"},
						{"countriesOrRegions":["GBR"],"declaration":"no"},
						{"countriesOrRegions":["USA"],"declaration":"yes"}
					]
				}
			}`))
		case r.Method == http.MethodPut && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/contents/app-123/requirements/req-123/forms":
			if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	got, err := testWebClient(server).SetMedicalDeviceDeclarationWithOptions(
		context.Background(),
		"account-123",
		"app-123",
		true,
		MedicalDeviceDeclarationOptions{CountriesOrRegions: []string{"EEA", "USA"}},
	)
	if err != nil {
		t.Fatalf("SetMedicalDeviceDeclarationWithOptions() error = %v", err)
	}
	if got == nil || !got.Declared || !got.Changed {
		t.Fatalf("unexpected result: %#v", got)
	}
	if requestBody["opaque"] != "keep" {
		t.Fatalf("top-level opaque form field was not preserved: %#v", requestBody)
	}
	medicalData, ok := requestBody["medicalDeviceData"].(map[string]any)
	if !ok || medicalData["other"] != "preserve" {
		t.Fatalf("opaque medical-device fields were not preserved: %#v", requestBody["medicalDeviceData"])
	}
	if support, ok := medicalData["supportInfo"].([]any); !ok || len(support) != 1 || support[0].(map[string]any)["instruction"] != "keep" {
		t.Fatalf("support information was not preserved: %#v", medicalData["supportInfo"])
	}
	if contacts, ok := medicalData["contactInformation"].([]any); !ok || len(contacts) != 1 || contacts[0].(map[string]any)["name"] != "keep" {
		t.Fatalf("contact information was not preserved: %#v", medicalData["contactInformation"])
	}
	rows, ok := medicalData["registrationInfo"].([]any)
	if !ok || len(rows) != 3 {
		t.Fatalf("registrationInfo rows = %#v, want 3 rows", medicalData["registrationInfo"])
	}
	rowsByRegion := make(map[string]map[string]any, len(rows))
	for _, raw := range rows {
		row := raw.(map[string]any)
		countries := row["countriesOrRegions"].([]any)
		rowsByRegion[countries[0].(string)] = row
	}
	if rowsByRegion["USA"]["registrationNumber"] != "123" || rowsByRegion["USA"]["unknown"] != "keep" {
		t.Fatalf("USA registration row was not preserved: %#v", rowsByRegion["USA"])
	}
	if _, ok := rowsByRegion["CAN"]; ok {
		t.Fatalf("unsupported existing region should not be copied: %#v", rowsByRegion)
	}
}

func TestSetMedicalDeviceDeclarationPreservesRegionalRowsForFalse(t *testing.T) {
	formCalls := 0
	var requestBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"requirementData":[{"contentId":"app-123","requirements":[{"id":"req-123","name":"MEDICAL_DEVICE","status":"COLLECTED"}]}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements/req-123/forms":
			formCalls++
			w.Header().Set("Content-Type", "application/json")
			if formCalls == 1 {
				_, _ = w.Write([]byte(`{
					"countriesOrRegions":["EEA","USA"],
					"medicalDeviceData":{
						"declaration":"yes",
						"registrationInfo":[
							{"countriesOrRegions":["EEA"],"declaration":"yes","registrationNumber":"123"},
							{"countriesOrRegions":["USA"],"declaration":"yes"}
						],
						"supportInfo":[{"instruction":"keep"}]
					}
				}`))
				return
			}
			// The captured Apple controller keeps the existing registrationInfo
			// array when switching the app-level declaration to No. Those regional
			// values are intentionally preserved while the global answer is No.
			_, _ = w.Write([]byte(`{
				"countriesOrRegions":["EEA","USA"],
				"medicalDeviceData":{
					"declaration":"no",
					"registrationInfo":[
						{"countriesOrRegions":["EEA"],"declaration":"yes","registrationNumber":"123"},
						{"countriesOrRegions":["USA"],"declaration":"yes"}
					],
					"supportInfo":[{"instruction":"keep"}]
				}
			}`))
		case r.Method == http.MethodPut && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/contents/app-123/requirements/req-123/forms":
			if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	got, err := testWebClient(server).SetMedicalDeviceDeclarationWithOptions(context.Background(), "account-123", "app-123", false, MedicalDeviceDeclarationOptions{})
	if err != nil {
		t.Fatalf("SetMedicalDeviceDeclarationWithOptions() error = %v", err)
	}
	if got == nil || got.Declared || !got.Changed || got.Status != "COLLECTED" {
		t.Fatalf("unexpected result: %#v", got)
	}
	medicalData, ok := requestBody["medicalDeviceData"].(map[string]any)
	if !ok || medicalData["declaration"] != "no" || medicalData["supportInfo"] == nil {
		t.Fatalf("false request did not preserve source form shape: %#v", requestBody)
	}
	rows, ok := medicalData["registrationInfo"].([]any)
	if !ok || len(rows) != 2 {
		t.Fatalf("false request did not preserve regional rows: %#v", medicalData["registrationInfo"])
	}
	for _, raw := range rows {
		row := raw.(map[string]any)
		if row["declaration"] != "yes" {
			t.Fatalf("false request changed a preserved regional declaration: %#v", rows)
		}
	}
	if formCalls != 2 {
		t.Fatalf("form calls = %d, want 2 for before and after verification", formCalls)
	}
}

func TestSetMedicalDeviceDeclarationRejectsUnpersistedAffirmativeRegions(t *testing.T) {
	requirementsCalls := 0
	formCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements":
			requirementsCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"requirementData":[{"contentId":"app-123","requirements":[{"id":"req-123","name":"MEDICAL_DEVICE","status":"COLLECTED"}]}]}`)
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements/req-123/forms":
			formCalls++
			w.Header().Set("Content-Type", "application/json")
			if formCalls == 1 {
				_, _ = w.Write([]byte(`{"data":{"medicalDeviceData":{}}}`))
			} else {
				// Apple accepted the app-level answer but persisted every region as
				// "no", so reporting the requested EEA selection would be false.
				_, _ = w.Write([]byte(`{
					"data":{
						"countriesOrRegions":["USA"],
						"medicalDeviceData":{
							"declaration":"yes",
							"registrationInfo":[
								{"countriesOrRegions":["EEA"],"declaration":"no"},
								{"countriesOrRegions":["GBR"],"declaration":"no"},
								{"countriesOrRegions":["USA"],"declaration":"no"}
							]
						}
					}
				}`))
			}
		case r.Method == http.MethodPost && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/contents/app-123/requirements/req-123/forms":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	_, err := testWebClient(server).SetMedicalDeviceDeclarationWithOptions(
		context.Background(),
		"account-123",
		"app-123",
		true,
		MedicalDeviceDeclarationOptions{CountriesOrRegions: []string{"EEA"}},
	)
	if err == nil || !strings.Contains(err.Error(), "verification") {
		t.Fatalf("expected affirmative region verification error, got %v", err)
	}
	if requirementsCalls != 2 || formCalls != 2 {
		t.Fatalf("verification calls = requirements %d, forms %d; want 2 each", requirementsCalls, formCalls)
	}
}

func TestSetMedicalDeviceDeclarationSkipsMatchingAffirmativeDeclaration(t *testing.T) {
	var requirementsCalls int
	var formCalls int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements":
			requirementsCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"requirementData":[{"contentId":"app-123","requirements":[{"id":"req-123","name":"MEDICAL_DEVICE","status":"COLLECTED","formId":"form-123"}]}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/ppm/complianceform/v1/accounts/account-123/requirements/req-123/forms":
			formCalls++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"data":{
					"countriesOrRegions":["EEA","GBR","USA"],
					"medicalDeviceData":{
						"declaration":"yes",
						"registrationInfo":[
							{"countriesOrRegions":["EEA"],"declaration":"yes"},
							{"countriesOrRegions":["GBR"],"declaration":"yes"},
							{"countriesOrRegions":["USA"],"declaration":"yes"}
						]
					}
				}
			}`))
		default:
			t.Fatalf("unexpected write or request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	got, err := testWebClient(server).SetMedicalDeviceDeclarationWithOptions(
		context.Background(),
		"account-123",
		"app-123",
		true,
		MedicalDeviceDeclarationOptions{CountriesOrRegions: []string{"USA", "GBR", "EEA"}},
	)
	if err != nil {
		t.Fatalf("SetMedicalDeviceDeclarationWithOptions() error = %v", err)
	}
	if got == nil || !got.Declared || got.Changed {
		t.Fatalf("unexpected no-op result: %#v", got)
	}
	if requirementsCalls != 1 || formCalls != 1 {
		t.Fatalf("no-op reads = requirements %d, forms %d; want 1 each", requirementsCalls, formCalls)
	}
}

func TestMedicalDeviceRegionsFromConstraintsCollectsUniqueNormalizedSortedRegions(t *testing.T) {
	constraints := map[string]complianceConstraint{
		"ignored": {
			AttributeName: "somethingElse",
			Options: []complianceConstraintOption{
				{Value: "IGNORED"},
			},
		},
		"regions": {
			AttributeName: " countriesOrRegions ",
			Options: []complianceConstraintOption{
				{Value: "usa"},
				{Value: " EU "},
				{ListValues: []string{"GBR", "usa", "EEA", " "}},
			},
		},
	}

	got, err := medicalDeviceRegionsFromConstraints(constraints)
	if err != nil {
		t.Fatalf("medicalDeviceRegionsFromConstraints() error = %v", err)
	}

	want := []string{"EEA", "GBR", "USA"}
	if !slices.Equal(got, want) {
		t.Fatalf("medicalDeviceRegionsFromConstraints() = %v, want %v", got, want)
	}
}

func TestMedicalDeviceRegionsFromConstraintsErrorsForMissingMetadata(t *testing.T) {
	tests := []struct {
		name        string
		constraints map[string]complianceConstraint
		wantErr     string
	}{
		{
			name:        "empty constraints",
			constraints: nil,
			wantErr:     "medical device form constraints are missing",
		},
		{
			name: "no countriesOrRegions attribute",
			constraints: map[string]complianceConstraint{
				"other": {
					AttributeName: "somethingElse",
					Options:       []complianceConstraintOption{{Value: "USA"}},
				},
			},
			wantErr: "medical device countries/regions are missing from form metadata",
		},
		{
			name: "no region values",
			constraints: map[string]complianceConstraint{
				"regions": {
					AttributeName: "countriesOrRegions",
					Options: []complianceConstraintOption{
						{Value: "   "},
						{ListValues: []string{"", " "}},
					},
				},
			},
			wantErr: "medical device countries/regions are missing from form metadata",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := medicalDeviceRegionsFromConstraints(tc.constraints)
			if err == nil {
				t.Fatalf("expected error, got regions %v", got)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}
