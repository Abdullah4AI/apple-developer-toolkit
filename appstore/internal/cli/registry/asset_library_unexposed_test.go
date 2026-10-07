package registry

import "testing"

func TestAssetLibraryIsRegistered(t *testing.T) {
	for _, cmd := range NewCatalog("dev").MetadataCommands() {
		if cmd.Name == "asset-library" {
			return
		}
	}
	t.Fatal("asset-library must be registered after live public endpoint verification")
}
