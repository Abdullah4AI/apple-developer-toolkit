package builds

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

const (
	deprecatedBuildsLatestFetchWarning = "Warning: `asc builds latest` is deprecated. Use `asc builds info --latest`."
	deprecatedBuildsLatestNextWarning  = "Warning: `asc builds latest --next` is deprecated. Use `asc builds next-build-number`."
)

type latestBuildSelectionOptions = shared.LatestBuildSelectionOptions

type nextBuildNumberOptions = shared.NextBuildNumberOptions

// BuildsLatestCommand returns a deprecated compatibility wrapper for the old
// latest-build command surface.
func BuildsLatestCommand() *ffcli.Command {
	fs := flag.NewFlagSet("latest", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID, bundle ID, or exact app name (required, or ASC_APP_ID env)")
	version := fs.String("version", "", "Filter by version string (e.g., 1.2.3); requires --platform for deterministic results")
	platform := fs.String("platform", "", "Filter by platform: IOS, MAC_OS, TV_OS, VISION_OS")
	processingState := fs.String("processing-state", "", "Filter by processing state: VALID, PROCESSING, FAILED, INVALID, or all")
	output := shared.BindOutputFlags(fs)
	next := fs.Bool("next", false, "Return next build number using processed builds and in-flight uploads")
	initialBuildNumber := fs.Int("initial-build-number", 1, "Initial build number when none exist (used with --next)")
	excludeExpired := fs.Bool("exclude-expired", false, "Exclude expired builds when selecting latest build")
	notExpired := fs.Bool("not-expired", false, "Alias for --exclude-expired")

	return &ffcli.Command{
		Name:       "latest",
		ShortUsage: "asc builds latest [flags]",
		ShortHelp:  "DEPRECATED: use `asc builds info --latest` or `asc builds next-build-number`.",
		LongHelp:   "Deprecated compatibility command for `asc builds info --latest` and `asc builds next-build-number`.",
		FlagSet:    fs,
		UsageFunc:  shared.DeprecatedUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *next {
				fmt.Fprintln(os.Stderr, deprecatedBuildsLatestNextWarning)
			} else {
				fmt.Fprintln(os.Stderr, deprecatedBuildsLatestFetchWarning)
			}

			excludeExpiredValue := *excludeExpired || *notExpired
			selectionOpts, err := normalizeLatestBuildSelectionOptions(*appID, *version, *platform, *processingState, excludeExpiredValue)
			if err != nil {
				return err
			}
			if *next && *initialBuildNumber < 1 {
				return shared.UsageError("--initial-build-number must be >= 1")
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds latest: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if *next {
				result, err := resolveNextBuildNumber(requestCtx, client, nextBuildNumberOptions{
					LatestBuildSelectionOptions: selectionOpts,
					InitialBuildNumber:          *initialBuildNumber,
				})
				if err != nil {
					return fmt.Errorf("builds latest: %w", err)
				}
				return shared.PrintOutput(result, *output.Output, *output.Pretty)
			}

			build, err := resolveLatestBuild(requestCtx, client, selectionOpts)
			if err != nil {
				return fmt.Errorf("builds latest: %w", err)
			}
			return shared.PrintOutput(build, *output.Output, *output.Pretty)
		},
	}
}

// BuildsNextBuildNumberCommand returns the canonical next build number subcommand.
func BuildsNextBuildNumberCommand() *ffcli.Command {
	fs := flag.NewFlagSet("next-build-number", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID, bundle ID, or exact app name (required, or ASC_APP_ID env)")
	version := fs.String("version", "", "Optional version filter for latest processed/uploaded build selection")
	platform := fs.String("platform", "", "Optional platform filter: IOS, MAC_OS, TV_OS, VISION_OS")
	processingState := fs.String("processing-state", "", "Optional processing state filter: VALID, PROCESSING, FAILED, INVALID, or all")
	initialBuildNumber := fs.Int("initial-build-number", 1, "Initial build number when none exist")
	excludeExpired := fs.Bool("exclude-expired", false, "Exclude expired builds when selecting the latest processed build")
	notExpired := fs.Bool("not-expired", false, "Alias for --exclude-expired")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "next-build-number",
		ShortUsage: "asc builds next-build-number --app APP [flags]",
		ShortHelp:  "Calculate the next build number for an app.",
		LongHelp: `Calculate the next build number for an app.

This command compares the latest processed build and in-flight build uploads,
then returns the next build number that should be safe to use.

Examples:
  asc builds next-build-number --app "123456789"
  asc builds next-build-number --app "123456789" --version "1.2.3" --platform IOS
  asc builds next-build-number --app "123456789" --version "1.2.3" --platform IOS --exclude-expired
  asc builds next-build-number --app "123456789" --initial-build-number 7`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			excludeExpiredValue := *excludeExpired || *notExpired
			selectionOpts, err := normalizeLatestBuildSelectionOptions(*appID, *version, *platform, *processingState, excludeExpiredValue)
			if err != nil {
				return err
			}
			if *initialBuildNumber < 1 {
				return shared.UsageError("--initial-build-number must be >= 1")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds next-build-number: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			result, err := resolveNextBuildNumber(requestCtx, client, nextBuildNumberOptions{
				LatestBuildSelectionOptions: selectionOpts,
				InitialBuildNumber:          *initialBuildNumber,
			})
			if err != nil {
				return fmt.Errorf("builds next-build-number: %w", err)
			}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func normalizeLatestBuildSelectionOptions(appID, version, platform, processingState string, excludeExpired bool) (latestBuildSelectionOptions, error) {
	opts, err := shared.NormalizeLatestBuildSelectionOptions(appID, version, platform, processingState, excludeExpired)
	if err != nil {
		return latestBuildSelectionOptions{}, err
	}
	return opts, nil
}

func resolveLatestBuild(ctx context.Context, client *asc.Client, opts latestBuildSelectionOptions) (*asc.BuildResponse, error) {
	return shared.ResolveLatestBuild(ctx, client, opts, false)
}

func resolveNextBuildNumber(ctx context.Context, client *asc.Client, opts nextBuildNumberOptions) (*asc.BuildsNextBuildNumberResult, error) {
	return shared.ResolveNextBuildNumber(ctx, client, opts)
}
