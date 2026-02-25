package signing

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

// SigningCommand returns the signing command with subcommands.
func SigningCommand() *ffcli.Command {
	fs := flag.NewFlagSet("signing", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "signing",
		ShortUsage: "asc signing <subcommand> [flags]",
		ShortHelp:  "Manage signing certificates and profiles.",
		LongHelp: `Manage signing assets in App Store Connect.

Examples:
  asc signing fetch --bundle-id com.example.app --profile-type IOS_APP_STORE --output ./signing`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SigningFetchCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
