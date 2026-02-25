package docs

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

// DocsListCommand returns the docs list subcommand.
func DocsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("docs list", flag.ExitOnError)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc docs list [flags]",
		ShortHelp:  "List available embedded documentation guides.",
		LongHelp: `List available embedded documentation guides.

Examples:
  asc docs list
  asc docs list --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			_ = ctx
			if len(args) > 0 {
				fmt.Fprintln(os.Stderr, "Error: docs list does not accept positional arguments")
				return flag.ErrHelp
			}
			if err := shared.PrintOutputWithRenderers(
				listGuideSummaries(),
				*output.Output,
				*output.Pretty,
				func() error {
					asc.RenderTable([]string{"slug", "description"}, guideRows())
					return nil
				},
				func() error {
					asc.RenderMarkdown([]string{"slug", "description"}, guideRows())
					return nil
				},
			); err != nil {
				return fmt.Errorf("docs list: %w", err)
			}
			return nil
		},
	}
}
