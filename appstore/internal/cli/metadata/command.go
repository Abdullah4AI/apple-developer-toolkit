package metadata

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

// MetadataCommand returns the metadata command group.
func MetadataCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metadata", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "metadata",
		ShortUsage: "asc metadata <subcommand> [flags]",
		ShortHelp:  "Manage app metadata with deterministic file workflows.",
		LongHelp: `Manage app metadata with deterministic file workflows.

Phase 1 scope:
  - app-info localizations: name, subtitle, privacyPolicyUrl, privacyChoicesUrl, privacyPolicyText
  - version localizations: description, keywords, marketingUrl, promotionalText, supportUrl, whatsNew

Not yet included in this group:
  - categories, copyright, review information, age ratings, screenshots

Examples:
  asc metadata pull --app "APP_ID" --version "1.2.3" --dir "./metadata"
  asc metadata pull --app "APP_ID" --version "1.2.3" --platform IOS --dir "./metadata"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			MetadataPullCommand(),
			MetadataPushCommand(),
			MetadataValidateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
