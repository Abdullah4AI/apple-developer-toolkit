package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

const iCloudCreateTestIdentifier = "iCloud.com.example.app"

// iCloudCreatePortal is an httptest Developer Portal that serves the team
// bootstrap, the visible and hidden cloudContainers collections, and the
// cloudContainers create POST. Every request is recorded.
type iCloudCreatePortal struct {
	t        *testing.T
	server   *httptest.Server
	mu       sync.Mutex
	requests []string

	visible    []string
	hidden     []string
	meta       string
	createHook func(w http.ResponseWriter, r *http.Request, body []byte)
	creates    int
	teamID     string
}

func newICloudCreatePortal(t *testing.T) *iCloudCreatePortal {
	t.Helper()
	p := &iCloudCreatePortal{t: t, teamID: "TEAM123456"}
	p.server = httptest.NewServer(http.HandlerFunc(p.serve))
	t.Cleanup(p.server.Close)
	return p
}

func (p *iCloudCreatePortal) client() *Client {
	return &Client{httpClient: p.server.Client(), developerPortalURL: p.server.URL}
}

func (p *iCloudCreatePortal) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	override := r.Header.Get("X-HTTP-Method-Override")
	// Hold the lock for the whole request: hooks mutate the collections
	// that later requests and the test goroutine read.
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests = append(p.requests, fmt.Sprintf("%s %s override=%s hidden=%s", r.Method, r.URL.Path, override, r.URL.Query().Get("filter[AND][hidden]")))
	switch {
	case r.Method == http.MethodPost && r.URL.Path == developerPortalTeamsPath:
		w.Header().Set("csrf", "csrf-token")
		w.Header().Set("csrf_ts", "csrf-ts")
		_, _ = fmt.Fprintf(w, `{"teams":[{"teamId":%q,"name":"Example Team","status":"active"}]}`, p.teamID)
	case r.Method == http.MethodPost && r.URL.Path == developerServicesPath+"/cloudContainers" && override == http.MethodGet:
		var request developerPortalProxyReadRequest
		if err := json.Unmarshal(body, &request); err != nil || request.TeamID != p.teamID {
			p.t.Errorf("list body = %s (err %v), want teamId %s", body, err, p.teamID)
		}
		rows := p.visible
		if r.URL.Query().Get("filter[AND][hidden]") == "true" {
			rows = p.hidden
		}
		meta := ""
		if p.meta != "" {
			meta = `,"meta":` + p.meta
		}
		_, _ = fmt.Fprintf(w, `{"data":[%s],"links":{"self":"/cloudContainers"}%s}`, strings.Join(rows, ","), meta)
	case r.Method == http.MethodPost && r.URL.Path == developerServicesPath+"/cloudContainers" && override == "":
		p.creates++
		if p.createHook == nil {
			p.t.Errorf("unexpected create POST: %s", body)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		p.createHook(w, r, body)
	default:
		p.t.Errorf("unexpected request %s %s override=%q", r.Method, r.URL.String(), override)
		w.WriteHeader(http.StatusNotFound)
	}
}

func (p *iCloudCreatePortal) createCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.creates
}

func (p *iCloudCreatePortal) recorded() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.requests...)
}

func iCloudContainerFixture(id, identifier, name string, hidden bool) string {
	return fmt.Sprintf(`{"type":"cloudContainers","id":%q,"attributes":{"identifier":%q,"hidden":%t,"prefix":"TEAM123456","canEdit":true,"name":%q,"canDelete":false,"responseId":"response-1"}}`, id, identifier, hidden, name)
}

