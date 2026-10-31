package iap

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

// IAPPricingCommand returns the canonical pricing command tree for IAPs.
func IAPPricingCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "pricing",
		ShortUsage: "asc iap pricing <subcommand> [flags]",
		ShortHelp:  "Manage in-app purchase pricing workflows.",
		LongHelp: `Manage in-app purchase pricing workflows.

Examples:
  asc iap pricing summary --app "APP_ID"
  asc iap pricing summary --iap-id "IAP_ID"
  asc iap pricing price-points list --iap-id "IAP_ID"
  asc iap pricing schedules view --iap-id "IAP_ID"
  asc iap pricing ppp --iap-id "IAP_ID" --base-price "4.99" --dry-run
  asc iap pricing availability view --iap-id "IAP_ID"
  asc iap pricing availabilities view --id "AVAILABILITY_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			IAPPricesCommand(),
			IAPPricePointsCommand(),
			IAPPriceSchedulesCommand(),
			IAPPricingPPPCommand(),
			IAPAvailabilityCommand(),
			IAPAvailabilitiesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
