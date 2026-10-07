package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
)

func TestAdsAgentReadOnlyEvalWorkflow(t *testing.T) {
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_ORG_ID", "987654")
	t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "111")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))

	reportPayload := writeAdsEvalPayload(t, "report.json", `{
		"startTime": "2026-05-01",
		"endTime": "2026-05-31",
		"returnRowTotals": true,
		"selector": {
			"orderBy": [
				{"field": "impressions", "sortOrder": "DESCENDING"}
			],
			"pagination": {"offset": 0, "limit": 100}
		}
	}`)

	log := newRequestLog(5)
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		assertAdsEvalBearer(t, req)
		log.Add(req.Method + " " + req.URL.RequestURI())

		switch req.URL.Path {
		case "/api/v5/me":
			assertAdsEvalNoOrg(t, req)
			assertAdsEvalNoBody(t, req)
			return adsJSONResponse(200, `{"data":{"id":"user-1"}}`), nil
		case "/api/v5/acls":
			assertAdsEvalNoOrg(t, req)
			assertAdsEvalNoBody(t, req)
			return adsJSONResponse(200, `{"data":[{"orgId":987654}]}`), nil
		case "/api/v5/campaigns":
			assertAdsEvalOrg(t, req)
			if req.Method != http.MethodGet {
				t.Fatalf("campaigns method = %s, want GET", req.Method)
			}
			if got := req.URL.Query().Get("limit"); got != "1" {
				t.Fatalf("campaigns limit = %q, want 1", got)
			}
			assertAdsEvalNoBody(t, req)
			return adsJSONResponse(200, `{"data":[{"id":12345}],"pagination":{"itemsPerPage":1,"startIndex":0,"totalResults":1}}`), nil
		case "/api/v5/reports/campaigns":
			assertAdsEvalOrg(t, req)
			if req.Method != http.MethodPost {
				t.Fatalf("reports method = %s, want POST", req.Method)
			}
			if got := req.Header.Get("Content-Type"); got != "application/json" {
				t.Fatalf("Content-Type = %q, want application/json", got)
			}
			body := readAdsEvalJSONBody(t, req)
			selector, ok := body["selector"].(map[string]any)
			if !ok {
				t.Fatalf("report body selector = %#v, want object", body["selector"])
			}
			if _, ok := selector["orderBy"].([]any); !ok {
				t.Fatalf("report body selector.orderBy = %#v, want array", selector["orderBy"])
			}
			return adsJSONResponse(200, `{"data":{"reportingDataResponse":{"row":[{"metadata":{"campaignId":12345}}]}}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	for _, tc := range []struct {
		args    []string
		warning string
	}{
		{args: []string{"ads", "v5", "me", "--output", "json"}, warning: adsV5ReplacementWarning("v5 me", "me view")},
		{args: []string{"ads", "v5", "me", "view", "--output", "json"}, warning: adsV5ReplacementWarning("v5 me view", "me view")},
		{args: []string{"ads", "v5", "acls", "--output", "json"}, warning: adsV5ReplacementWarning("v5 acls", "acls list")},
		{args: []string{"ads", "v5", "campaigns", "--limit", "1", "--output", "json"}, warning: adsV5ReplacementWarning("v5 campaigns", "campaigns find")},
		{args: []string{"ads", "v5", "reports", "campaigns", "--file", reportPayload, "--output", "json"}, warning: adsV5ReplacementWarning("v5 reports campaigns", "reports apps campaigns")},
		{args: []string{"ads", "v5", "api", "request", "--method", "GET", "--path", "v5/me", "--output", "json"}, warning: adsV5ReplacementWarning("v5 api request", "api request")},
	} {
		stdout, stderr, err := runAdsEvalCommand(t, tc.args...)
		if err != nil {
			t.Fatalf("asc %s error: %v\nstderr: %s", strings.Join(tc.args, " "), err, stderr)
		}
		if stderr != tc.warning {
			t.Fatalf("asc %s stderr = %q, want %q", strings.Join(tc.args, " "), stderr, tc.warning)
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
			t.Fatalf("asc %s stdout is not JSON: %v\n%s", strings.Join(tc.args, " "), err, stdout)
		}
	}

	joined := strings.Join(log.Snapshot(), "\n")
	for _, want := range []string{
		"GET /api/v5/me",
		"GET /api/v5/acls",
		"GET /api/v5/campaigns?limit=1",
		"POST /api/v5/reports/campaigns",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("requests = %q, missing %q", joined, want)
		}
	}
}

func TestAdsAuthDiscoverSummarizesMeAndAcls(t *testing.T) {
	isolateAdsGuideEnv(t)
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_ORG_ID", "987654")
	t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "111")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))

	log := newRequestLog(2)
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		assertAdsEvalBearer(t, req)
		if req.URL.Host != "api.ads.apple.com" {
			t.Fatalf("unexpected auth discovery host %q", req.URL.Host)
		}
		assertAdsEvalNoOrg(t, req)
		assertAdsEvalNoBody(t, req)
		log.Add(req.Method + " " + req.URL.RequestURI())

		switch req.URL.Path {
		case "/v1/me":
			return adsJSONResponse(200, `{"result":{"userId":"user-1","name":"Ada Example"}}`), nil
		case "/v1/acls":
			return adsJSONResponse(200, `{"result":{"acls":[{"adAccount":{"id":111,"orgId":987654,"name":"Example Org"},"roles":["Admin"]},{"adAccount":{"id":222,"orgId":123456,"name":"Other Org"},"roles":["ReadOnly"]}]}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	stdout, stderr, err := runAdsEvalCommand(t, "ads", "auth", "discover", "--output", "json")
	if err != nil {
		t.Fatalf("discover error: %v\nstderr: %s", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("discover stderr = %q, want empty", stderr)
	}
	if strings.Contains(stdout, `"ACCESS"`) {
		t.Fatalf("discover leaked access token: %s", stdout)
	}

	var result struct {
		AuthSource        string `json:"auth_source"`
		OrgID             string `json:"org_id"`
		OrgIDSource       string `json:"org_id_source"`
		AdAccountID       string `json:"ad_account_id"`
		AdAccountIDSource string `json:"ad_account_id_source"`
		Me                struct {
			ID string `json:"id"`
		} `json:"me"`
		Accounts []struct {
			AdAccountID string   `json:"ad_account_id"`
			OrgID       string   `json:"org_id"`
			Name        string   `json:"name"`
			Roles       []string `json:"roles"`
			Active      bool     `json:"active"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("discover stdout is not JSON: %v\n%s", err, stdout)
	}
	if result.AuthSource != "ASC_ADS_ACCESS_TOKEN" || result.OrgID != "987654" || result.OrgIDSource != "ASC_ADS_ORG_ID" {
		t.Fatalf("discovery context = %+v, want env token/org", result)
	}
	if result.AdAccountID != "111" || result.AdAccountIDSource != "ASC_ADS_AD_ACCOUNT_ID" || result.Me.ID != "user-1" {
		t.Fatalf("ad account/me = %+v, want selected account and stable v1 me", result)
	}
	if len(result.Accounts) != 2 || result.Accounts[0].AdAccountID != "111" || result.Accounts[0].OrgID != "987654" || result.Accounts[0].Name != "Example Org" || !result.Accounts[0].Active {
		t.Fatalf("accounts = %+v, want active ad account 111 first", result.Accounts)
	}
	if got := strings.Join(result.Accounts[0].Roles, ","); got != "Admin" {
		t.Fatalf("roles = %q, want Admin", got)
	}

	requests := strings.Join(log.Snapshot(), "\n")
	for _, want := range []string{"GET /v1/me", "GET /v1/acls"} {
		if !strings.Contains(requests, want) {
			t.Fatalf("requests = %q, missing %q", requests, want)
		}
	}
}

func TestAdsPlatformAdAccountViewUsesOneValueForPathAndContext(t *testing.T) {
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "123")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))

	seen := []string{}
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		seen = append(seen, req.URL.Path+" "+req.Header.Get("X-AP-Context"))
		return adsJSONResponse(200, `{"result":{"id":123}}`), nil
	}))

	for _, args := range [][]string{
		{"ads", "ad-accounts", "view", "--output", "json"},
		{"ads", "ad-accounts", "view", "--ad-account", "456", "--output", "json"},
	} {
		stdout, stderr, err := runAdsEvalCommand(t, args...)
		if err != nil || stderr != "" || !json.Valid([]byte(stdout)) {
			t.Fatalf("asc %s stdout=%q stderr=%q error=%v", strings.Join(args, " "), stdout, stderr, err)
		}
	}
	if got, want := strings.Join(seen, "\n"), "/v1/ad-accounts/123 adAccountId=123;\n/v1/ad-accounts/456 adAccountId=456;"; got != want {
		t.Fatalf("requests = %q, want %q", got, want)
	}
}

func TestAdsAuthDiscoverTableShowsUserAndAccounts(t *testing.T) {
	isolateAdsGuideEnv(t)
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_ORG_ID", "987654")
	t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "111")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))

	log := newRequestLog(2)
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		assertAdsEvalBearer(t, req)
		if req.URL.Host != "api.ads.apple.com" {
			t.Fatalf("unexpected auth discovery host %q", req.URL.Host)
		}
		assertAdsEvalNoOrg(t, req)
		assertAdsEvalNoBody(t, req)
		log.Add(req.Method + " " + req.URL.RequestURI())

		switch req.URL.Path {
		case "/v1/me":
			return adsJSONResponse(200, `{"result":{"userId":"user-1","name":"Ada Example"}}`), nil
		case "/v1/acls":
			return adsJSONResponse(200, `{"result":{"acls":[{"adAccount":{"id":111,"orgId":987654,"name":"Example Org"},"roles":["Admin"]}]}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	stdout, stderr, err := runAdsEvalCommand(t, "ads", "auth", "discover")
	if err != nil {
		t.Fatalf("discover table error: %v\nstderr: %s", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("discover table stderr = %q, want empty", stderr)
	}
	for _, want := range []string{
		"Auth source: ASC_ADS_ACCESS_TOKEN",
		"User: Ada Example (user-1)",
		"Selected org: 987654 (ASC_ADS_ORG_ID)",
		"111 - Example Org (active)",
		"Roles: Admin",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("discover table stdout = %q, missing %q", stdout, want)
		}
	}
	if requests := strings.Join(log.Snapshot(), "\n"); !strings.Contains(requests, "GET /v1/me") || !strings.Contains(requests, "GET /v1/acls") {
		t.Fatalf("requests = %q, want me and acls lookups", requests)
	}
}

func TestAdsAuthDiscoverAcceptsRealMeFieldsAndSingletonACL(t *testing.T) {
	isolateAdsGuideEnv(t)
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_ORG_ID", "987654")
	t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "111")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))

	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		assertAdsEvalBearer(t, req)
		if req.URL.Host != "api.ads.apple.com" {
			t.Fatalf("unexpected auth discovery host %q", req.URL.Host)
		}
		assertAdsEvalNoOrg(t, req)
		assertAdsEvalNoBody(t, req)

		switch req.URL.Path {
		case "/v1/me":
			return adsJSONResponse(200, `{"result":{"userId":"user-1","orgId":987654}}`), nil
		case "/v1/acls":
			return adsJSONResponse(200, `{"result":{"acls":[{"adAccount":{"id":111,"orgId":987654,"name":"Example Org"},"roles":["Admin"]}]}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	stdout, stderr, err := runAdsEvalCommand(t, "ads", "auth", "discover")
	if err != nil {
		t.Fatalf("discover table error: %v\nstderr: %s", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("discover table stderr = %q, want empty", stderr)
	}
	for _, want := range []string{
		"User: user-1",
		"111 - Example Org (active)",
		"Roles: Admin",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("discover table stdout = %q, missing %q", stdout, want)
		}
	}
}

func TestAdsAuthDiscoverRejectsInvalidOutput(t *testing.T) {
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}))

	stdout, stderr, err := runAdsEvalCommand(t, "ads", "auth", "discover", "--output", "invalid")
	if rootcmd.ExitCodeFromError(err) != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d (err=%v)", rootcmd.ExitCodeFromError(err), rootcmd.ExitUsage, err)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, `(got "invalid")`) {
		t.Fatalf("stderr = %q, want invalid --output usage error", stderr)
	}
}

func TestAdsAuthDiscoverRejectsInvalidExplicitAdAccountBeforeNetwork(t *testing.T) {
	isolateAdsGuideEnv(t)
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}))

	stdout, stderr, err := runAdsEvalCommand(t, "ads", "auth", "discover", "--ad-account", "123;orgId=456", "--output", "json")
	if rootcmd.ExitCodeFromError(err) != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d (err=%v)", rootcmd.ExitCodeFromError(err), rootcmd.ExitUsage, err)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "--ad-account") || !strings.Contains(stderr, "semicolon") {
		t.Fatalf("stderr = %q, want invalid --ad-account usage error", stderr)
	}
}

func TestAdsAuthDiscoverRejectsControlOnlyExplicitAdAccountBeforeNetwork(t *testing.T) {
	isolateAdsGuideEnv(t)
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}))

	stdout, stderr, err := runAdsEvalCommand(t, "ads", "auth", "discover", "--ad-account", "\n", "--output", "json")
	if rootcmd.ExitCodeFromError(err) != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d (err=%v)", rootcmd.ExitCodeFromError(err), rootcmd.ExitUsage, err)
	}
	if stdout != "" || !strings.Contains(stderr, "control characters") {
		t.Fatalf("stdout = %q stderr = %q, want control-character usage error", stdout, stderr)
	}
}

func TestAdsAuthDiscoverRejectsMalformedDiscoveryResponses(t *testing.T) {
	tests := []struct {
		name    string
		meBody  string
		aclBody string
		wantErr string
	}{
		{
			name:    "me",
			meBody:  `{"data":`,
			aclBody: `{"data":[]}`,
			wantErr: "me response parse failed",
		},
		{
			name:    "acls",
			meBody:  `{"data":{"id":"user-1"}}`,
			aclBody: `{"data":`,
			wantErr: "acl response parse failed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isolateAdsGuideEnv(t)
			t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
			installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				assertAdsEvalBearer(t, req)
				if req.URL.Host != "api.ads.apple.com" {
					t.Fatalf("unexpected auth discovery host %q", req.URL.Host)
				}
				assertAdsEvalNoOrg(t, req)
				assertAdsEvalNoBody(t, req)

				switch req.URL.Path {
				case "/v1/me":
					if test.meBody == `{"data":` {
						return adsJSONResponse(200, `{"result":`), nil
					}
					if test.meBody == `{"data":{"id":"user-1"}}` {
						return adsJSONResponse(200, `{"result":{"userId":"user-1"}}`), nil
					}
					return adsJSONResponse(200, strings.Replace(test.meBody, `{"data":`, `{"result":`, 1)), nil
				case "/v1/acls":
					if test.aclBody == `{"data":` {
						return adsJSONResponse(200, `{"result":`), nil
					}
					if test.aclBody == `{"data":[]}` {
						return adsJSONResponse(200, `{"result":{"acls":[]}}`), nil
					}
					return adsJSONResponse(200, strings.Replace(test.aclBody, `{"data":`, `{"result":{"acls":`, 1)), nil
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			}))

			stdout, stderr, err := runAdsEvalCommand(t, "ads", "auth", "discover", "--output", "json")
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want %q", err, test.wantErr)
			}
			if stdout != "" || stderr != "" {
				t.Fatalf("stdout = %q stderr = %q, want empty output on parse failure", stdout, stderr)
			}
		})
	}
}

func TestAdsAuthDiscoverContinuesWhenOptionalOrgConfigIsInvalid(t *testing.T) {
	isolateAdsGuideEnv(t)
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_AD_ACCOUNT_ID", "111")
	configPath := writeAdsEvalPayload(t, "config.json", `{"ads":`)
	t.Setenv("ASC_CONFIG_PATH", configPath)

	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		assertAdsEvalBearer(t, req)
		if req.URL.Host != "api.ads.apple.com" {
			t.Fatalf("unexpected auth discovery host %q", req.URL.Host)
		}
		assertAdsEvalNoOrg(t, req)
		assertAdsEvalNoBody(t, req)

		switch req.URL.Path {
		case "/v1/me":
			return adsJSONResponse(200, `{"result":{"userId":"user-1","name":"Ada Example"}}`), nil
		case "/v1/acls":
			return adsJSONResponse(200, `{"result":{"acls":[{"adAccount":{"id":111,"orgId":987654,"name":"Example Org"}}]}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	stdout, stderr, err := runAdsEvalCommand(t, "ads", "auth", "discover", "--output", "json")
	if err != nil {
		t.Fatalf("discover error: %v\nstderr: %s", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("discover stderr = %q, want empty", stderr)
	}
	var result struct {
		OrgID       string `json:"org_id"`
		OrgIDSource string `json:"org_id_source"`
		Accounts    []struct {
			AdAccountID string `json:"ad_account_id"`
			OrgID       string `json:"org_id"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("discover stdout is not JSON: %v\n%s", err, stdout)
	}
	if result.OrgID != "" || result.OrgIDSource != "" {
		t.Fatalf("org context = %+v, want no selected org from invalid config", result)
	}
	if len(result.Accounts) != 1 || result.Accounts[0].AdAccountID != "111" || result.Accounts[0].OrgID != "987654" {
		t.Fatalf("accounts = %+v, want discovered account despite invalid config", result.Accounts)
	}
}

func TestAdsCampaignUpdateSendsRequiredEnvelope(t *testing.T) {
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_ORG_ID", "987654")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))

	campaignUpdate := writeAdsEvalPayload(t, "campaign-update.json", `{"campaign":{"status":"PAUSED"}}`)
	requests := newRequestLog(1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests.Add(req.Method + " " + req.URL.RequestURI())
		if req.Method != http.MethodPut || req.URL.Path != "/api/v5/campaigns/1001" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		if got := req.Header.Get("Authorization"); got != "Bearer ACCESS" {
			t.Errorf("Authorization = %q, want Bearer ACCESS", got)
		}
		if got := req.Header.Get("X-AP-Context"); got != "orgId=987654" {
			t.Errorf("X-AP-Context = %q, want orgId=987654", got)
		}

		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Errorf("decode campaign update body: %v", err)
		}
		campaign, ok := body["campaign"].(map[string]any)
		if !ok {
			t.Errorf("campaign update body = %#v, want campaign envelope", body)
		} else if got := campaign["status"]; got != "PAUSED" {
			t.Errorf("campaign update status = %#v, want PAUSED", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"id":1001,"status":"PAUSED"}}`)
	}))
	t.Cleanup(server.Close)

	transport, ok := server.Client().Transport.(*http.Transport)
	if !ok {
		t.Fatalf("server transport type = %T, want *http.Transport", server.Client().Transport)
	}
	transport = transport.Clone()
	transport.Proxy = nil
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	transport.TLSClientConfig.ServerName = "example.com"
	dialer := &net.Dialer{}
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, server.Listener.Addr().String())
	}
	installDefaultTransport(t, transport)

	stdout, stderr, err := runAdsEvalCommand(
		t,
		"ads", "v5", "campaigns", "update",
		"--campaign", "1001",
		"--file", campaignUpdate,
		"--confirm",
		"--output", "json",
	)
	if err != nil {
		t.Fatalf("campaign update error: %v\nstderr: %s", err, stderr)
	}
	if got, want := stderr, adsV5ReplacementWarning("v5 campaigns update", "campaigns update"); got != want {
		t.Fatalf("campaign update stderr = %q, want %q", got, want)
	}
	if got := requests.Snapshot(); len(got) != 1 || got[0] != "PUT /api/v5/campaigns/1001" {
		t.Fatalf("requests = %q, want one campaign update", got)
	}
	if !strings.Contains(stdout, `"status":"PAUSED"`) {
		t.Fatalf("campaign update stdout = %q, want paused response", stdout)
	}
}

func TestAdsAgentMutationEvalWorkflow(t *testing.T) {
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_ORG_ID", "987654")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))

	campaignCreate := writeAdsEvalPayload(t, "campaign-create.json", `{"name":"ASC CLI Eval Campaign","status":"PAUSED"}`)
	campaignUpdate := writeAdsEvalPayload(t, "campaign-update.json", `{"campaign":{"status":"PAUSED"}}`)
	keywords := writeAdsEvalPayload(t, "keywords.json", `[{"text":"example keyword","matchType":"EXACT","status":"PAUSED"}]`)
	keywordIDs := writeAdsEvalPayload(t, "keyword-ids.json", `[111111111]`)

	log := newRequestLog(5)
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		assertAdsEvalBearer(t, req)
		assertAdsEvalOrg(t, req)
		log.Add(req.Method + " " + req.URL.RequestURI())

		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/api/v5/campaigns":
			body := readAdsEvalJSONBody(t, req)
			if got := body["name"]; got != "ASC CLI Eval Campaign" {
				t.Fatalf("campaign create name = %#v, want ASC CLI Eval Campaign", got)
			}
			return adsJSONResponse(200, `{"data":{"id":1001}}`), nil
		case req.Method == http.MethodPut && req.URL.Path == "/api/v5/campaigns/1001":
			return adsJSONResponse(200, `{"data":{"id":1001,"status":"PAUSED"}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/api/v5/campaigns/1001/adgroups/2002/targetingkeywords/bulk":
			items := readAdsEvalJSONArrayBody(t, req)
			if len(items) != 1 {
				t.Fatalf("keyword create body length = %d, want 1", len(items))
			}
			return adsJSONResponse(200, `{"data":[{"id":111111111}]}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/api/v5/campaigns/1001/adgroups/2002/targetingkeywords/delete/bulk":
			items := readAdsEvalJSONArrayBody(t, req)
			if len(items) != 1 || items[0] != float64(111111111) {
				t.Fatalf("keyword delete body = %#v, want [111111111]", items)
			}
			return adsJSONResponse(200, `{"data":[111111111]}`), nil
		case req.Method == http.MethodDelete && req.URL.Path == "/api/v5/campaigns/1001":
			assertAdsEvalNoBody(t, req)
			return adsJSONResponse(204, ``), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	for _, tc := range []struct {
		args    []string
		warning string
	}{
		{args: []string{"ads", "v5", "campaigns", "create", "--file", campaignCreate, "--confirm", "--output", "json"}, warning: adsV5ReplacementWarning("v5 campaigns create", "campaigns create")},
		{args: []string{"ads", "v5", "campaigns", "update", "--campaign", "1001", "--file", campaignUpdate, "--confirm", "--output", "json"}, warning: adsV5ReplacementWarning("v5 campaigns update", "campaigns update")},
		{args: []string{"ads", "v5", "targeting-keywords", "create-bulk", "--campaign", "1001", "--ad-group", "2002", "--file", keywords, "--confirm", "--output", "json"}, warning: adsV5ReplacementWarning("v5 targeting-keywords create-bulk", "targeting-keywords create-bulk")},
		{args: []string{"ads", "v5", "targeting-keywords", "delete-bulk", "--campaign", "1001", "--ad-group", "2002", "--file", keywordIDs, "--confirm", "--output", "json"}, warning: adsV5NoReplacementWarning("v5 targeting-keywords delete-bulk", "No one-command replacement exists. Query matching keywords with `asc ads targeting-keywords find`, then delete each ID with `asc ads targeting-keywords delete --keyword KEYWORD_ID --confirm`.")},
		{args: []string{"ads", "v5", "campaigns", "delete", "--campaign", "1001", "--confirm", "--output", "json"}, warning: adsV5ReplacementWarning("v5 campaigns delete", "campaigns delete")},
	} {
		stdout, stderr, err := runAdsEvalCommand(t, tc.args...)
		if err != nil {
			t.Fatalf("asc %s error: %v\nstderr: %s", strings.Join(tc.args, " "), err, stderr)
		}
		if stderr != tc.warning {
			t.Fatalf("asc %s stderr = %q, want %q", strings.Join(tc.args, " "), stderr, tc.warning)
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
			t.Fatalf("asc %s stdout is not JSON: %v\n%s", strings.Join(tc.args, " "), err, stdout)
		}
	}

	joined := strings.Join(log.Snapshot(), "\n")
	for _, want := range []string{
		"POST /api/v5/campaigns",
		"PUT /api/v5/campaigns/1001",
		"POST /api/v5/campaigns/1001/adgroups/2002/targetingkeywords/bulk",
		"POST /api/v5/campaigns/1001/adgroups/2002/targetingkeywords/delete/bulk",
		"DELETE /api/v5/campaigns/1001",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("requests = %q, missing %q", joined, want)
		}
	}
}

func TestAdsAgentEvalRejectsArrayPayloadMistakesBeforeNetwork(t *testing.T) {
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_ORG_ID", "987654")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}))

	objectPayload := writeAdsEvalPayload(t, "keyword-object.json", `{"text":"not an array"}`)
	_, _, err := runAdsEvalCommand(
		t,
		"ads", "v5", "targeting-keywords", "create-bulk",
		"--campaign", "1001",
		"--ad-group", "2002",
		"--file", objectPayload,
		"--confirm",
		"--output", "json",
	)
	if err == nil || !strings.Contains(err.Error(), "payload must be a JSON array") {
		t.Fatalf("error = %v, want JSON array validation", err)
	}
}

func TestAdsAgentRawAPIEvalRequiresConfirmAndAcceptsAppleURL(t *testing.T) {
	t.Setenv("ASC_ADS_ACCESS_TOKEN", "ACCESS")
	t.Setenv("ASC_ADS_ORG_ID", "987654")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))

	log := newRequestLog(1)
	installDefaultTransport(t, adsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		assertAdsEvalBearer(t, req)
		assertAdsEvalOrg(t, req)
		log.Add(req.Method + " " + req.URL.RequestURI())
		if req.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", req.Method)
		}
		if got := req.URL.RawQuery; got != "audit=true" {
			t.Fatalf("query = %q, want audit=true", got)
		}
		assertAdsEvalNoBody(t, req)
		return adsJSONResponse(204, ``), nil
	}))
	warning := adsV5ReplacementWarning("v5 api request", "api request")

	_, stderr, err := runAdsEvalCommand(
		t,
		"ads", "v5", "api", "request",
		"--method", "DELETE",
		"--path", "https://api.searchads.apple.com/api/v5/campaigns/1001?audit=true",
		"--output", "json",
	)
	if !errors.Is(err, flag.ErrHelp) || !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("error = %v stderr = %q, want confirm usage error", err, stderr)
	}
	if got := strings.Count(stderr, warning); got != 1 {
		t.Fatalf("stderr = %q, want exactly one %q warning", stderr, warning)
	}
	if got := len(log.Snapshot()); got != 0 {
		t.Fatalf("requests before confirm = %d, want 0", got)
	}

	stdout, stderr, err := runAdsEvalCommand(
		t,
		"ads", "v5", "api", "request",
		"--method", "DELETE",
		"--path", "https://api.searchads.apple.com/api/v5/campaigns/1001?audit=true",
		"--confirm",
		"--output", "json",
	)
	if err != nil {
		t.Fatalf("confirmed raw delete error: %v\nstderr: %s", err, stderr)
	}
	if got, want := stderr, warning; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	var parsed struct {
		Data any `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if parsed.Data != nil {
		t.Fatalf("data = %#v, want nil", parsed.Data)
	}
	requests := log.Snapshot()
	if len(requests) != 1 || requests[0] != "DELETE /api/v5/campaigns/1001?audit=true" {
		t.Fatalf("requests = %#v", requests)
	}
}

func runAdsEvalCommand(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		root := RootCommand("dev")
		if err := root.Parse(args); err != nil {
			runErr = err
			return
		}
		runErr = root.Run(context.Background())
	})
	return stdout, stderr, runErr
}

func writeAdsEvalPayload(t *testing.T, name string, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	return path
}

func assertAdsEvalBearer(t *testing.T, req *http.Request) {
	t.Helper()

	if got := req.Header.Get("Authorization"); got != "Bearer ACCESS" {
		t.Fatalf("Authorization = %q, want Bearer ACCESS", got)
	}
}

func assertAdsEvalOrg(t *testing.T, req *http.Request) {
	t.Helper()

	want := "orgId=987654"
	if got := req.Header.Get("X-AP-Context"); got != want {
		t.Fatalf("X-AP-Context = %q, want %s", got, want)
	}
}

func assertAdsEvalNoOrg(t *testing.T, req *http.Request) {
	t.Helper()

	if got := req.Header.Get("X-AP-Context"); got != "" {
		t.Fatalf("X-AP-Context = %q, want empty", got)
	}
}

func assertAdsEvalNoBody(t *testing.T, req *http.Request) {
	t.Helper()

	if req.Body == nil {
		return
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if strings.TrimSpace(string(body)) != "" {
		t.Fatalf("body = %q, want empty", string(body))
	}
	if got := req.Header.Get("Content-Type"); got != "" {
		t.Fatalf("Content-Type = %q, want empty", got)
	}
}

func readAdsEvalJSONBody(t *testing.T, req *http.Request) map[string]any {
	t.Helper()

	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("body is not a JSON object: %v\n%s", err, body)
	}
	return parsed
}

func readAdsEvalJSONArrayBody(t *testing.T, req *http.Request) []any {
	t.Helper()

	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var parsed []any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("body is not a JSON array: %v\n%s", err, body)
	}
	return parsed
}
