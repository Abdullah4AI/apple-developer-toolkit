package builds

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

// ResolveBuildOptions configures how a build is resolved.
type ResolveBuildOptions struct {
	BuildID               string
	AppID                 string
	Version               string
	BuildNumber           string
	Platform              string
	Latest                bool
	ProcessingStateValues []string
	ExcludeExpired        bool
}

type buildNumberSelectionOptions struct {
	AppID                 string
	Version               string
	BuildNumber           string
	Platform              string
	Since                 *time.Time
	ProcessingStateValues []string
}

// ResolveBuild finds a build by ID, by app+build-number+platform, or by latest.
// Returns the build response or an error. Callers use this to avoid duplicating
// build lookup logic across commands (dsyms, wait, find, etc.).
func ResolveBuild(ctx context.Context, client *asc.Client, opts ResolveBuildOptions) (*asc.BuildResponse, error) {
	if err := validateResolveBuildOptions(opts); err != nil {
		return nil, err
	}
	if client == nil {
		return nil, fmt.Errorf("build client is required")
	}

	buildNumber := strings.TrimSpace(opts.BuildNumber)

	// Direct build ID.
	if opts.BuildID != "" {
		resp, err := client.GetBuild(ctx, opts.BuildID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch build %s: %w", opts.BuildID, err)
		}
		return resp, nil
	}

	// Latest mode: find the most recently uploaded build.
	if opts.Latest {
		platform := strings.TrimSpace(opts.Platform)
		if platform != "" {
			normalized, err := shared.NormalizeAppStoreVersionPlatform(platform)
			if err != nil {
				return nil, shared.UsageError(err.Error())
			}
			platform = normalized
		}
		selection, err := resolveLatestBuildSelection(ctx, client, latestBuildSelectionOptions{
			AppID:                 strings.TrimSpace(opts.AppID),
			Version:               strings.TrimSpace(opts.Version),
			Platform:              platform,
			ProcessingStateValues: opts.ProcessingStateValues,
			ExcludeExpired:        opts.ExcludeExpired,
		}, false)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch latest build: %w", err)
		}
		return selection.LatestBuild, nil
	}

	return resolveBuildByNumberSelection(ctx, client, buildNumberSelectionOptions{
		AppID:                 strings.TrimSpace(opts.AppID),
		Version:               strings.TrimSpace(opts.Version),
		BuildNumber:           buildNumber,
		Platform:              strings.TrimSpace(opts.Platform),
		ProcessingStateValues: opts.ProcessingStateValues,
	}, false)
}

func validateResolveBuildOptions(opts ResolveBuildOptions) error {
	buildID := strings.TrimSpace(opts.BuildID)
	buildNumber := strings.TrimSpace(opts.BuildNumber)
	version := strings.TrimSpace(opts.Version)
	platform := strings.TrimSpace(opts.Platform)
	appInput := strings.TrimSpace(opts.AppID)
	hasExplicitAppSelectors := appInput != "" || opts.Latest || buildNumber != "" || version != "" || platform != "" || len(opts.ProcessingStateValues) > 0 || opts.ExcludeExpired

	if buildID != "" && hasExplicitAppSelectors {
		return shared.UsageError("--build-id cannot be combined with --app, --latest, --build-number, --version, --platform, --processing-state, --exclude-expired, or --not-expired")
	}
	if opts.Latest && buildNumber != "" {
		return shared.UsageError("--latest and --build-number are mutually exclusive")
	}
	if buildID != "" {
		return nil
	}

	if shared.ResolveAppID(appInput) == "" {
		return shared.UsageError("--build-id or --app is required (or set ASC_APP_ID)")
	}
	if len(opts.ProcessingStateValues) > 0 && !opts.Latest {
		return shared.UsageError("--processing-state requires --latest")
	}
	if opts.ExcludeExpired && !opts.Latest {
		return shared.UsageError("--exclude-expired and --not-expired require --latest")
	}
	if !opts.Latest && buildNumber == "" {
		return shared.UsageError("--build-id, --latest, or --build-number is required")
	}
	if platform != "" {
		if _, err := shared.NormalizeAppStoreVersionPlatform(platform); err != nil {
			return shared.UsageError(err.Error())
		}
	}
	return nil
}

