package web

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	webcore "github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/web"
)

var setWebMedicalDeviceDeclarationFn = func(ctx context.Context, client *webcore.Client, accountID, appID string, declared bool) (*webcore.MedicalDeviceDeclarationResult, error) {
	return client.SetMedicalDeviceDeclaration(ctx, accountID, appID, declared)
}

// WebAppsMedicalDeviceCommand returns the regulated medical device command group.
func WebAppsMedicalDeviceCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web apps medical-device", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "medical-device",
		ShortUsage: "asc web apps medical-device <subcommand> [flags]",
		ShortHelp:  "[experimental] Manage the regulated medical device declaration via web sessions.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Manage the regulated medical device declaration exposed in App Store Connect
under App Information -> App Store Regulations & Permits.

This command currently automates only the common "No" path. If your app is
actually a regulated medical device, continue using the App Store Connect web UI
until the full undocumented "Yes" write contract is captured safely.

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebAppsMedicalDeviceSetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebAppsMedicalDeviceSetCommand sets the regulated medical device declaration.
func WebAppsMedicalDeviceSetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web apps medical-device set", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	var declared shared.OptionalBool
	fs.Var(&declared, "declared", "Set regulated medical device declaration: false")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc web apps medical-device set --app APP_ID --declared false [flags]",
		ShortHelp:  "[experimental] Set regulated medical device declaration via web API.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Set the regulated medical device declaration through Apple's private
compliance-form web endpoint used by App Store Connect.

Only the "No" path is currently supported, which covers the common case for
apps that are not regulated medical devices.

Examples:
  asc web apps medical-device set --app "6748252780" --declared false

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("web apps medical-device set does not accept positional arguments")
			}

			resolvedAppID := strings.TrimSpace(shared.ResolveAppID(*appID))
			if resolvedAppID == "" {
				return shared.UsageError("--app is required (or set ASC_APP_ID)")
			}
			if !declared.IsSet() {
				return shared.UsageError("--declared is required (supported value: false)")
			}
			if declared.Value() {
				return shared.UsageError("--declared true is not yet supported; only false is currently supported")
			}

			session, err := resolveWebSessionForCommand(ctx, authFlags)
			if err != nil {
				return err
			}

			accountID := strings.TrimSpace(session.PublicProviderID)
			if accountID == "" {
				return fmt.Errorf("web apps medical-device set failed: web session is missing public provider/account id (run 'asc web auth login')")
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			client := newWebClientFn(session)
			var result *webcore.MedicalDeviceDeclarationResult
			err = withWebSpinner("Saving regulated medical device declaration", func() error {
				var err error
				result, err = setWebMedicalDeviceDeclarationFn(requestCtx, client, accountID, resolvedAppID, false)
				return err
			})
			if err != nil {
				return withWebAuthHint(err, "web apps medical-device set")
			}
			if result == nil {
				return fmt.Errorf("web apps medical-device set failed: missing declaration result")
			}

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return renderWebMedicalDeviceTable(result) },
				func() error { return renderWebMedicalDeviceMarkdown(result) },
			)
		},
	}
}

func renderWebMedicalDeviceTable(result *webcore.MedicalDeviceDeclarationResult) error {
	asc.RenderTable(
		[]string{"App ID", "Requirement", "Declared", "Status", "Countries/Regions"},
		[][]string{{
			result.AppID,
			result.RequirementName,
			fmt.Sprintf("%t", result.Declared),
			valueOrNA(result.Status),
			strings.Join(result.CountriesOrRegions, ","),
		}},
	)
	return nil
}

func renderWebMedicalDeviceMarkdown(result *webcore.MedicalDeviceDeclarationResult) error {
	asc.RenderMarkdown(
		[]string{"App ID", "Requirement", "Declared", "Status", "Countries/Regions"},
		[][]string{{
			result.AppID,
			result.RequirementName,
			fmt.Sprintf("%t", result.Declared),
			valueOrNA(result.Status),
			strings.Join(result.CountriesOrRegions, ","),
		}},
	)
	return nil
}
