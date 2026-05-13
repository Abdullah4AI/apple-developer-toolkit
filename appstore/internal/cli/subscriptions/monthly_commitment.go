package subscriptions

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/ascterritory"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

type subscriptionBillingMode string

const (
	subscriptionBillingModeUpfront           subscriptionBillingMode = "upfront"
	subscriptionBillingModeMonthlyCommitment subscriptionBillingMode = "monthly-commitment"
)

var errMonthlyCommitmentPublicAPINotAvailable = errors.New("monthly subscriptions with a 12-month commitment are not yet supported by Apple's public App Store Connect API; Apple documents the App Store Connect web UI flow, but the public OpenAPI schema currently has no billing-mode field or resource for subscriptionAvailabilities or subscriptionPrices")

var monthlyCommitmentExcludedTerritories = map[string]struct{}{
	"USA": {},
	"SGP": {},
}

// SubscriptionsPricingMonthlyCommitmentCommand returns the monthly-commitment command group.
func SubscriptionsPricingMonthlyCommitmentCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing monthly-commitment", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "monthly-commitment",
		ShortUsage: "asc subscriptions pricing monthly-commitment <subcommand> [flags]",
		ShortHelp:  "[experimental] Prepare Monthly with 12-Month Commitment subscription billing.",
		LongHelp: `[experimental] Prepare Monthly with 12-Month Commitment subscription billing.

Apple announced Monthly Subscriptions with a 12-Month Commitment on April 27, 2026.
The public App Store Connect API currently exposes standard subscription
availability and prices only; these commands validate the supported inputs and
return a clear upstream-API error until Apple publishes the billing-mode fields.

Examples:
  asc subscriptions pricing monthly-commitment enable --subscription-id "SUB_ID" --price "9.99" --price-territory "Norway" --territories "Norway,Germany,France"
  asc subscriptions pricing monthly-commitment disable --subscription-id "SUB_ID" --territories "Norway"
  asc subscriptions pricing monthly-commitment list --subscription-id "SUB_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsPricingMonthlyCommitmentEnableCommand(),
			SubscriptionsPricingMonthlyCommitmentDisableCommand(),
			SubscriptionsPricingMonthlyCommitmentListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsPricingMonthlyCommitmentEnableCommand validates enabling monthly commitment billing.
func SubscriptionsPricingMonthlyCommitmentEnableCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing monthly-commitment enable", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	price := fs.String("price", "", "Monthly customer price; total commitment is price x 12")
	priceTerritory := fs.String("price-territory", "", "Territory used to compare the upfront annual price")
	territories := fs.String("territories", "", "Territories to enable, comma-separated; USA and Singapore are excluded")
	availableInNew := fs.Bool("available-in-new-territories", false, "Include new eligible territories automatically when Apple exposes a public API")

	return &ffcli.Command{
		Name:       "enable",
		ShortUsage: "asc subscriptions pricing monthly-commitment enable [flags]",
		ShortHelp:  "[experimental] Validate monthly-commitment enable inputs.",
		LongHelp: `[experimental] Validate Monthly with 12-Month Commitment enable inputs.

The subscription must use subscriptionPeriod ONE_YEAR. USA and Singapore are
removed from --territories because Apple excludes those storefronts. The CLI
also checks that the 12-payment total is at least the upfront annual price and
no more than 1.5x the upfront annual price when the current upfront price can be
read from App Store Connect.

