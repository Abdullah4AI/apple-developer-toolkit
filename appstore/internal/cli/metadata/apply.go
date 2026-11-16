package metadata

import (
	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

// newMetadataApplyCommand returns the canonical apply alias for metadata push.
func newMetadataApplyCommand(d *shared.Deps) *ffcli.Command {
	return newMetadataMutationCommand(d, metadataMutationCommandConfig{
		name:      "apply",
		verbTitle: "Apply",
	})
}
