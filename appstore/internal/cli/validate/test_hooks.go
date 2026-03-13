package validate

import (
	"context"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/validation"
)

// SetClientFactory replaces the ASC client factory for tests.
// It returns a restore function to reset the previous handler.
func SetClientFactory(fn func() (*asc.Client, error)) func() {
	previous := clientFactory
	if fn == nil {
		clientFactory = shared.GetASCClient
	} else {
		clientFactory = fn
	}
	return func() {
		clientFactory = previous
	}
}

// SetFetchSubscriptionsFunc replaces the subscription fetcher for tests.
// It returns a restore function to reset the previous handler.
func SetFetchSubscriptionsFunc(fn func(context.Context, *asc.Client, string) ([]validation.Subscription, error)) func() {
	previous := fetchSubscriptionsFn
	if fn == nil {
		fetchSubscriptionsFn = fetchSubscriptions
	} else {
		fetchSubscriptionsFn = fn
	}
	return func() {
		fetchSubscriptionsFn = previous
	}
}

// SetFetchIAPsFunc replaces the IAP fetcher for tests.
// It returns a restore function to reset the previous handler.
func SetFetchIAPsFunc(fn func(context.Context, *asc.Client, string) ([]validation.IAP, error)) func() {
	previous := fetchIAPsFn
	if fn == nil {
		fetchIAPsFn = fetchIAPs
	} else {
		fetchIAPsFn = fn
	}
	return func() {
		fetchIAPsFn = previous
	}
}