Examples:
  asc subscriptions pricing monthly-commitment enable --subscription-id "SUB_ID" --price "9.99" --price-territory "Norway" --territories "Norway,Germany,France"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := shared.RecoverBoolFlagTailArgs(fs, args, availableInNew); err != nil {
				return err
			}
			if len(args) > 0 {
				return shared.UsageError("subscriptions pricing monthly-commitment enable does not accept positional arguments")
			}

			id := strings.TrimSpace(*subscriptionID)
			if id == "" {
				return shared.UsageError("--subscription-id is required")
			}
			monthlyPrice := strings.TrimSpace(*price)
			if monthlyPrice == "" {
				return shared.UsageError("--price is required")
			}
			if err := shared.ValidateFinitePriceFlag("--price", monthlyPrice); err != nil {
				return shared.UsageError(err.Error())
			}
			priceTerritoryID, err := normalizeMonthlyCommitmentTerritory(*priceTerritory, "--price-territory")
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if _, excluded := monthlyCommitmentExcludedTerritories[priceTerritoryID]; excluded {
				return shared.UsageError("--price-territory cannot be USA or Singapore for monthly-commitment billing")
			}

			territoryIDs, err := shared.NormalizeASCTerritoryCSV(*territories)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if len(territoryIDs) == 0 {
				return shared.UsageError("--territories is required")
			}
			territoryIDs, excluded := filterMonthlyCommitmentTerritories(territoryIDs)
			printMonthlyCommitmentTerritoryWarning(excluded)
			if len(territoryIDs) == 0 {
				return shared.UsageError("no eligible monthly-commitment territories remain after excluding USA and Singapore")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions pricing monthly-commitment enable: %w", err)
			}

			id, err = resolveSubscriptionLookupIDWithTimeout(ctx, client, *appID, id)
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			subResp, err := client.GetSubscription(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions pricing monthly-commitment enable: failed to fetch subscription: %w", err)
			}
			if subResp.Data.Attributes.SubscriptionPeriod != string(asc.SubscriptionPeriodOneYear) {
				return shared.UsageError("--subscription-id must refer to a ONE_YEAR subscription for monthly-commitment billing")
			}

			summary, err := resolveSubscriptionPriceSummary(requestCtx, client, subWithGroup{Sub: subResp.Data}, priceTerritoryID)
			if err != nil {
				return fmt.Errorf("subscriptions pricing monthly-commitment enable: failed to fetch upfront price: %w", err)
			}
			if summary.CurrentPrice == nil || strings.TrimSpace(summary.CurrentPrice.Amount) == "" {
				return fmt.Errorf("subscriptions pricing monthly-commitment enable: current upfront price is missing for %s", priceTerritoryID)
			}
			if err := validateMonthlyCommitmentPriceRange(monthlyPrice, summary.CurrentPrice.Amount); err != nil {
				return fmt.Errorf("subscriptions pricing monthly-commitment enable: %w", err)
			}

			return fmt.Errorf("subscriptions pricing monthly-commitment enable: %w", errMonthlyCommitmentPublicAPINotAvailable)
		},
	}
}

// SubscriptionsPricingMonthlyCommitmentDisableCommand validates disabling monthly commitment billing.
func SubscriptionsPricingMonthlyCommitmentDisableCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing monthly-commitment disable", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")
	territories := fs.String("territories", "", "Territories to disable, comma-separated; USA and Singapore are excluded")

	return &ffcli.Command{
		Name:       "disable",
		ShortUsage: "asc subscriptions pricing monthly-commitment disable [flags]",
		ShortHelp:  "[experimental] Validate monthly-commitment disable inputs.",
		LongHelp: `[experimental] Validate Monthly with 12-Month Commitment disable inputs.

Examples:
  asc subscriptions pricing monthly-commitment disable --subscription-id "SUB_ID" --territories "Norway"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("subscriptions pricing monthly-commitment disable does not accept positional arguments")
			}
			if strings.TrimSpace(*subscriptionID) == "" {
				return shared.UsageError("--subscription-id is required")
			}
			territoryIDs, err := shared.NormalizeASCTerritoryCSV(*territories)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if len(territoryIDs) == 0 {
				return shared.UsageError("--territories is required")
			}
			territoryIDs, excluded := filterMonthlyCommitmentTerritories(territoryIDs)
			printMonthlyCommitmentTerritoryWarning(excluded)
			if len(territoryIDs) == 0 {
				return shared.UsageError("no eligible monthly-commitment territories remain after excluding USA and Singapore")
			}
			return fmt.Errorf("subscriptions pricing monthly-commitment disable: %w", errMonthlyCommitmentPublicAPINotAvailable)
		},
	}
}

// SubscriptionsPricingMonthlyCommitmentListCommand reports upstream support status.
func SubscriptionsPricingMonthlyCommitmentListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing monthly-commitment list", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc subscriptions pricing monthly-commitment list --subscription-id \"SUB_ID\"",
		ShortHelp:  "[experimental] List monthly-commitment billing configuration when Apple exposes a public API.",
		LongHelp: `[experimental] List Monthly with 12-Month Commitment billing configuration.

Examples:
  asc subscriptions pricing monthly-commitment list --subscription-id "SUB_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("subscriptions pricing monthly-commitment list does not accept positional arguments")
			}
			if strings.TrimSpace(*subscriptionID) == "" {
				return shared.UsageError("--subscription-id is required")
			}
			return fmt.Errorf("subscriptions pricing monthly-commitment list: %w", errMonthlyCommitmentPublicAPINotAvailable)
		},
	}
}

