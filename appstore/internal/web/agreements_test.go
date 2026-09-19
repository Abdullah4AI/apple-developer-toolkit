package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func agreementsTestClient(t *testing.T, fn roundTripFunc) *Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error: %v", err)
	}
	return &Client{httpClient: &http.Client{Jar: jar, Transport: fn}}
}

func contractMessagesFixture() string {
	return `[{"id":"contract_message","group":"Alert","subject":"Apple Developer Program License Agreement Updated","message":"The Apple Developer Program License Agreement has been updated and needs to be reviewed.","priority":null}]`
}

func agreementHistoryFixture(accepted bool) string {
	dateAccepted := "0"
	if accepted {
		dateAccepted = "1787158607000"
	}
	return `{"resultCode":0,"agreements":[{"agreementDownloadUrl":"/services-account/agreement/XG8DNV4HYY/content/pdf","dateEffective":1787060333000,"dateAccepted":` + dateAccepted + `,"dateAgreeBy":1790899199000,"status":"active","version":"5031","isAgreementPLA":true,"agreementId":"XG8DNV4HYY","title":"Apple Developer Program License Agreement"}]}`
}

func TestGetAgreementsStatusReportsPendingAgreement(t *testing.T) {
	requestCount := 0
	client := agreementsTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if r.Method != http.MethodGet || r.URL.String() != "https://appstoreconnect.apple.com/olympus/v1/contractMessages" {
				t.Fatalf("unexpected contract messages request %s %s", r.Method, r.URL.String())
			}
			return developerPortalTestResponse(http.StatusOK, contractMessagesFixture(), nil), nil
		case 2:
			if r.Method != http.MethodPost || r.URL.Path != "/services-account/QH65B2/account/listTeams.action" {
				t.Fatalf("unexpected bootstrap request %s %s", r.Method, r.URL.String())
			}
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), http.Header{"csrf": []string{"bootstrap-csrf"}, "csrf_ts": []string{"bootstrap-ts"}}), nil
		case 3:
			if r.Method != http.MethodPost || r.URL.String() != "https://developer.apple.com/services-account/QH65B2/account/getAgreementHistory" {
				t.Fatalf("unexpected agreement history request %s %s", r.Method, r.URL.String())
			}
			if got := r.Header.Get("csrf"); got != "bootstrap-csrf" {
				t.Fatalf("agreement history request csrf header = %q, want bootstrap-csrf", got)
			}
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode agreement history request: %v", err)
			}
			if payload["teamId"] != "TEAM123456" {
				t.Fatalf("agreement history teamId = %v, want TEAM123456", payload["teamId"])
			}
			return developerPortalTestResponse(http.StatusOK, agreementHistoryFixture(false), nil), nil
		default:
			t.Fatalf("unexpected extra request %s %s", r.Method, r.URL.String())
			return nil, nil
		}
	})

	result, err := client.GetAgreementsStatus(context.Background())
	if err != nil {
		t.Fatalf("GetAgreementsStatus() error: %v", err)
	}
	if requestCount != 3 {
		t.Fatalf("request count = %d, want 3", requestCount)
	}
	if result.TeamID != "TEAM123456" {
		t.Fatalf("TeamID = %q, want TEAM123456", result.TeamID)
	}
	if !result.Pending {
		t.Fatal("Pending = false, want true")
	}
	if len(result.ContractMessages) != 1 || result.ContractMessages[0].Subject != "Apple Developer Program License Agreement Updated" {
		t.Fatalf("ContractMessages = %+v, want the license agreement banner", result.ContractMessages)
	}
	if len(result.Agreements) != 1 {
		t.Fatalf("Agreements length = %d, want 1", len(result.Agreements))
	}
	agreement := result.Agreements[0]
	if agreement.AgreementID != "XG8DNV4HYY" {
		t.Fatalf("AgreementID = %q, want XG8DNV4HYY", agreement.AgreementID)
	}
	if !agreement.IsProgramLicenseAgreement {
		t.Fatal("IsProgramLicenseAgreement = false, want true")
	}
	if !agreement.Pending {
		t.Fatal("agreement Pending = false, want true")
	}
	if agreement.Version != "5031" || agreement.Status != "active" {
		t.Fatalf("agreement version/status = %q/%q, want 5031/active", agreement.Version, agreement.Status)
	}
	wantEffective := time.UnixMilli(1787060333000).UTC().Format(time.RFC3339)
	if agreement.DateEffective != wantEffective {
		t.Fatalf("DateEffective = %q, want %q", agreement.DateEffective, wantEffective)
	}
	if agreement.DateAccepted != "" {
		t.Fatalf("DateAccepted = %q, want empty for a pending agreement", agreement.DateAccepted)
	}
	wantAgreeBy := time.UnixMilli(1790899199000).UTC().Format(time.RFC3339)
	if agreement.DateAgreeBy != wantAgreeBy {
		t.Fatalf("DateAgreeBy = %q, want %q", agreement.DateAgreeBy, wantAgreeBy)
	}
	if agreement.DownloadURL != "https://developer.apple.com/services-account/agreement/XG8DNV4HYY/content/pdf" {
		t.Fatalf("DownloadURL = %q, want absolute developer portal URL", agreement.DownloadURL)
	}
}

