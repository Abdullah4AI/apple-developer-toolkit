package alternativedistribution

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

// AlternativeDistributionCommand returns the alternative distribution command group.
func AlternativeDistributionCommand() *ffcli.Command {
	fs := flag.NewFlagSet("alternative-distribution", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "alternative-distribution",
		ShortUsage: "asc alternative-distribution <subcommand> [flags]",
		ShortHelp:  "Manage alternative distribution resources.",
		LongHelp: `Manage alternative distribution resources.

Examples:
  asc alternative-distribution domains list
  asc alternative-distribution domains create --domain "example.com" --reference-name "Example"
  asc alternative-distribution keys list
  asc alternative-distribution keys create --app "APP_ID" --public-key-path "./key.pem"
  asc alternative-distribution packages get --package-id "PACKAGE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AlternativeDistributionDomainsCommand(),
			AlternativeDistributionKeysCommand(),
			AlternativeDistributionPackagesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
