package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// Apple's Developer Portal reports a Bundle ID's selected
// bundleIdCapabilities relationship with a paging total of 0 beside the
// linkage it returns, echoing the requested include limit, and the nested
// appGroups relationship of its APP_GROUPS capability with the same zero
// total and the unbounded limit the Services ID placeholder carries
// (both captured 2026-10-03):
//
//	"meta":{"paging":{"total":0,"limit":50}},"data":[...two references...],"links":{"self":...,"related":...}
//	"meta":{"paging":{"total":0,"limit":2147483647}},"data":[...one reference...],"links":{"self":...,"related":...}
const (
	developerBundleIDZeroTotalPaging  = `"meta":{"paging":{"total":0,"limit":50}},`
	developerAppGroupsZeroTotalPaging = `"meta":{"paging":{"total":0,"limit":2147483647}},`
)

func withZeroTotalCapabilityPaging(t *testing.T, bundle string) string {
	t.Helper()
	const relationship = `"bundleIdCapabilities":{"data":[`
	if !strings.Contains(bundle, relationship) {
		t.Fatalf("fixture has no bundleIdCapabilities linkage: %s", bundle)
	}
	bundle = strings.Replace(bundle, relationship, `"bundleIdCapabilities":{`+developerBundleIDZeroTotalPaging+`"data":[`, 1)
	return strings.ReplaceAll(bundle, `"appGroups":{"data":[`, `"appGroups":{`+developerAppGroupsZeroTotalPaging+`"data":[`)
}

func TestAssignDeveloperAppGroupAcceptsZeroTotalCapabilityPlaceholder(t *testing.T) {
	before := withZeroTotalCapabilityPaging(t, `{
		"data":{"id":"bundle-1","type":"bundleIds","attributes":{"name":"Example","identifier":"com.example.app","platform":"IOS"},"relationships":{"bundleIdCapabilities":{"data":[{"type":"bundleIdCapabilities","id":"push-1"}]}}},
		"included":[{"type":"bundleIdCapabilities","id":"push-1","attributes":{"enabled":true,"settings":[],"editable":true},"relationships":{"capability":{"data":{"type":"capabilities","id":"PUSH_NOTIFICATIONS"}}}}]
	}`)
	after := withZeroTotalCapabilityPaging(t, developerBundleAppGroupsFixture(true, "GROUP12345"))
	var patched bool
	client := newDeveloperAppGroupsTestClient(t, func(requestNumber int, request *http.Request) (*http.Response, error) {
		switch requestNumber {
		case 1:
			return assertDeveloperPortalBootstrap(t, request), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, before, nil), nil
		case 3:
			return developerPortalTestResponse(http.StatusOK, `{"resultCode":0,"applicationGroupList":[]}`, http.Header{"csrf": {"primed-csrf"}, "csrf_ts": {"primed-ts"}}), nil
		case 4:
			if request.Method != http.MethodPatch || request.URL.Path != "/services-account/v1/bundleIds/bundle-1" {
				t.Fatalf("unexpected bundle patch %s %s", request.Method, request.URL.String())
			}
			var payload developerBundleIDPatchRequest
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Fatalf("decode patch: %v", err)
			}
			var capabilities developerResourceRelationship
			if err := json.Unmarshal(payload.Data.Relationships["bundleIdCapabilities"], &capabilities); err != nil {
				t.Fatalf("decode capabilities: %v", err)
			}
			if len(capabilities.Data) != 2 || capabilities.Data[0].ID != "push-1" {
				t.Fatalf("PATCH must keep the existing capability and add APP_GROUPS, got %+v", capabilities.Data)
			}
			patched = true
			return developerPortalTestResponse(http.StatusOK, `{"data":{"type":"bundleIds","id":"bundle-1"}}`, nil), nil
		case 5:
			return developerPortalTestResponse(http.StatusOK, after, nil), nil
		default:
			t.Fatalf("unexpected request %d", requestNumber)
			return nil, nil
		}
	})

	result, err := client.AssignDeveloperAppGroup(context.Background(), DeveloperAppGroupAssignRequest{BundleID: "bundle-1", GroupID: "GROUP12345"})
	if err != nil {
		t.Fatalf("AssignDeveloperAppGroup() error: %v", err)
	}
	if !patched || !result.Changed || result.Status != "assigned" {
		t.Fatalf("unexpected result: %+v (patched %t)", result, patched)
	}
}

func TestValidateDeveloperRelationshipCompletenessZeroTotalPlaceholder(t *testing.T) {
	const requested = developerBundleIDCapabilitiesIncludeLimit
	tests := map[string]struct {
		relationship   string
		returned       int
		requestedLimit int
		wantErr        string
	}{
		"placeholder echoing the requested limit with room to spare": {
			relationship: `{"meta":{"paging":{"total":0,"limit":50}}}`, returned: 2, requestedLimit: requested,
		},
		"exact total still accepted": {
			relationship: `{"meta":{"paging":{"total":2,"limit":50}}}`, returned: 2, requestedLimit: requested,
		},
		"placeholder filling the requested limit may be truncated": {
			relationship: `{"meta":{"paging":{"total":0,"limit":50}}}`, returned: 50, requestedLimit: requested,
			wantErr: "returned 50 of 0 resources",
		},
		"placeholder with a limit other than the one requested": {
			relationship: `{"meta":{"paging":{"total":0,"limit":10}}}`, returned: 2, requestedLimit: requested,
			wantErr: "returned 2 of 0 resources",
		},
		"placeholder without a limit": {
			relationship: `{"meta":{"paging":{"total":0}}}`, returned: 2, requestedLimit: requested,
			wantErr: "returned 2 of 0 resources",
		},
		"placeholder with a continuation link": {
			relationship: `{"meta":{"paging":{"total":0,"limit":50}},"links":{"next":"https://developer.apple.com/next"}}`, returned: 2, requestedLimit: requested,
			wantErr: "is paginated and therefore incomplete",
		},
		"bounded placeholder where no limit was requested": {
			relationship: `{"meta":{"paging":{"total":0,"limit":50}}}`, returned: 2, requestedLimit: 0,
			wantErr: "returned 2 of 0 resources",
		},
		"unbounded placeholder where no limit was requested": {
			relationship: `{"meta":{"paging":{"total":0,"limit":2147483647}}}`, returned: 1, requestedLimit: 0,
		},
		"unbounded placeholder where a limit was requested": {
			relationship: `{"meta":{"paging":{"total":0,"limit":2147483647}}}`, returned: 2, requestedLimit: requested,
			wantErr: "returned 2 of 0 resources",
		},
		"unbounded placeholder with a continuation link": {
			relationship: `{"meta":{"paging":{"total":0,"limit":2147483647}},"links":{"next":"https://developer.apple.com/next"}}`, returned: 1,
			wantErr: "is paginated and therefore incomplete",
		},
		"real shortfall": {
			relationship: `{"meta":{"paging":{"total":3,"limit":50}}}`, returned: 2, requestedLimit: requested,
			wantErr: "returned 2 of 3 resources",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateDeveloperRelationshipCompleteness(json.RawMessage(test.relationship), test.returned, test.requestedLimit, "Bundle ID capability graph")
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("validateDeveloperRelationshipCompleteness() error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("validateDeveloperRelationshipCompleteness() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}
