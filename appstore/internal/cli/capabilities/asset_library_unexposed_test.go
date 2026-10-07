package capabilities

import (
	"slices"
	"strings"
	"testing"
)

func TestAssetLibraryCapabilitiesIncludeImageUpload(t *testing.T) {
	for _, c := range capabilityRows() {
		if c.Capability == "Asset Library media and specifications" {
			if c.Status != statusCLISupported || !strings.Contains(strings.Join(c.Notes, " "), "image upload") {
				t.Fatalf("incorrect scope: %+v", c)
			}
			return
		}
	}
	t.Fatal("missing verified Asset Library capability")
}

func TestAssetLibraryCapabilitiesExposeStandaloneReview(t *testing.T) {
	for _, c := range capabilityRows() {
		if c.Capability != "Asset Library media and specifications" {
			continue
		}
		for _, command := range []string{"asc review submissions-create", "asc review items add", "asc review submissions-submit --confirm"} {
			if !slices.Contains(c.Commands, command) {
				t.Errorf("missing review command %q: %+v", command, c)
			}
		}
		for _, resource := range []string{"appAssetLibraryImages", "appAssetLibraryVideos", "reviewSubmissions", "reviewSubmissionItems"} {
			if !slices.Contains(c.APIResources, resource) {
				t.Errorf("missing review resource %q: %+v", resource, c)
			}
		}
		notes := strings.Join(c.Notes, " ")
		// Keep the review prerequisite discoverable without snapshotting audit prose.
		if strings.Contains(notes, "Review submission is not exposed") || !strings.Contains(notes, "approved app version") {
			t.Errorf("missing standalone review prerequisite or misleading availability: %s", notes)
		}
		return
	}
	t.Fatal("missing Asset Library capability")
}