func TestGetAgreementsStatusNotPendingWhenAccepted(t *testing.T) {
	requestCount := 0
	client := agreementsTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return developerPortalTestResponse(http.StatusOK, `[ ]`, nil), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
		case 3:
			return developerPortalTestResponse(http.StatusOK, agreementHistoryFixture(true), nil), nil
		default:
			t.Fatalf("unexpected extra request %s %s", r.Method, r.URL.String())
			return nil, nil
		}
	})

	result, err := client.GetAgreementsStatus(context.Background())
	if err != nil {
		t.Fatalf("GetAgreementsStatus() error: %v", err)
	}
	if result.Pending {
		t.Fatal("Pending = true, want false")
	}
	if len(result.ContractMessages) != 0 {
		t.Fatalf("ContractMessages = %+v, want empty", result.ContractMessages)
	}
	if len(result.Agreements) != 1 || result.Agreements[0].Pending {
		t.Fatalf("Agreements = %+v, want one accepted agreement", result.Agreements)
	}
	if result.Agreements[0].DateAccepted == "" {
		t.Fatal("DateAccepted is empty, want RFC3339 timestamp")
	}
}

func TestGetAgreementsStatusPreservesContractMessagesWhenPortalUnavailable(t *testing.T) {
	requestCount := 0
	client := agreementsTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return developerPortalTestResponse(http.StatusOK, contractMessagesFixture(), nil), nil
		case 2:
			return developerPortalTestResponse(http.StatusInternalServerError, `{}`, nil), nil
		default:
			t.Fatalf("unexpected extra request %s %s", r.Method, r.URL.String())
			return nil, nil
		}
	})

	result, err := client.GetAgreementsStatus(context.Background())
	if err == nil {
		t.Fatal("GetAgreementsStatus() error = nil, want Developer Portal error")
	}
	if result == nil || !result.Pending || len(result.ContractMessages) != 1 {
		t.Fatalf("GetAgreementsStatus() result = %#v, want preserved pending contract message", result)
	}
}

