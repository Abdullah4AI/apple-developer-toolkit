package validate

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/validation"
)

type validateSubscriptionsOptions struct {
	AppID  string
	Strict bool
	Output string
	Pretty bool
}

// ValidateSubscriptionsCommand returns the asc validate subscriptions subcommand.
func ValidateSubscriptionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("subscriptions", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	strict := fs.Bool("strict", false, "Treat warnings as errors (exit non-zero)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "subscriptions",
		ShortUsage: "asc validate subscriptions --app \"APP_ID\" [flags]",
		ShortHelp:  "Validate subscription review readiness (warning-only by default).",
		LongHelp: `Validate review readiness for auto-renewable subscriptions.

This command is conservative: it emits warnings for subscriptions that look
unsubmitted or need action, but it does not block by default (use --strict for CI).

Examples:
  asc validate subscriptions --app "APP_ID"
  asc validate subscriptions --app "APP_ID" --output table
  asc validate subscriptions --app "APP_ID" --strict`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			return runValidateSubscriptions(ctx, validateSubscriptionsOptions{
				AppID:  resolvedAppID,
				Strict: *strict,
				Output: *output.Output,
				Pretty: *output.Pretty,
			})
		},
	}
}

func runValidateSubscriptions(ctx context.Context, opts validateSubscriptionsOptions) error {
	client, err := clientFactory()
	if err != nil {
		return fmt.Errorf("validate subscriptions: %w", err)
	}

	groupsCtx, groupsCancel := shared.ContextWithTimeout(ctx)
	groupsResp, err := client.GetSubscriptionGroups(groupsCtx, opts.AppID, asc.WithSubscriptionGroupsLimit(200))
	groupsCancel()
	if err != nil {
		return fmt.Errorf("validate subscriptions: failed to fetch subscription groups: %w", err)
	}

	paginatedGroups, err := asc.PaginateAll(ctx, groupsResp, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
		defer pageCancel()
		return client.GetSubscriptionGroups(pageCtx, opts.AppID, asc.WithSubscriptionGroupsNextURL(nextURL))
	})
	if err != nil {
		return fmt.Errorf("validate subscriptions: paginate subscription groups: %w", err)
	}

	groups, ok := paginatedGroups.(*asc.SubscriptionGroupsResponse)
	if !ok {
		return fmt.Errorf("validate subscriptions: unexpected subscription groups response type %T", paginatedGroups)
	}

	subs := make([]validation.Subscription, 0)
	for _, group := range groups.Data {
		groupID := strings.TrimSpace(group.ID)
		if groupID == "" {
			continue
		}

		subsCtx, subsCancel := shared.ContextWithTimeout(ctx)
		subsResp, err := client.GetSubscriptions(subsCtx, groupID, asc.WithSubscriptionsLimit(200))
		subsCancel()
		if err != nil {
			return fmt.Errorf("validate subscriptions: failed to fetch subscriptions for group %s: %w", groupID, err)
		}

		paginatedSubs, err := asc.PaginateAll(ctx, subsResp, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
			pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
			defer pageCancel()
			return client.GetSubscriptions(pageCtx, groupID, asc.WithSubscriptionsNextURL(nextURL))
		})
		if err != nil {
			return fmt.Errorf("validate subscriptions: paginate subscriptions: %w", err)
		}

		subsResult, ok := paginatedSubs.(*asc.SubscriptionsResponse)
		if !ok {
			return fmt.Errorf("validate subscriptions: unexpected subscriptions response type %T", paginatedSubs)
		}

		for _, sub := range subsResult.Data {
			attrs := sub.Attributes
			subs = append(subs, validation.Subscription{
				ID:        sub.ID,
				Name:      attrs.Name,
				ProductID: attrs.ProductID,
				State:     attrs.State,
				GroupID:   groupID,
			})
		}
	}

	report := validation.ValidateSubscriptions(validation.SubscriptionsInput{
		AppID:         opts.AppID,
		Subscriptions: subs,
	}, opts.Strict)

	if err := shared.PrintOutput(&report, opts.Output, opts.Pretty); err != nil {
		return err
	}

	if report.Summary.Blocking > 0 {
		return shared.NewReportedError(fmt.Errorf("validate subscriptions: found %d blocking issue(s)", report.Summary.Blocking))
	}

	return nil
}