func TestCreateDeveloperICloudContainerSendsJSONAPIPayloadAndVerifiesReadBack(t *testing.T) {
	portal := newICloudCreatePortal(t)
	portal.visible = []string{iCloudContainerFixture("cloud-0", "iCloud.com.example.other", "Other", false)}
	portal.createHook = func(w http.ResponseWriter, r *http.Request, body []byte) {
		if r.Header.Get("csrf") != "csrf-token" || r.Header.Get("csrf_ts") != "csrf-ts" {
			t.Errorf("create CSRF headers = %q/%q", r.Header.Get("csrf"), r.Header.Get("csrf_ts"))
		}
		if got := r.Header.Get("Content-Type"); got != "application/vnd.api+json" {
			t.Errorf("create Content-Type = %q", got)
		}
		var root map[string]json.RawMessage
		if err := json.Unmarshal(body, &root); err != nil {
			t.Fatalf("decode create body: %v", err)
		}
		if len(root) != 1 || root["data"] == nil {
			t.Errorf("create root members = %s, want only data", body)
		}
		var payload struct {
			Data map[string]json.RawMessage `json:"data"`
		}
		_ = json.Unmarshal(body, &payload)
		if len(payload.Data) != 2 || string(payload.Data["type"]) != `"cloudContainers"` {
			t.Errorf("create data = %s, want type and attributes only", body)
		}
		var attributes map[string]string
		if err := json.Unmarshal(payload.Data["attributes"], &attributes); err != nil {
			t.Fatalf("decode attributes: %v", err)
		}
		want := map[string]string{"identifier": iCloudCreateTestIdentifier, "name": "Example Container", "teamId": "TEAM123456"}
		if !mapsEqual(attributes, want) {
			t.Errorf("attributes = %v, want %v", attributes, want)
		}
		portal.visible = append(portal.visible, iCloudContainerFixture("cloud-1", iCloudCreateTestIdentifier, "Example Container", false))
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"data":`+iCloudContainerFixture("cloud-1", iCloudCreateTestIdentifier, "Example Container", false)+`}`)
	}

	result, err := portal.client().CreateDeveloperICloudContainer(context.Background(), DeveloperICloudContainerCreateRequest{
		Identifier: " " + iCloudCreateTestIdentifier + " ",
		Name:       " Example Container ",
	})
	if err != nil {
		t.Fatalf("CreateDeveloperICloudContainer() error: %v", err)
	}
	if result.Operation != "create" || result.ContainerID != "cloud-1" || result.Identifier != iCloudCreateTestIdentifier ||
		result.Name != "Example Container" || result.RequestedName != "" || result.Prefix != "TEAM123456" ||
		!result.Changed || !result.Verified || !result.Permanent || result.Status != "created" {
		t.Fatalf("unexpected receipt: %+v", result)
	}
	wantRequests := []string{
		"POST " + developerPortalTeamsPath + " override= hidden=",
		"POST /services-account/v1/cloudContainers override=GET hidden=false",
		"POST /services-account/v1/cloudContainers override=GET hidden=true",
		"POST /services-account/v1/cloudContainers override= hidden=",
		"POST /services-account/v1/cloudContainers override=GET hidden=false",
		"POST /services-account/v1/cloudContainers override=GET hidden=true",
	}
	if strings.Join(portal.recorded(), "\n") != strings.Join(wantRequests, "\n") {
		t.Fatalf("requests =\n%s\nwant\n%s", strings.Join(portal.recorded(), "\n"), strings.Join(wantRequests, "\n"))
	}
}

func TestCreateDeveloperICloudContainerConvergesWhenCreateBodyIsEmpty(t *testing.T) {
	portal := newICloudCreatePortal(t)
	portal.createHook = func(w http.ResponseWriter, r *http.Request, body []byte) {
		portal.visible = []string{iCloudContainerFixture("cloud-1", iCloudCreateTestIdentifier, "Apple Name", false)}
		w.WriteHeader(http.StatusCreated)
	}
	result, err := portal.client().CreateDeveloperICloudContainer(context.Background(), DeveloperICloudContainerCreateRequest{Identifier: iCloudCreateTestIdentifier, Name: "Example Container"})
	if err != nil {
		t.Fatalf("CreateDeveloperICloudContainer() error: %v", err)
	}
	if result.ContainerID != "cloud-1" || result.Name != "Apple Name" || result.RequestedName != "Example Container" || !result.Verified {
		t.Fatalf("unexpected receipt: %+v", result)
	}
}

func TestCreateDeveloperICloudContainerRefusesExistingIdentifierBeforePOST(t *testing.T) {
	for _, hidden := range []bool{false, true} {
		t.Run(fmt.Sprintf("hidden=%t", hidden), func(t *testing.T) {
			portal := newICloudCreatePortal(t)
			existing := iCloudContainerFixture("cloud-9", iCloudCreateTestIdentifier, "Existing", hidden)
			if hidden {
				portal.hidden = []string{existing}
			} else {
				portal.visible = []string{existing}
			}
			_, err := portal.client().CreateDeveloperICloudContainer(context.Background(), DeveloperICloudContainerCreateRequest{Identifier: iCloudCreateTestIdentifier, Name: "Example"})
			if err == nil || !strings.Contains(err.Error(), "already exists") || !strings.Contains(err.Error(), "cloud-9") {
				t.Fatalf("error = %v, want already exists with id", err)
			}
			if portal.createCount() != 0 {
				t.Fatalf("create POSTs = %d, want 0", portal.createCount())
			}
		})
	}
}

func TestCreateDeveloperICloudContainerRefusesIncompletePreflight(t *testing.T) {
	portal := newICloudCreatePortal(t)
	portal.meta = `{"paging":{"total":1001,"limit":1000}}`
	_, err := portal.client().CreateDeveloperICloudContainer(context.Background(), DeveloperICloudContainerCreateRequest{Identifier: iCloudCreateTestIdentifier, Name: "Example"})
	if err == nil || !strings.Contains(err.Error(), "cannot safely preflight") || !strings.Contains(err.Error(), "reports 1001") {
		t.Fatalf("error = %v, want incomplete preflight refusal", err)
	}
	if portal.createCount() != 0 {
		t.Fatalf("create POSTs = %d, want 0", portal.createCount())
	}
}

func TestCreateDeveloperICloudContainerReportsRejectedCreateWithoutReadBack(t *testing.T) {
	portal := newICloudCreatePortal(t)
	portal.createHook = func(w http.ResponseWriter, r *http.Request, body []byte) {
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, `{"errors":[{"status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"An attribute value is invalid."}]}`)
	}
	_, err := portal.client().CreateDeveloperICloudContainer(context.Background(), DeveloperICloudContainerCreateRequest{Identifier: iCloudCreateTestIdentifier, Name: "Example"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusConflict {
		t.Fatalf("error = %v, want APIError 409", err)
	}
	var unverified *DeveloperICloudContainerUnverifiedError
	if errors.As(err, &unverified) {
		t.Fatalf("a rejected create must not be reported as unverified: %v", err)
	}
	if !strings.Contains(err.Error(), "ENTITY_ERROR.ATTRIBUTE.INVALID") {
		t.Fatalf("error = %v, want Apple error code", err)
	}
	if got := len(portal.recorded()); got != 4 {
		t.Fatalf("requests = %d (%v), want bootstrap, two preflight lists, and the POST only", got, portal.recorded())
	}
}

func TestCreateDeveloperICloudContainerReportsServerErrorAsUnverified(t *testing.T) {
	portal := newICloudCreatePortal(t)
	portal.createHook = func(w http.ResponseWriter, r *http.Request, body []byte) {
		w.WriteHeader(http.StatusBadGateway)
	}
	_, err := portal.client().CreateDeveloperICloudContainer(context.Background(), DeveloperICloudContainerCreateRequest{Identifier: iCloudCreateTestIdentifier, Name: "Example"})
	var unverified *DeveloperICloudContainerUnverifiedError
	if !errors.As(err, &unverified) || !strings.Contains(err.Error(), "asc web icloud-containers list") {
		t.Fatalf("error = %v, want unverified with list hint", err)
	}
	if portal.createCount() != 1 {
		t.Fatalf("create POSTs = %d, want exactly 1 (no retry)", portal.createCount())
	}
}

func TestCreateDeveloperICloudContainerReportsMissingReadBackAsUnverified(t *testing.T) {
	portal := newICloudCreatePortal(t)
	portal.createHook = func(w http.ResponseWriter, r *http.Request, body []byte) {
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"data":`+iCloudContainerFixture("cloud-1", iCloudCreateTestIdentifier, "Example", false)+`}`)
	}
	_, err := portal.client().CreateDeveloperICloudContainer(context.Background(), DeveloperICloudContainerCreateRequest{Identifier: iCloudCreateTestIdentifier, Name: "Example"})
	var unverified *DeveloperICloudContainerUnverifiedError
	if !errors.As(err, &unverified) || !strings.Contains(err.Error(), "found 0 containers") {
		t.Fatalf("error = %v, want unverified read-back", err)
	}
	if portal.createCount() != 1 {
		t.Fatalf("create POSTs = %d, want exactly 1", portal.createCount())
	}
}

func TestCreateDeveloperICloudContainerRejectsReadBackIDMismatch(t *testing.T) {
	portal := newICloudCreatePortal(t)
	portal.createHook = func(w http.ResponseWriter, r *http.Request, body []byte) {
		portal.visible = []string{iCloudContainerFixture("cloud-2", iCloudCreateTestIdentifier, "Example", false)}
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"data":`+iCloudContainerFixture("cloud-1", iCloudCreateTestIdentifier, "Example", false)+`}`)
	}
	_, err := portal.client().CreateDeveloperICloudContainer(context.Background(), DeveloperICloudContainerCreateRequest{Identifier: iCloudCreateTestIdentifier, Name: "Example"})
	var unverified *DeveloperICloudContainerUnverifiedError
	if !errors.As(err, &unverified) || !strings.Contains(err.Error(), "cloud-2") {
		t.Fatalf("error = %v, want unverified id mismatch", err)
	}
}

func TestCreateDeveloperICloudContainerValidatesBeforeAnyRequest(t *testing.T) {
	portal := newICloudCreatePortal(t)
	for _, tc := range []struct {
		identifier, name, want string
	}{
		{"com.example.app", "Example", `must start with "iCloud."`},
		{"icloud.com.example.app", "Example", `use "iCloud.com.example.app"`},
		{"iCloud.", "Example", "reverse-DNS string"},
		{"iCloud.com.example app", "Example", "only letters, digits"},
		{"iCloud.com_example.app", "Example", "only letters, digits"},
		{"iCloud..com.example", "Example", "empty dot-separated"},
		{"iCloud.com.example.", "Example", "empty dot-separated"},
		{iCloudCreateTestIdentifier, "  ", "--name is required"},
	} {
		_, err := portal.client().CreateDeveloperICloudContainer(context.Background(), DeveloperICloudContainerCreateRequest{Identifier: tc.identifier, Name: tc.name})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("identifier %q name %q: error = %v, want %q", tc.identifier, tc.name, err, tc.want)
		}
	}
	if len(portal.recorded()) != 0 {
		t.Fatalf("validation sent requests: %v", portal.recorded())
	}
}

func TestValidateDeveloperICloudContainerIdentifierAcceptsReverseDNS(t *testing.T) {
	for _, identifier := range []string{"iCloud.com.example.app", "iCloud.com.example-team.App2", "iCloud.a"} {
		if err := ValidateDeveloperICloudContainerIdentifier(identifier); err != nil {
			t.Errorf("ValidateDeveloperICloudContainerIdentifier(%q) = %v", identifier, err)
		}
	}
}

// iCloudContainerCreateCapture is the live create accepted by Apple on
// 2026-09-29 (testdata/icloud_container_create_capture.json). Its provenance
// block records which fields were captured and which are inferred.
type iCloudContainerCreateCapture struct {
	Request struct {
		Method string          `json:"method"`
		Path   string          `json:"path"`
		Body   json.RawMessage `json:"body"`
	} `json:"request"`
	CreateStatus     int             `json:"createStatus"`
	ReadBackResource json.RawMessage `json:"readBackResource"`
}

func loadICloudContainerCreateCapture(t *testing.T) iCloudContainerCreateCapture {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "icloud_container_create_capture.json"))
	if err != nil {
		t.Fatalf("read iCloud container create capture: %v", err)
	}
	var capture iCloudContainerCreateCapture
	if err := json.Unmarshal(raw, &capture); err != nil {
		t.Fatalf("decode iCloud container create capture: %v", err)
	}
	return capture
}