func TestGetAgreementHistoryReadsPortalWithoutContractMessages(t *testing.T) {
	requestCount := 0
	client := agreementsTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		if r.URL.Host == "appstoreconnect.apple.com" {
			t.Fatalf("history-only read must not request %s", r.URL.String())
		}
		switch requestCount {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
		case 2:
			if r.URL.Path != developerPortalAgreementHistoryPath {
				t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
			}
			return developerPortalTestResponse(http.StatusOK, agreementHistoryFixture(true), nil), nil
		default:
			t.Fatalf("unexpected extra request %s %s", r.Method, r.URL.String())
			return nil, nil
		}
	})

	result, err := client.GetAgreementHistory(context.Background())
	if err != nil {
		t.Fatalf("GetAgreementHistory() error: %v", err)
	}
	if requestCount != 2 {
		t.Fatalf("request count = %d, want 2 (bootstrap + history)", requestCount)
	}
	if result.TeamID != "TEAM123456" {
		t.Fatalf("TeamID = %q, want TEAM123456", result.TeamID)
	}
	if len(result.ContractMessages) != 0 {
		t.Fatalf("ContractMessages = %+v, want none for a history-only read", result.ContractMessages)
	}
	if result.Pending {
		t.Fatal("Pending = true, want false for an accepted history")
	}
	if len(result.Agreements) != 1 || result.Agreements[0].AgreementID != "XG8DNV4HYY" || result.Agreements[0].Pending {
		t.Fatalf("Agreements = %+v, want one accepted XG8DNV4HYY record", result.Agreements)
	}
}

func TestGetAgreementsStatusSurfacesResultCodeError(t *testing.T) {
	requestCount := 0
	client := agreementsTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return developerPortalTestResponse(http.StatusOK, `[ ]`, nil), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
		default:
			return developerPortalTestResponse(http.StatusOK, `{"resultCode":3050,"resultString":"Please select a team.","userString":"Please select a team."}`, nil), nil
		}
	})

	_, err := client.GetAgreementsStatus(context.Background())
	if err == nil || !strings.Contains(err.Error(), "Please select a team.") {
		t.Fatalf("GetAgreementsStatus() error = %v, want Apple resultCode message", err)
	}
	if err != nil && !strings.Contains(err.Error(), "3050") {
		t.Fatalf("GetAgreementsStatus() error = %v, want resultCode in message", err)
	}
	var resultErr *DeveloperPortalAgreementsResultError
	if !errors.As(err, &resultErr) {
		t.Fatalf("GetAgreementsStatus() error = %T, want *DeveloperPortalAgreementsResultError", err)
	}
	if resultErr.ResultCode != 3050 || resultErr.Message != "Please select a team." {
		t.Fatalf("result error = %+v, want code 3050 and Apple message", resultErr)
	}
}

func TestGetAgreementsStatusSurfacesHTTPStatusErrors(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		wantText   string
		wantAPIErr bool
	}{
		{name: "expired session", status: http.StatusUnauthorized, wantText: "web session is unauthorized or expired for Developer Portal", wantAPIErr: true},
		{name: "server error", status: http.StatusInternalServerError, wantText: "web api error", wantAPIErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			requestCount := 0
			client := agreementsTestClient(t, func(r *http.Request) (*http.Response, error) {
				requestCount++
				switch requestCount {
				case 1:
					return developerPortalTestResponse(http.StatusOK, `[]`, nil), nil
				case 2:
					return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
				default:
					return developerPortalTestResponse(tc.status, `{}`, nil), nil
				}
			})

			_, err := client.GetAgreementsStatus(context.Background())
			if err == nil || !strings.Contains(err.Error(), tc.wantText) {
				t.Fatalf("GetAgreementsStatus() error = %v, want text %q", err, tc.wantText)
			}
			var apiErr *APIError
			if errors.As(err, &apiErr) != tc.wantAPIErr {
				t.Fatalf("GetAgreementsStatus() error = %T, APIError match = %t, want %t", err, errors.As(err, &apiErr), tc.wantAPIErr)
			}
			if tc.wantAPIErr && apiErr.Status != tc.status {
				t.Fatalf("APIError status = %d, want %d", apiErr.Status, tc.status)
			}
		})
	}
}

