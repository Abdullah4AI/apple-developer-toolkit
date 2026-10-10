package categories

import (
	"testing"
)

func TestCategoriesSetCommand_SubcategoryFlagsRegistered(t *testing.T) {
	cmd := CategoriesSetCommand()

	for _, name := range []string{
		"primary-subcategory-one",
		"primary-subcategory-two",
		"secondary-subcategory-one",
		"secondary-subcategory-two",
	} {
		if cmd.FlagSet.Lookup(name) == nil {
			t.Fatalf("expected --%s flag to be registered", name)
		}
	}
}
