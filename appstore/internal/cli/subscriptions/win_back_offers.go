package subscriptions

import (
	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/winbackoffers"
)

// newSubscriptionsWinBackOffersCommand returns the canonical nested win-back offers tree.
func newSubscriptionsWinBackOffersCommand(d *shared.Deps) *ffcli.Command {
	return shared.RewriteCommandTreePath(
		winbackoffers.NewWinBackOffersCommand(d),
		"asc win-back-offers",
		"asc subscriptions win-back-offers",
	)
}
