package migrate

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v3/ffcli"

	metadatacmd "github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/metadata"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

// MigrateMetadataCommand provides migration-friendly aliases for metadata workflows.
func MigrateMetadataCommand() *ffcli.Command {
	fs := flag.NewFlagSet("migrate metadata", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "metadata",
		ShortUsage: "asc migrate metadata <pull|push|validate> [flags]",
		ShortHelp:  "Compatibility aliases for asc metadata commands.",
		LongHelp: `Compatibility aliases for asc metadata commands.

These aliases help teams move from fastlane/deliver conventions while
adopting native asc metadata workflows.

Prefer direct commands for new scripts:
  asc metadata pull ...
  asc metadata push ...
  asc metadata validate ...`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			metadatacmd.MetadataPullCommand(),
			metadatacmd.MetadataPushCommand(),
			metadatacmd.MetadataValidateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			fmt.Fprintln(os.Stderr, "Tip: use `asc metadata ...`; `asc migrate metadata ...` is a compatibility alias.")
			return flag.ErrHelp
		},
	}
}
