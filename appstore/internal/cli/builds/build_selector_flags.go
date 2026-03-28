package builds

import (
	"context"
	"flag"
	"strings"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

type buildSelectorFlags struct {
	buildID       *string
	legacyBuildID *trackedStringFlag
	legacyID      *trackedStringFlag
	appID         *string
	latest        *bool
	version       *string
	buildNumber   *string
	platform      *string
}

type buildSelectorFlagOptions struct {
	includeLegacyID  bool
	buildIDUsage     string
	appUsage         string
	latestUsage      string
	versionUsage     string
	buildNumberUsage string
	platformUsage    string
}

func bindBuildSelectorFlags(fs *flag.FlagSet, opts buildSelectorFlagOptions) buildSelectorFlags {
	buildIDUsage := strings.TrimSpace(opts.buildIDUsage)
	if buildIDUsage == "" {
		buildIDUsage = "Build ID"
	}
	appUsage := strings.TrimSpace(opts.appUsage)
	if appUsage == "" {
		appUsage = "App Store Connect app ID, bundle ID, or exact app name (required when --build-id is not provided)"
	}
	latestUsage := strings.TrimSpace(opts.latestUsage)
	if latestUsage == "" {
		latestUsage = "Select the latest matching build for --app context"
	}
	versionUsage := strings.TrimSpace(opts.versionUsage)
	if versionUsage == "" {
		versionUsage = "Optional marketing version filter (CFBundleShortVersionString) for --app selectors"
	}
	buildNumberUsage := strings.TrimSpace(opts.buildNumberUsage)
	if buildNumberUsage == "" {
		buildNumberUsage = "Select a unique build by build number (CFBundleVersion) for --app context"
	}
	platformUsage := strings.TrimSpace(opts.platformUsage)
	if platformUsage == "" {
		platformUsage = "Optional platform filter for --app selectors: IOS, MAC_OS, TV_OS, VISION_OS"
	}

	selectors := buildSelectorFlags{
		buildID:       fs.String("build-id", "", buildIDUsage),
		legacyBuildID: bindHiddenStringFlag(fs, "build"),
		appID:         fs.String("app", "", appUsage),
		latest:        fs.Bool("latest", false, latestUsage),
		version:       fs.String("version", "", versionUsage),
		buildNumber:   fs.String("build-number", "", buildNumberUsage),
		platform:      fs.String("platform", "", platformUsage),
	}
	if opts.includeLegacyID {
		selectors.legacyID = bindHiddenStringFlag(fs, "id")
	}

	return selectors
}

func (s buildSelectorFlags) applyLegacyAliases() error {
	if err := applyLegacyBuildIDAlias(s.buildID, s.legacyBuildID); err != nil {
		return err
	}
	if s.legacyID != nil {
		if err := applyLegacyIDAlias(s.buildID, s.legacyID); err != nil {
			return err
		}
	}
	return nil
}

func (s buildSelectorFlags) resolveOptions() ResolveBuildOptions {
	return ResolveBuildOptions{
		BuildID:     strings.TrimSpace(s.value(s.buildID)),
		AppID:       strings.TrimSpace(s.value(s.appID)),
		Version:     strings.TrimSpace(s.value(s.version)),
		BuildNumber: strings.TrimSpace(s.value(s.buildNumber)),
		Platform:    strings.TrimSpace(s.value(s.platform)),
		Latest:      s.latest != nil && *s.latest,
	}
}

func (s buildSelectorFlags) validate() error {
	return validateResolveBuildOptions(s.resolveOptions())
}

func (s buildSelectorFlags) resolveBuild(ctx context.Context, client *asc.Client) (*asc.BuildResponse, error) {
	return ResolveBuild(ctx, client, s.resolveOptions())
}

func (s buildSelectorFlags) resolveBuildID(ctx context.Context, client *asc.Client) (string, error) {
	buildID := strings.TrimSpace(s.value(s.buildID))
	if buildID != "" {
		return buildID, nil
	}

	buildResp, err := s.resolveBuild(ctx, client)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(buildResp.Data.ID), nil
}

func (s buildSelectorFlags) value(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}
