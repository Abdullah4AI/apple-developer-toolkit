package assetlibrary

import "testing"

func TestLibraryFiltersOnlyOnMediaLists(t *testing.T) {
	for _, group := range Command().Subcommands {
		for _, child := range group.Subcommands {
			for _, name := range libraryListFilterFlags {
				// Instance --id selectors predate the new collection ID filter.
				if name == "id" && child.Name != "list" {
					continue
				}
				// Upload --category selects a destination; it is not a list filter.
				if name == "category" && child.Name == "upload" {
					continue
				}
				got := child.FlagSet.Lookup(name) != nil
				want := (group.Name == "images" || group.Name == "videos") && child.Name == "list"
				if got != want {
					t.Fatalf("%s %s --%s exposure=%v want%v", group.Name, child.Name, name, got, want)
				}
			}
		}
	}
}