func resolveBuildByNumberSelection(
	ctx context.Context,
	client *asc.Client,
	opts buildNumberSelectionOptions,
	allowEmpty bool,
) (*asc.BuildResponse, error) {
	if client == nil {
		return nil, fmt.Errorf("build client is required")
	}

	appID := shared.ResolveAppID(strings.TrimSpace(opts.AppID))
	if appID == "" {
		return nil, shared.UsageError("--app is required (or set ASC_APP_ID)")
	}

	buildNumber := strings.TrimSpace(opts.BuildNumber)
	version := strings.TrimSpace(opts.Version)
	platform := applyLegacyImplicitBuildNumberPlatformDefault(buildNumber, strings.TrimSpace(opts.Platform))
	if platform != "" {
		normalized, err := shared.NormalizeAppStoreVersionPlatform(platform)
		if err != nil {
			return nil, shared.UsageError(err.Error())
		}
		platform = normalized
	}

	resolvedAppID, err := shared.ResolveAppIDWithLookup(ctx, client, appID)
	if err != nil {
		return nil, err
	}

	buildOpts := []asc.BuildsOption{
		asc.WithBuildsBuildNumber(buildNumber),
		asc.WithBuildsSort("-uploadedDate"),
		asc.WithBuildsLimit(200),
	}
	if len(opts.ProcessingStateValues) > 0 {
		buildOpts = append(buildOpts, asc.WithBuildsProcessingStates(opts.ProcessingStateValues))
	}
	if version != "" {
		preReleaseVersionIDs, err := findPreReleaseVersionIDs(ctx, client, resolvedAppID, version, platform)
		if err != nil {
			return nil, err
		}
		if len(preReleaseVersionIDs) == 0 {
			if allowEmpty {
				return nil, nil
			}
			return nil, noBuildFoundForBuildNumber(resolvedAppID, buildNumber, version, platform)
		}
		buildOpts = append(buildOpts, asc.WithBuildsPreReleaseVersions(preReleaseVersionIDs))
	} else if platform != "" {
		buildOpts = append(buildOpts, asc.WithBuildsPreReleaseVersionPlatforms([]string{platform}))
	}

	if opts.Since != nil {
		return resolveBuildByNumberSelectionSince(
			ctx,
			client,
			resolvedAppID,
			buildNumber,
			version,
			platform,
			buildOpts,
			opts.Since,
			allowEmpty,
		)
	}

	buildsResp, err := client.GetBuilds(ctx, resolvedAppID, buildOpts...)
	if err != nil {
		return nil, err
	}
	if len(buildsResp.Data) == 0 {
		if allowEmpty {
			return nil, nil
		}
		return nil, noBuildFoundForBuildNumber(resolvedAppID, buildNumber, version, platform)
	}

	if len(buildsResp.Data) > 1 || strings.TrimSpace(buildsResp.Links.Next) != "" {
		return nil, ambiguousBuildNumberSelection(resolvedAppID, buildNumber, version, platform)
	}

	return &asc.BuildResponse{Data: buildsResp.Data[0], Links: buildsResp.Links}, nil
}

func resolveBuildByNumberSelectionSince(
	ctx context.Context,
	client *asc.Client,
	appID, buildNumber, version, platform string,
	buildOpts []asc.BuildsOption,
	since *time.Time,
	allowEmpty bool,
) (*asc.BuildResponse, error) {
	threshold := since.UTC()
	var selected *asc.Resource[asc.BuildAttributes]
	pageOpts := append([]asc.BuildsOption{}, buildOpts...)

	for {
		buildsResp, err := client.GetBuilds(ctx, appID, pageOpts...)
		if err != nil {
			return nil, err
		}

		for _, build := range buildsResp.Data {
			uploadedAt, err := parseBuildUploadedTimestamp(build.Attributes.UploadedDate)
			if err != nil {
				return nil, fmt.Errorf("failed to parse uploadedDate for build %s: %w", build.ID, err)
			}
			if uploadedAt.Before(threshold) {
				if selected == nil {
					if allowEmpty {
						return nil, nil
					}
					return nil, noBuildFoundForBuildNumber(appID, buildNumber, version, platform)
				}
				return &asc.BuildResponse{Data: *selected}, nil
			}
			if selected != nil {
				return nil, ambiguousBuildNumberSelection(appID, buildNumber, version, platform)
			}

			selectedBuild := build
			selected = &selectedBuild
		}

		nextURL := strings.TrimSpace(buildsResp.Links.Next)
		if nextURL == "" {
			if selected == nil {
				if allowEmpty {
					return nil, nil
				}
				return nil, noBuildFoundForBuildNumber(appID, buildNumber, version, platform)
			}
			return &asc.BuildResponse{Data: *selected}, nil
		}

		pageOpts = []asc.BuildsOption{asc.WithBuildsNextURL(nextURL)}
	}
}

func noBuildFoundForBuildNumber(appID, buildNumber, version, platform string) error {
	return fmt.Errorf(
		"no build found for app %s with build number %q%s",
		appID,
		buildNumber,
		describeBuildNumberSelectionFilters(version, platform),
	)
}

func ambiguousBuildNumberSelection(appID, buildNumber, version, platform string) error {
	return fmt.Errorf(
		"multiple builds found for app %s with build number %q%s; %s",
		appID,
		buildNumber,
		describeBuildNumberSelectionFilters(version, platform),
		describeBuildNumberSelectionHint(version, platform),
	)
}

func describeBuildNumberSelectionFilters(version, platform string) string {
	var parts []string
	if strings.TrimSpace(version) != "" {
		parts = append(parts, fmt.Sprintf("version %q", strings.TrimSpace(version)))
	}
	if strings.TrimSpace(platform) != "" {
		parts = append(parts, fmt.Sprintf("platform %s", strings.TrimSpace(platform)))
	}
	if len(parts) == 0 {
		return ""
	}
	return " for " + strings.Join(parts, " and ")
}

func describeBuildNumberSelectionHint(version, platform string) string {
	switch {
	case strings.TrimSpace(version) == "" && strings.TrimSpace(platform) == "":
		return "add --version and/or --platform, or use --build-id"
	case strings.TrimSpace(version) == "":
		return "add --version, or use --build-id"
	case strings.TrimSpace(platform) == "":
		return "add --platform, or use --build-id"
	default:
		return "use --build-id"
	}
}
