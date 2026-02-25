package initcmd

import (
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/docs"
)

// InitCommand returns the root init command.
func InitCommand() *ffcli.Command {
	return docs.NewInitReferenceCommand(
		"init",
		"init",
		"asc init [flags]",
		"Initialize asc helper docs in the current repo.",
		`Initialize asc helper docs in the current repo.

Examples:
  asc init
  asc init --path ./ASC.md
  asc init --force --link=false`,
		"init",
	)
}