func normalizeSubscriptionBillingMode(value string) (subscriptionBillingMode, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "_", "-")
	if normalized == "" {
		return subscriptionBillingModeUpfront, nil
	}
	switch normalized {
	case "standard", string(subscriptionBillingModeUpfront):
		return subscriptionBillingModeUpfront, nil
	case "monthly-commitment", "monthly-with-12-month-commitment", "installment-billed-yearly":
		return subscriptionBillingModeMonthlyCommitment, nil
	default:
		return "", fmt.Errorf("--billing-mode must be one of: upfront, monthly-commitment")
	}
}

func normalizeMonthlyCommitmentTerritory(value string, flagName string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("%s is required", flagName)
	}
	territory, err := ascterritory.Normalize(trimmed)
	if err != nil {
		return "", err
	}
	return territory, nil
}

func filterMonthlyCommitmentTerritories(territories []string) ([]string, []string) {
	eligible := make([]string, 0, len(territories))
	excluded := make([]string, 0, 2)
	seenExcluded := make(map[string]struct{}, 2)
	for _, territory := range territories {
		id := strings.ToUpper(strings.TrimSpace(territory))
		if id == "" {
			continue
		}
		if _, ok := monthlyCommitmentExcludedTerritories[id]; ok {
			if _, seen := seenExcluded[id]; !seen {
				excluded = append(excluded, id)
				seenExcluded[id] = struct{}{}
			}
			continue
		}
		eligible = append(eligible, id)
	}
	return eligible, excluded
}

func printMonthlyCommitmentTerritoryWarning(excluded []string) {
	if len(excluded) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "Warning: monthly-commitment billing is unavailable in %s; removed from --territories.\n", strings.Join(excluded, ","))
}

func validateMonthlyCommitmentPriceRange(monthlyPrice string, upfrontPrice string) error {
	monthly, err := parsePositiveMoneyRat(monthlyPrice, "--price")
	if err != nil {
		return err
	}
	upfront, err := parsePositiveMoneyRat(upfrontPrice, "upfront price")
	if err != nil {
		return err
	}

	total := new(big.Rat).Mul(monthly, big.NewRat(12, 1))
	max := new(big.Rat).Mul(upfront, big.NewRat(3, 2))
	if total.Cmp(upfront) < 0 || total.Cmp(max) > 0 {
		return fmt.Errorf(
			"monthly commitment total %s is outside the allowed range [%s, %s] based on upfront price %s",
			formatMoneyRat(total),
			formatMoneyRat(upfront),
			formatMoneyRat(max),
			formatMoneyRat(upfront),
		)
	}
	return nil
}

func parsePositiveMoneyRat(value string, label string) (*big.Rat, error) {
	trimmed := strings.TrimSpace(value)
	rat, ok := new(big.Rat).SetString(trimmed)
	if !ok || rat.Sign() <= 0 {
		return nil, fmt.Errorf("%s must be a positive decimal price", label)
	}
	return rat, nil
}

func formatMoneyRat(value *big.Rat) string {
	if value == nil {
		return ""
	}
	return value.FloatString(2)
}
