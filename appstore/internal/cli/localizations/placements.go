package localizations

import (
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

// LocalizationsPlacementsCommand exposes localized header and search placements.
func LocalizationsPlacementsCommand() *ffcli.Command {
	return shared.CreativePlacementsCommand("appStoreVersionLocalizations", "localizations", "an App Store version localization")
}
