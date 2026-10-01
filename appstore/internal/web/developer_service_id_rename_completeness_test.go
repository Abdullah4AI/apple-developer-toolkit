package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// serviceIDCapturedEmptyCapabilitiesRelationship returns the
// data.relationships.bundleIdCapabilities object Apple returned for a Services
// ID with zero capabilities (live capture, 2026-09-28). The captured resource
// ID in the relationship links was replaced with the synthetic "service-1";
// nothing else was changed.
func serviceIDCapturedEmptyCapabilitiesRelationship(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "service_id_empty_capabilities_relationship.json"))
	if err != nil {
		t.Fatalf("read captured empty capability fixture: %v", err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		t.Fatalf("compact captured empty capability fixture: %v", err)
	}
	return compact.String()
}

const serviceIDRenameTwoCapabilityReferences = `[{"type":"bundleIdCapabilities","id":"cap-1"},{"type":"bundleIdCapabilities","id":"cap-2"}]`

func serviceIDRenameRelationships(capabilityRelationship string) string {
	return `{"bundleIdCapabilities":` + capabilityRelationship + `}`
}

func serviceIDRenameTwoCapabilityRelationship(extra string) string {
	return `{"data":` + serviceIDRenameTwoCapabilityReferences + extra + `}`
}

// renameServiceIDWithRelationships runs a rename whose preflight and
// post-write reads carry the given bundleIdCapabilities relationships. It
// reports the receipt, the error, the number of PATCH requests, and the PATCH's
// capability relationship.
func renameServiceIDWithRelationships(t *testing.T, preflight, postRead string) (verified bool, patches int, sent string, err error) {
	t.Helper()
	var requests int
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		requests++
		if r.Method == http.MethodPatch {
			patches++
			var payload developerBundleIDPatchRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode rename payload: %v", err)
			}
			sent = string(payload.Data.Relationships["bundleIdCapabilities"])
			return developerPortalTestResponse(http.StatusOK, `{}`, nil), nil
		}
		switch requests {
		case 1:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), http.Header{"csrf": {"csrf"}, "csrf_ts": {"csrf-ts"}}), nil
		case 2:
			return developerPortalTestResponse(http.StatusOK, serviceIDDetailFixtureWithRelationships("Old Name", "SERVICES", serviceIDRenameRelationships(preflight)), nil), nil
		case 4:
			return developerPortalTestResponse(http.StatusOK, serviceIDDetailFixtureWithRelationships("New Name", "SERVICES", serviceIDRenameRelationships(postRead)), nil), nil
		default:
			t.Fatalf("unexpected request %d: %s %s", requests, r.Method, r.URL.String())
			return nil, nil
		}
	})
	result, err := client.RenameDeveloperServiceID(context.Background(), DeveloperServiceIDRenameRequest{ServiceID: "service-1", Name: "New Name"})
	return result != nil && result.Verified, patches, sent, err
}

func TestRenameDeveloperServiceIDRejectsUnprovenCapabilityCompletenessBeforeMutation(t *testing.T) {
	tests := []struct {
		name         string
		relationship string
		wantErr      string
	}{
		{"truncated linkage without meta", serviceIDRenameTwoCapabilityRelationship(``), "no paging metadata"},
		{"meta without paging", serviceIDRenameTwoCapabilityRelationship(`,"meta":{"opaque":"keep"}`), "no paging metadata"},
		{"missing total", serviceIDRenameTwoCapabilityRelationship(`,"meta":{"paging":{"limit":2147483647}}`), "paging total"},
		{"limit below linkage count", serviceIDRenameTwoCapabilityRelationship(`,"meta":{"paging":{"total":2,"limit":1}}`), "beyond its paging limit"},
		{"next link present", serviceIDRenameTwoCapabilityRelationship(`,"links":{"next":"/bundleIds/service-1/relationships/bundleIdCapabilities?cursor=2"},"meta":{"paging":{"total":2,"limit":2147483647}}`), "paginated"},
		{"positive total mismatch", serviceIDRenameTwoCapabilityRelationship(`,"meta":{"paging":{"total":3,"limit":2147483647}}`), "returned 2 of 3"},
		{"placeholder with non-maximum limit", serviceIDRenameTwoCapabilityRelationship(`,"meta":{"paging":{"total":0,"limit":50}}`), "returned 2 of 0"},
		{"empty linkage without meta", `{"data":[]}`, "no paging metadata"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			verified, patches, _, err := renameServiceIDWithRelationships(t, tc.relationship, tc.relationship)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) || !strings.Contains(err.Error(), "cannot safely rename") {
				t.Fatalf("RenameDeveloperServiceID() error = %v, want %q before mutation", err, tc.wantErr)
			}
			if verified {
				t.Fatal("RenameDeveloperServiceID() returned a verified receipt")
			}
			if patches != 0 {
				t.Fatalf("PATCH requests = %d, want none", patches)
			}
		})
	}
}

func TestRenameDeveloperServiceIDAcceptsProvenCapabilityCompleteness(t *testing.T) {
	tests := []struct {
		name         string
		relationship func(*testing.T) string
	}{
		{"captured placeholder", func(*testing.T) string {
			return serviceIDRenameTwoCapabilityRelationship(`,"meta":{"paging":{"total":0,"limit":2147483647}}`)
		}},
		{"exact total", func(*testing.T) string {
			return serviceIDRenameTwoCapabilityRelationship(`,"meta":{"paging":{"total":2,"limit":50}}`)
		}},
		{"captured empty capabilities", serviceIDCapturedEmptyCapabilitiesRelationship},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			relationship := tc.relationship(t)
			verified, patches, sent, err := renameServiceIDWithRelationships(t, relationship, relationship)
			if err != nil {
				t.Fatalf("RenameDeveloperServiceID() error: %v", err)
			}
			if !verified {
				t.Fatal("RenameDeveloperServiceID() returned an unverified receipt")
			}
			if patches != 1 {
				t.Fatalf("PATCH requests = %d, want 1", patches)
			}
			if sent != relationship {
				t.Fatalf("PATCH capability relationship = %s, want unchanged %s", sent, relationship)
			}
		})
	}
}

func TestRenameDeveloperServiceIDRejectsUnprovenCapabilityCompletenessAfterMutation(t *testing.T) {
	preflight := serviceIDRenameTwoCapabilityRelationship(`,"meta":{"paging":{"total":2,"limit":2147483647}}`)
	tests := []struct {
		name     string
		postRead string
		wantErr  string
	}{
		{"truncated linkage without meta", serviceIDRenameTwoCapabilityRelationship(``), "no paging metadata"},
		{"missing total", serviceIDRenameTwoCapabilityRelationship(`,"meta":{"paging":{"limit":2147483647}}`), "paging total"},
		{"limit below linkage count", serviceIDRenameTwoCapabilityRelationship(`,"meta":{"paging":{"total":2,"limit":1}}`), "beyond its paging limit"},
		{"next link present", serviceIDRenameTwoCapabilityRelationship(`,"links":{"next":"/next"},"meta":{"paging":{"total":2,"limit":2147483647}}`), "paginated"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			verified, patches, _, err := renameServiceIDWithRelationships(t, preflight, tc.postRead)
			var unverified *DeveloperServiceIDUnverifiedError
			if !errors.As(err, &unverified) || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("RenameDeveloperServiceID() error = %v, want unverified %q", err, tc.wantErr)
			}
			if verified {
				t.Fatal("RenameDeveloperServiceID() returned a verified receipt")
			}
			if patches != 1 {
				t.Fatalf("PATCH requests = %d, want exactly 1 with no retry", patches)
			}
		})
	}
}