func TestAcceptAgreementsSendsTeamAndAgreementIDs(t *testing.T) {
	requestCount := 0
	client := agreementsTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if r.Method != http.MethodPost || r.URL.Path != "/services-account/QH65B2/account/listTeams.action" {
				t.Fatalf("unexpected bootstrap request %s %s", r.Method, r.URL.String())
			}
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
		case 2:
			if r.Method != http.MethodPost || r.URL.String() != "https://developer.apple.com/services-account/QH65B2/account/acceptAgreements" {
				t.Fatalf("unexpected accept request %s %s", r.Method, r.URL.String())
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read accept request body: %v", err)
			}
			var payload struct {
				TeamID       string   `json:"teamId"`
				AgreementIDs []string `json:"agreementIds"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode accept request body %q: %v", string(body), err)
			}
			if payload.TeamID != "TEAM123456" {
				t.Fatalf("accept teamId = %q, want TEAM123456", payload.TeamID)
			}
			if len(payload.AgreementIDs) != 1 || payload.AgreementIDs[0] != "XG8DNV4HYY" {
				t.Fatalf("accept agreementIds = %v, want [XG8DNV4HYY]", payload.AgreementIDs)
			}
			return developerPortalTestResponse(http.StatusOK, agreementHistoryFixture(true), nil), nil
		default:
			t.Fatalf("unexpected extra request %s %s", r.Method, r.URL.String())
			return nil, nil
		}
	})

	result, err := client.AcceptAgreements(context.Background(), AgreementsAcceptRequest{AgreementIDs: []string{" XG8DNV4HYY "}})
	if err != nil {
		t.Fatalf("AcceptAgreements() error: %v", err)
	}
	if requestCount != 2 {
		t.Fatalf("request count = %d, want 2", requestCount)
	}
	if result.TeamID != "TEAM123456" {
		t.Fatalf("TeamID = %q, want TEAM123456", result.TeamID)
	}
	if result.Status != "accepted" {
		t.Fatalf("Status = %q, want accepted", result.Status)
	}
	if len(result.AgreementIDs) != 1 || result.AgreementIDs[0] != "XG8DNV4HYY" {
		t.Fatalf("AgreementIDs = %v, want [XG8DNV4HYY]", result.AgreementIDs)
	}
	if len(result.Agreements) != 1 || result.Agreements[0].Pending || result.Agreements[0].DateAccepted == "" {
		t.Fatalf("Agreements = %+v, want one accepted agreement", result.Agreements)
	}
}

func TestAcceptAgreementsRequiresAgreementID(t *testing.T) {
	client := agreementsTestClient(t, func(r *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		return nil, nil
	})

	_, err := client.AcceptAgreements(context.Background(), AgreementsAcceptRequest{AgreementIDs: []string{"  "}})
	if err == nil || !strings.Contains(err.Error(), "agreement id is required") {
		t.Fatalf("AcceptAgreements() error = %v, want missing agreement id error", err)
	}
}

func TestAcceptAgreementsSurfacesResultCodeError(t *testing.T) {
	requestCount := 0
	client := agreementsTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		if requestCount == 1 {
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
		}
		return developerPortalTestResponse(http.StatusOK, `{"resultCode":2200,"resultString":"Not allowed.","userString":"Only the Account Holder can accept this agreement."}`, nil), nil
	})

	_, err := client.AcceptAgreements(context.Background(), AgreementsAcceptRequest{AgreementIDs: []string{"XG8DNV4HYY"}})
	if err == nil || !strings.Contains(err.Error(), "Only the Account Holder can accept this agreement.") {
		t.Fatalf("AcceptAgreements() error = %v, want Apple user message", err)
	}
}

// agreementDownloadPortal serves the Developer Portal bootstrap, agreement
// history, and agreement content endpoints over TLS. contentHandler decides how
// the content endpoint answers; downloadURL overrides the reported
// agreementDownloadUrl when non-empty.
type agreementDownloadPortal struct {
	server         *httptest.Server
	downloadURL    string
	contentCalls   int
	contentHandler func(w http.ResponseWriter, r *http.Request)
}

func newAgreementDownloadPortal(t *testing.T) *agreementDownloadPortal {
	t.Helper()
	portal := &agreementDownloadPortal{}
	portal.contentHandler = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.7 agreement body"))
	}
	portal.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == developerPortalTeamsPath:
			_, _ = io.WriteString(w, developerPortalTeamsFixture())
		case r.Method == http.MethodPost && r.URL.Path == developerPortalAgreementHistoryPath:
			downloadURL := portal.downloadURL
			if downloadURL == "" {
				downloadURL = "/services-account/agreement/XG8DNV4HYY/content/pdf"
			}
			_, _ = io.WriteString(w, `{"resultCode":0,"agreements":[{"agreementDownloadUrl":"`+downloadURL+`","dateEffective":1787060333000,"dateAccepted":1787158607000,"dateAgreeBy":1790899199000,"status":"active","version":"5031","isAgreementPLA":true,"agreementId":"XG8DNV4HYY","title":"Apple Developer Program License Agreement"}]}`)
		case r.Method == http.MethodGet && r.URL.Path == "/services-account/agreement/XG8DNV4HYY/content/pdf":
			portal.contentCalls++
			portal.contentHandler(w, r)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(portal.server.Close)
	return portal
}

func (p *agreementDownloadPortal) client(t *testing.T) *Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New() error: %v", err)
	}
	httpClient := p.server.Client()
	httpClient.Jar = jar
	return &Client{httpClient: httpClient, developerPortalURL: p.server.URL}
}

func TestDownloadAgreementFetchesSameOriginContent(t *testing.T) {
	portal := newAgreementDownloadPortal(t)
	portal.contentHandler = func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); !strings.Contains(got, "application/pdf") {
			t.Errorf("content Accept = %q, want application/pdf", got)
		}
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.7 agreement body"))
	}

	download, err := portal.client(t).DownloadAgreement(context.Background(), " XG8DNV4HYY ")
	if err != nil {
		t.Fatalf("DownloadAgreement() error: %v", err)
	}
	if portal.contentCalls != 1 {
		t.Fatalf("content requests = %d, want 1", portal.contentCalls)
	}
	if download.AgreementID != "XG8DNV4HYY" || download.TeamID != "TEAM123456" {
		t.Fatalf("download identity = %q/%q, want XG8DNV4HYY/TEAM123456", download.AgreementID, download.TeamID)
	}
	if download.Title != "Apple Developer Program License Agreement" || download.Version != "5031" {
		t.Fatalf("download title/version = %q/%q", download.Title, download.Version)
	}
	if download.ContentType != "application/pdf" {
		t.Fatalf("ContentType = %q, want application/pdf", download.ContentType)
	}
	if string(download.Body) != "%PDF-1.7 agreement body" {
		t.Fatalf("Body = %q, want agreement content", download.Body)
	}
}

func TestDownloadAgreementRedactsMalformedRedirectLocation(t *testing.T) {
	portal := newAgreementDownloadPortal(t)
	portal.contentHandler = func(w http.ResponseWriter, r *http.Request) {
		// net/http fails to parse this Location before CheckRedirect runs and
		// embeds the raw header value in its error.
		w.Header().Set("Location", "https://developer.apple.com:badport/agreement.pdf?token=very-secret")
		w.WriteHeader(http.StatusFound)
	}

	_, err := portal.client(t).DownloadAgreement(context.Background(), "XG8DNV4HYY")
	if err == nil || !strings.Contains(err.Error(), "redirect") {
		t.Fatalf("DownloadAgreement() error = %v, want malformed redirect rejection", err)
	}
	for _, leaked := range []string{"very-secret", "token=", "badport"} {
		if strings.Contains(err.Error(), leaked) {
			t.Fatalf("DownloadAgreement() error = %q leaks %q from the Location header", err, leaked)
		}
	}
}

func TestDownloadAgreementRejectsEmptySuccessfulBody(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
	}{
		{name: "204 no content", status: http.StatusNoContent},
		{name: "200 empty body", status: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			portal := newAgreementDownloadPortal(t)
			portal.contentHandler = func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/pdf")
				w.WriteHeader(tc.status)
			}

			download, err := portal.client(t).DownloadAgreement(context.Background(), "XG8DNV4HYY")
			if err == nil {
				t.Fatalf("DownloadAgreement() = %+v, want empty content error", download)
			}
			if !strings.Contains(err.Error(), "empty") {
				t.Fatalf("DownloadAgreement() error = %q, want empty content rejection", err)
			}
		})
	}
}

func TestDownloadAgreementRejectsCrossOriginRedirect(t *testing.T) {
	elsewhereCalls := 0
	elsewhere := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		elsewhereCalls++
		_, _ = w.Write([]byte("leaked"))
	}))
	defer elsewhere.Close()

	portal := newAgreementDownloadPortal(t)
	portal.contentHandler = func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL+"/agreement.pdf?token=very-secret&X-Amz-Signature=abc123", http.StatusFound)
	}

	download, err := portal.client(t).DownloadAgreement(context.Background(), "XG8DNV4HYY")
	if err == nil {
		t.Fatalf("DownloadAgreement() = %+v, want cross-origin redirect error", download)
	}
	if !strings.Contains(err.Error(), "redirect") {
		t.Fatalf("DownloadAgreement() error = %q, want redirect rejection", err)
	}
	for _, leaked := range []string{"very-secret", "X-Amz-Signature", "?token="} {
		if strings.Contains(err.Error(), leaked) {
			t.Fatalf("DownloadAgreement() error = %q leaks %q", err, leaked)
		}
	}
	if elsewhereCalls != 0 {
		t.Fatalf("cross-origin redirect target was requested %d times, want 0", elsewhereCalls)
	}
}

func TestDownloadAgreementRejectsNonHTTPSRedirect(t *testing.T) {
	portal := newAgreementDownloadPortal(t)
	portal.contentHandler = func(w http.ResponseWriter, r *http.Request) {
		insecure := "http://" + strings.TrimPrefix(portal.server.URL, "https://") + "/services-account/agreement/XG8DNV4HYY/content/pdf?sig=very-secret"
		http.Redirect(w, r, insecure, http.StatusFound)
	}

	_, err := portal.client(t).DownloadAgreement(context.Background(), "XG8DNV4HYY")
	if err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("DownloadAgreement() error = %v, want non-https redirect rejection", err)
	}
	if strings.Contains(err.Error(), "very-secret") {
		t.Fatalf("DownloadAgreement() error = %q leaks the signed redirect URL", err)
	}
}

func TestDownloadAgreementCapsSameOriginRedirectLoopDespitePermissiveClientPolicy(t *testing.T) {
	portal := newAgreementDownloadPortal(t)
	portal.contentHandler = func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/services-account/agreement/XG8DNV4HYY/content/pdf?hop=very-secret", http.StatusFound)
	}
	client := portal.client(t)
	client.httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return nil }

	_, err := client.DownloadAgreement(context.Background(), "XG8DNV4HYY")
	if err == nil || !strings.Contains(err.Error(), "10 redirects") {
		t.Fatalf("DownloadAgreement() error = %v, want redirect cap error", err)
	}
	if strings.Contains(err.Error(), "very-secret") {
		t.Fatalf("DownloadAgreement() error = %q leaks the redirect URL", err)
	}
	if portal.contentCalls > 11 {
		t.Fatalf("content requests = %d, want the redirect chain capped at 10 hops", portal.contentCalls)
	}
}

func TestDownloadAgreementRejectsRedirectRewrittenByClientPolicy(t *testing.T) {
	elsewhereCalls := 0
	elsewhere := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		elsewhereCalls++
		_, _ = w.Write([]byte("leaked"))
	}))
	defer elsewhere.Close()
	elsewhereURL, err := url.Parse(elsewhere.URL + "/agreement.pdf?token=very-secret")
	if err != nil {
		t.Fatalf("url.Parse() error: %v", err)
	}

	portal := newAgreementDownloadPortal(t)
	portal.contentHandler = func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/services-account/agreement/XG8DNV4HYY/content/pdf?hop=1", http.StatusFound)
	}
	client := portal.client(t)
	client.httpClient.CheckRedirect = func(redirect *http.Request, via []*http.Request) error {
		redirect.URL = elsewhereURL
		return nil
	}

	_, err = client.DownloadAgreement(context.Background(), "XG8DNV4HYY")
	if err == nil || !strings.Contains(err.Error(), "redirect") {
		t.Fatalf("DownloadAgreement() error = %v, want rejection of the rewritten redirect", err)
	}
	if strings.Contains(err.Error(), "very-secret") {
		t.Fatalf("DownloadAgreement() error = %q leaks the rewritten URL", err)
	}
	if elsewhereCalls != 0 {
		t.Fatalf("rewritten redirect target was requested %d times, want 0", elsewhereCalls)
	}
}

func TestDownloadAgreementRejectsCrossOriginDownloadURL(t *testing.T) {
	portal := newAgreementDownloadPortal(t)
	portal.downloadURL = "https://cdn.example.test/agreements/XG8DNV4HYY.pdf?token=very-secret"

	_, err := portal.client(t).DownloadAgreement(context.Background(), "XG8DNV4HYY")
	if err == nil || !strings.Contains(err.Error(), "Developer Portal origin") {
		t.Fatalf("DownloadAgreement() error = %v, want same-origin rejection", err)
	}
	if strings.Contains(err.Error(), "very-secret") {
		t.Fatalf("DownloadAgreement() error = %q leaks the signed URL", err)
	}
	if portal.contentCalls != 0 {
		t.Fatalf("content requests = %d, want 0 for a rejected URL", portal.contentCalls)
	}
}

func TestDownloadAgreementRejectsNonHTTPSDownloadURL(t *testing.T) {
	requestCount := 0
	client := agreementsTestClient(t, func(r *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, `{"resultCode":0,"agreements":[{"agreementDownloadUrl":"http://developer.apple.com/services-account/agreement/XG8DNV4HYY/content/pdf?sig=very-secret","agreementId":"XG8DNV4HYY","dateEffective":1,"dateAccepted":2}]}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
			return nil, nil
		}
	})

	_, err := client.DownloadAgreement(context.Background(), "XG8DNV4HYY")
	if err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("DownloadAgreement() error = %v, want https requirement", err)
	}
	if strings.Contains(err.Error(), "very-secret") {
		t.Fatalf("DownloadAgreement() error = %q leaks the signed URL", err)
	}
}

