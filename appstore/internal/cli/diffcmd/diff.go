package diffcmd

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

// DiffCommand returns the root diff command group.
func DiffCommand() *ffcli.Command {
	fs := flag.NewFlagSet("diff", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "diff",
		ShortUsage: "asc diff <subcommand> [flags]",
		ShortHelp:  "Generate deterministic non-mutating diff plans.",
		LongHelp: `Generate deterministic non-mutating diff plans.

Examples:
  asc diff localizations --app "APP_ID" --path "./metadata/localizations" --version "VERSION_ID"
  asc diff localizations --app "APP_ID" --from-version "VERSION_ID_A" --to-version "VERSION_ID_B"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			DiffLocalizationsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
