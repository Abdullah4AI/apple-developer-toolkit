package xcodecloud

import (
	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

// newXcodeCloudIssuesCommand returns the xcode-cloud issues command with subcommands.
func newXcodeCloudIssuesCommand(d *shared.Deps) *ffcli.Command {
	return newXcodeCloudActionResourceCommand(d, xcodeCloudIssuesCommandConfig)
}