func TestDownloadAgreementErrorsRedactSignedURL(t *testing.T) {
	signedHistory := `{"resultCode":0,"agreements":[{"agreementDownloadUrl":"https://developer.apple.com/services-account/agreement/XG8DNV4HYY/content/pdf?token=very-secret&X-Amz-Signature=abc123","agreementId":"XG8DNV4HYY","dateEffective":1,"dateAccepted":2}]}`
	tests := []struct {
		name     string
		content  func(r *http.Request) (*http.Response, error)
		wantText string
	}{
		{
			name: "transport failure",
			content: func(r *http.Request) (*http.Response, error) {
				return nil, errors.New("connection reset by peer")
			},
			wantText: "connection reset by peer",
		},
		{
			name: "server error",
			content: func(r *http.Request) (*http.Response, error) {
				return developerPortalTestResponse(http.StatusInternalServerError, `<html>boom</html>`, http.Header{"Content-Type": []string{"text/html"}}), nil
			},
			wantText: "500",
		},
		{
			name: "expired session",
			content: func(r *http.Request) (*http.Response, error) {
				return developerPortalTestResponse(http.StatusUnauthorized, ``, nil), nil
			},
			wantText: "unauthorized or expired",
		},
		{
			name: "html instead of agreement",
			content: func(r *http.Request) (*http.Response, error) {
				return developerPortalTestResponse(http.StatusOK, `<html>sign in</html>`, http.Header{"Content-Type": []string{"text/html; charset=utf-8"}}), nil
			},
			wantText: "HTML",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			requestCount := 0
			client := agreementsTestClient(t, func(r *http.Request) (*http.Response, error) {
				requestCount++
				switch requestCount {
				case 1:
					return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
				case 2:
					return developerPortalTestResponse(http.StatusOK, signedHistory, nil), nil
				case 3:
					if r.Method != http.MethodGet || r.URL.Host != "developer.apple.com" || r.URL.Query().Get("token") != "very-secret" {
						t.Fatalf("unexpected content request %s %s", r.Method, r.URL.String())
					}
					return tc.content(r)
				default:
					t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
					return nil, nil
				}
			})

			_, err := client.DownloadAgreement(context.Background(), "XG8DNV4HYY")
			if err == nil || !strings.Contains(err.Error(), tc.wantText) {
				t.Fatalf("DownloadAgreement() error = %v, want text %q", err, tc.wantText)
			}
			for _, leaked := range []string{"very-secret", "X-Amz-Signature", "?token=", "content/pdf"} {
				if strings.Contains(err.Error(), leaked) {
					t.Fatalf("DownloadAgreement() error = %q leaks %q", err, leaked)
				}
			}
		})
	}
}

