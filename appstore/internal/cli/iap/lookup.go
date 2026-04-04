package iap

import (
	"context"
	"flag"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

const iapLookupAppUsage = "App Store Connect app ID (or ASC_APP_ID env; required when --iap-id uses a product ID or name)"

func addIAPLookupAppFlag(fs *flag.FlagSet) *string {
	return fs.String("app", "", iapLookupAppUsage)
}

func resolveIAPLookupID(ctx context.Context, client *asc.Client, appValue, selector string) (string, error) {
	resolvedAppID := shared.ResolveAppID(appValue)
	if err := shared.RequireAppForStableSelector(resolvedAppID, selector, "--iap-id"); err != nil {
		return "", err
	}
	return shared.ResolveIAPID(ctx, client, resolvedAppID, selector)
}

func resolveIAPLookupIDWithTimeout(ctx context.Context, client *asc.Client, appValue, selector string) (string, error) {
	lookupCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	return resolveIAPLookupID(lookupCtx, client, appValue, selector)
}
