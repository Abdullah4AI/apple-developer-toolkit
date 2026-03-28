package builds

import (
	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

// BuildsFindCommand returns a deprecated compatibility alias for builds info.
func BuildsFindCommand() *ffcli.Command {
	return shared.DeprecatedAliasLeafCommand(
		BuildsInfoCommand(),
		"find",
		"asc builds find --app APP --build-number BUILD_NUMBER [--version VERSION] [--platform PLATFORM] [flags]",
		"asc builds info",
		"Warning: `asc builds find` is deprecated. Use `asc builds info`.",
	)
}