func TestDownloadAgreementRequiresKnownAgreementID(t *testing.T) {
	portal := newAgreementDownloadPortal(t)

	_, err := portal.client(t).DownloadAgreement(context.Background(), "UNKNOWN0001")
	if err == nil || !strings.Contains(err.Error(), "UNKNOWN0001") || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("DownloadAgreement() error = %v, want unknown agreement failure", err)
	}
	if portal.contentCalls != 0 {
		t.Fatalf("content requests = %d, want 0", portal.contentCalls)
	}

	_, err = portal.client(t).DownloadAgreement(context.Background(), "   ")
	if err == nil || !strings.Contains(err.Error(), "agreement id is required") {
		t.Fatalf("DownloadAgreement() blank id error = %v", err)
	}
}

func TestAcceptAgreementsRejectsMissingResultCode(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		switch requestCount {
		case 1:
			if r.Method != http.MethodPost || r.URL.Path != developerPortalTeamsPath {
				t.Errorf("bootstrap request = %s %s, want POST %s", r.Method, r.URL.Path, developerPortalTeamsPath)
			}
			if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
				t.Errorf("bootstrap Content-Type = %q, want application/x-www-form-urlencoded", got)
			}
			_, _ = io.WriteString(w, developerPortalTeamsFixture())
		case 2:
			if r.Method != http.MethodPost || r.URL.Path != developerPortalAcceptAgreementsPath {
				t.Errorf("accept request = %s %s, want POST %s", r.Method, r.URL.Path, developerPortalAcceptAgreementsPath)
			}
			if got := r.Header.Get("Content-Type"); got != "application/json" {
				t.Errorf("accept Content-Type = %q, want application/json", got)
			}
			if got := r.Header.Get("X-Requested-With"); got != "XMLHttpRequest" {
				t.Errorf("accept X-Requested-With = %q, want XMLHttpRequest", got)
			}
			var payload struct {
				TeamID       string   `json:"teamId"`
				AgreementIDs []string `json:"agreementIds"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode accept payload: %v", err)
			}
			if payload.TeamID != "TEAM123456" || len(payload.AgreementIDs) != 1 || payload.AgreementIDs[0] != "XG8DNV4HYY" {
				t.Errorf("accept payload = %+v, want selected team and agreement", payload)
			}
			_, _ = io.WriteString(w, `{"agreements":[]}`)
		default:
			t.Errorf("unexpected request %d: %s %s", requestCount, r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	defer server.Close()
	client := &Client{httpClient: server.Client(), developerPortalURL: server.URL}

	result, err := client.AcceptAgreements(context.Background(), AgreementsAcceptRequest{AgreementIDs: []string{"XG8DNV4HYY"}})
	if err == nil || !strings.Contains(err.Error(), "missing resultCode") {
		t.Fatalf("AcceptAgreements() result/error = %+v/%v, want missing resultCode error", result, err)
	}
}
