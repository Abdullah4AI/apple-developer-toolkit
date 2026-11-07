package gamecenter

import (
	"flag"
	"fmt"
	"os"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

// validateGameCenterReplacementConfirm enforces --confirm on relationship
// replacements, which can drop existing relationships.
func validateGameCenterReplacementConfirm(fs *flag.FlagSet, confirmed bool) error {
	if confirmed {
		return nil
	}
	if flagPassed(fs, "confirm") {
		return shared.UsageError("--confirm must be true when specified")
	}
	fmt.Fprintln(os.Stderr, "Error: --confirm is required")
	return shared.MissingRequiredUsageError("--confirm")
}
