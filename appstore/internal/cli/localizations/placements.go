package localizations

import (
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

// LocalizationsPlacementsCommand exposes localized header and search placements.
func newLocalizationsPlacementsCommand(d *shared.Deps) *ffcli.Command {
	return d.CreativePlacementsCommand("appStoreVersionLocalizations", "localizations", "an App Store version localization")
}