func TestCreateDeveloperICloudContainerMatchesLiveCapture(t *testing.T) {
	capture := loadICloudContainerCreateCapture(t)
	var want any
	if err := json.Unmarshal(capture.Request.Body, &want); err != nil {
		t.Fatalf("decode captured request body: %v", err)
	}
	for _, responseBody := range []struct{ name, body string }{
		{"inferred resource body", `{"data":` + string(capture.ReadBackResource) + `}`},
		{"empty body", ""},
	} {
		t.Run(responseBody.name, func(t *testing.T) {
			portal := newICloudCreatePortal(t)
			portal.createHook = func(w http.ResponseWriter, r *http.Request, body []byte) {
				if r.Method != capture.Request.Method || r.URL.Path != capture.Request.Path {
					t.Errorf("create transport = %s %s, want captured %s %s", r.Method, r.URL.Path, capture.Request.Method, capture.Request.Path)
				}
				var got any
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("decode create body: %v", err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("create body = %s, want captured %s", body, capture.Request.Body)
				}
				portal.visible = []string{string(capture.ReadBackResource)}
				w.WriteHeader(capture.CreateStatus)
				_, _ = io.WriteString(w, responseBody.body)
			}
			portal.teamID = "YQZQG7N4WG"
			result, err := portal.client().CreateDeveloperICloudContainer(context.Background(), DeveloperICloudContainerCreateRequest{
				Identifier: "iCloud.com.rorkai.asc.capture.t1790620906",
				Name:       "asc capture container",
			})
			if err != nil {
				t.Fatalf("CreateDeveloperICloudContainer() error: %v", err)
			}
			if result.ContainerID != "7PU5FAD3YR" || result.Prefix != "YQZQG7N4WG" || result.Name != "asc capture container" ||
				result.RequestedName != "" || result.Hidden || !result.Verified || !result.Permanent || result.Status != "created" {
				t.Fatalf("unexpected receipt: %+v", result)
			}
		})
	}
}

