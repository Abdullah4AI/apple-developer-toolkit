package metadata

import "github.com/peterbourgon/ff/v3/ffcli"

// MetadataApplyCommand returns the canonical apply alias for metadata push.
func MetadataApplyCommand() *ffcli.Command {
	return newMetadataMutationCommand(metadataMutationCommandConfig{
		name:      "apply",
		verbTitle: "Apply",
	})
}
