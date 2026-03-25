package cmdtest

import (
	"github.com/peterbourgon/ff/v3/ffcli"

	cmd "github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/auth"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

func resetCmdtestState() {
	asc.ResetConfigCacheForTest()
	auth.ResetInvalidBypassKeychainWarningsForTest()
	shared.ResetDefaultOutputFormat()
	shared.ResetTierCacheForTest()
}

func RootCommand(version string) *ffcli.Command {
	resetCmdtestState()
	return cmd.RootCommand(version)
}

type ReportedError = shared.ReportedError