func TestCreateDeveloperICloudContainerRejectsContradictoryCreateResponse(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"wrong type", `{"data":{"type":"bundleIds","id":"cloud-1","attributes":{"identifier":"` + iCloudCreateTestIdentifier + `"}}}`, "want cloudContainers"},
		{"wrong identifier", `{"data":` + iCloudContainerFixture("cloud-1", "iCloud.com.example.other", "Example", false) + `}`, "iCloud.com.example.other"},
		{"not JSON", `<html>`, "failed to parse create response"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			portal := newICloudCreatePortal(t)
			portal.createHook = func(w http.ResponseWriter, r *http.Request, body []byte) {
				portal.visible = []string{iCloudContainerFixture("cloud-1", iCloudCreateTestIdentifier, "Example", false)}
				w.WriteHeader(http.StatusCreated)
				_, _ = io.WriteString(w, tc.body)
			}
			result, err := portal.client().CreateDeveloperICloudContainer(context.Background(), DeveloperICloudContainerCreateRequest{Identifier: iCloudCreateTestIdentifier, Name: "Example"})
			var unverified *DeveloperICloudContainerUnverifiedError
			if result != nil || !errors.As(err, &unverified) || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "cloud-1") {
				t.Fatalf("result = %+v, error = %v; want unverified naming %q and the read-back id", result, err, tc.want)
			}
			if portal.createCount() != 1 {
				t.Fatalf("create POSTs = %d, want 1", portal.createCount())
			}
		})
	}
}
