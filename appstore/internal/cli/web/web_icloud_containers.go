package web

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	webcore "github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/web"
)

var listDeveloperICloudContainersFn = func(ctx context.Context, client *webcore.Client, hidden bool) (*webcore.DeveloperICloudContainersListResult, error) {
	return client.ListDeveloperICloudContainers(ctx, hidden)
}

// WebICloudContainersCommand returns the read-only Developer Portal iCloud
// container command group.
func WebICloudContainersCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web icloud-containers", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "icloud-containers",
		ShortUsage: "asc web icloud-containers <subcommand> [flags]",
		ShortHelp:  "Read iCloud containers via a Developer Portal web session.",
		LongHelp: `Read iCloud containers through the selected Apple Developer team.

This command is read-only. Apple currently accepts a bounded 1000-resource
collection request for this web-session endpoint; the command does not expose a
--paginate flag. Use --output json when the complete Apple response envelope,
including links and metadata, is needed.
`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebICloudContainersListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebICloudContainersListCommand lists iCloud containers for the selected
// Developer Portal team.
func WebICloudContainersListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web icloud-containers list", flag.ExitOnError)
	hidden := fs.Bool("hidden", false, "List hidden iCloud containers instead of visible containers")
	authFlags := bindWebSessionFlags(fs)
	portalFlags := bindDeveloperPortalFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc web icloud-containers list [--hidden] [flags]",
		ShortHelp:  "List iCloud containers via a Developer Portal web session.",
		LongHelp: `List iCloud containers visible to the selected Apple Developer team.

Visible containers are returned by default. Pass --hidden to request the hidden
collection. The request asks Apple for up to 1000 resources and does not expose
--paginate; any links or paging metadata Apple returns remain available in JSON.
This command does not create, rename, delete, or inspect individual containers.

Examples:
  asc web icloud-containers list --output table
  asc web icloud-containers list --hidden --output json
`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web icloud-containers list does not accept positional arguments")
			}
			if err := validateDeveloperPortalFlags(portalFlags); err != nil {
				return err
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			session, requestCtx, cancel, err := resolveWebSessionForCommand(ctx, authFlags)
			defer cancel()
			if err != nil {
				return withWebAuthHint(err, "web icloud-containers list")
			}

			var result *webcore.DeveloperICloudContainersListResult
			err = withWebSpinner("Loading Developer Portal iCloud containers", func() error {
				var listErr error
				result, listErr = listDeveloperICloudContainersFn(requestCtx, newDeveloperPortalClient(session, portalFlags), *hidden)
				return listErr
			})
			if err != nil {
				return withWebAuthHint(err, "web icloud-containers list")
			}
			if result == nil {
				return fmt.Errorf("web icloud-containers list failed: missing list result")
			}
			persistDeveloperPortalSession(session)

			warnICloudContainerPagingTotal(result)
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderDeveloperICloudContainersTable(result) },
				func() error { return renderDeveloperICloudContainersMarkdown(result) },
			)
		},
	}
}

func developerICloudContainersHeaders() []string {
	return []string{"ID", "Name", "Identifier", "Prefix", "Hidden", "Can Edit", "Can Delete", "Response ID"}
}

func developerICloudContainersRows(containers []webcore.DeveloperICloudContainer) [][]string {
	rows := make([][]string, 0, len(containers))
	for _, container := range containers {
		attributes := container.Attributes
		rows = append(rows, []string{
			shared.OrNA(container.ID),
			shared.OrNA(attributes.Name),
			shared.OrNA(attributes.Identifier),
			shared.OrNA(attributes.Prefix),
			strconv.FormatBool(attributes.Hidden),
			strconv.FormatBool(attributes.CanEdit),
			strconv.FormatBool(attributes.CanDelete),
			shared.OrNA(attributes.ResponseID),
		})
	}
	return rows
}

func renderDeveloperICloudContainersTable(result *webcore.DeveloperICloudContainersListResult) error {
	if result == nil {
		asc.RenderTable(developerICloudContainersHeaders(), nil)
		return nil
	}
	asc.RenderTable(developerICloudContainersHeaders(), developerICloudContainersRows(result.Data))
	return nil
}

func renderDeveloperICloudContainersMarkdown(result *webcore.DeveloperICloudContainersListResult) error {
	if result == nil {
		asc.RenderMarkdown(developerICloudContainersHeaders(), nil)
		return nil
	}
	asc.RenderMarkdown(developerICloudContainersHeaders(), developerICloudContainersRows(result.Data))
	return nil
}

// Shared output warns for links.next; this covers totals without a next link.
func warnICloudContainerPagingTotal(result *webcore.DeveloperICloudContainersListResult) {
	if result.GetLinks().Next != "" {
		return
	}
	if total, ok := asc.ParsePagingTotalOK(result.GetMeta()); ok && total > len(result.Data) {
		fmt.Fprintf(os.Stderr, "Warning: showing %d of %d results; this command reads only the first page\n", len(result.Data), total)
	}
}
