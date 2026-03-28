package betabuildlocalizations

import "github.com/peterbourgon/ff/v3/ffcli"

// BetaBuildLocalizationsCommand returns the legacy deprecated compatibility
// tree. The behavior stays available during the transition instead of failing
// hard so existing scripts can migrate with warnings first.
func BetaBuildLocalizationsCommand() *ffcli.Command {
	return legacyBetaBuildLocalizationsCommand()
}
