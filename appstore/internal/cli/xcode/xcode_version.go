package xcode

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	localxcode "github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/xcode"
)

var (
	runGetVersion  = localxcode.GetVersion
	runSetVersion  = localxcode.SetVersion
	runBumpVersion = localxcode.BumpVersion
)

// XcodeVersionCommand returns the xcode version command group.
func XcodeVersionCommand() *ffcli.Command {
	fs := flag.NewFlagSet("version", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "version",
		ShortUsage: "asc xcode version <subcommand> [flags]",
		ShortHelp:  "Read and modify Xcode project version numbers.",
		LongHelp: `Read and modify Xcode project version and build numbers using agvtool.

Requires Apple Generic Versioning to be enabled in the Xcode project.
macOS only.

Examples:
  asc xcode version view
  asc xcode version view --project-dir ./MyApp
  asc xcode version view --project ./MyApp/App.xcodeproj
  asc xcode version edit --version "1.3.0"
  asc xcode version edit --build-number "42"
  asc xcode version edit --version "1.3.0" --build-number "42"
  asc xcode version bump --type patch
  asc xcode version bump --type minor
  asc xcode version bump --type major
  asc xcode version bump --type build`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			xcodeVersionViewCommand(),
			xcodeVersionEditCommand(),
			xcodeVersionBumpCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func selectedProjectInput(projectDir, project string) string {
	if explicitProject := strings.TrimSpace(project); explicitProject != "" {
		return explicitProject
	}
	dir := strings.TrimSpace(projectDir)
	if dir == "" {
		return "."
	}
	return dir
}

func xcodeVersionViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("view", flag.ExitOnError)

	projectDir := fs.String("project-dir", ".", "Path to directory containing .xcodeproj")
	project := fs.String("project", "", "Path to a specific .xcodeproj (preferred when the directory contains multiple projects)")
	target := fs.String("target", "", "Xcode target name (for multi-target projects)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc xcode version view [--project XCODEPROJ] [--project-dir DIR] [--target NAME]",
		ShortHelp:  "View current version and build number.",
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
			}

			result, err := runGetVersion(ctx, selectedProjectInput(*projectDir, *project), strings.TrimSpace(*target))
			if err != nil {
				return fmt.Errorf("xcode version view: %w", err)
			}

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error {
					fmt.Printf("Version: %s\n", result.Version)
					fmt.Printf("Build:   %s\n", result.BuildNumber)
					return nil
				},
				func() error {
					fmt.Printf("**Version:** %s\n\n**Build:** %s\n", result.Version, result.BuildNumber)
					return nil
				},
			)
		},
	}
}

func xcodeVersionEditCommand() *ffcli.Command {
	fs := flag.NewFlagSet("edit", flag.ExitOnError)

	projectDir := fs.String("project-dir", ".", "Path to directory containing .xcodeproj")
	project := fs.String("project", "", "Path to a specific .xcodeproj (preferred when the directory contains multiple projects)")
	version := fs.String("version", "", "Marketing version (CFBundleShortVersionString)")
	buildNumber := fs.String("build-number", "", "Build number (CFBundleVersion)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "edit",
		ShortUsage: "asc xcode version edit [--version VER] [--build-number NUM] [--project XCODEPROJ] [--project-dir DIR]",
		ShortHelp:  "Edit version and/or build number.",
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
			}

			v := strings.TrimSpace(*version)
			b := strings.TrimSpace(*buildNumber)
			if v == "" && b == "" {
				return shared.UsageError("--version or --build-number is required")
			}

			result, err := runSetVersion(ctx, localxcode.SetVersionOptions{
				ProjectDir:  selectedProjectInput(*projectDir, *project),
				Version:     v,
				BuildNumber: b,
			})
			if err != nil {
				return fmt.Errorf("xcode version edit: %w", err)
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func xcodeVersionBumpCommand() *ffcli.Command {
	fs := flag.NewFlagSet("bump", flag.ExitOnError)

	projectDir := fs.String("project-dir", ".", "Path to directory containing .xcodeproj")
	project := fs.String("project", "", "Path to a specific .xcodeproj (preferred when the directory contains multiple projects)")
	target := fs.String("target", "", "Xcode target name to use when reading the current version/build in multi-target projects")
	bumpType := fs.String("type", "", "Bump type: major, minor, patch, or build (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "bump",
		ShortUsage: "asc xcode version bump --type TYPE [--project XCODEPROJ] [--project-dir DIR] [--target NAME]",
		ShortHelp:  "Increment version or build number.",
		LongHelp: `Increment the version or build number in an Xcode project.

Bump types:
  major   1.2.3 → 2.0.0
  minor   1.2.3 → 1.3.0
  patch   1.2.3 → 1.2.4
  build   Increment CFBundleVersion (build number)

Note:
  --project selects a specific .xcodeproj when the containing directory has
  multiple sibling projects.
  --target is only used to choose which target's current version/build should be
  read as the bump baseline in multi-target projects. The write still updates the
  whole project, matching agvtool behavior.

Examples:
  asc xcode version bump --type patch
  asc xcode version bump --type patch --project ./MyApp/App.xcodeproj
  asc xcode version bump --type patch --target Extension
  asc xcode version bump --type minor --project-dir ./MyApp
  asc xcode version bump --type build`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
			}

			parsed, err := localxcode.ParseBumpType(*bumpType)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			result, err := runBumpVersion(ctx, localxcode.BumpVersionOptions{
				ProjectDir: selectedProjectInput(*projectDir, *project),
				Target:     strings.TrimSpace(*target),
				BumpType:   parsed,
			})
			if err != nil {
				return fmt.Errorf("xcode version bump: %w", err)
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
