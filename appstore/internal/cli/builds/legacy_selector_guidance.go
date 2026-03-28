package builds

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

type trackedStringFlag struct {
	value string
	set   bool
}

func (f *trackedStringFlag) String() string {
	if f == nil {
		return ""
	}
	return f.value
}

func (f *trackedStringFlag) Set(value string) error {
	f.value = value
	f.set = true
	return nil
}

func (f *trackedStringFlag) Used() bool {
	return f != nil && f.set
}

func (f *trackedStringFlag) Value() string {
	if f == nil {
		return ""
	}
	return strings.TrimSpace(f.value)
}

type trackedBoolFlag struct {
	value bool
	set   bool
}

func (f *trackedBoolFlag) String() string {
	if f == nil {
		return "false"
	}
	return strconv.FormatBool(f.value)
}

func (f *trackedBoolFlag) Set(value string) error {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return err
	}
	f.value = parsed
	f.set = true
	return nil
}

func (f *trackedBoolFlag) IsBoolFlag() bool {
	return true
}

func (f *trackedBoolFlag) Used() bool {
	return f != nil && f.set
}

func bindHiddenStringFlag(fs *flag.FlagSet, name string) *trackedStringFlag {
	value := &trackedStringFlag{}
	fs.Var(value, name, "DEPRECATED: use --build-id")
	shared.HideFlagFromHelp(fs.Lookup(name))
	return value
}

func bindHiddenBoolFlag(fs *flag.FlagSet, name string) *trackedBoolFlag {
	value := &trackedBoolFlag{}
	fs.Var(value, name, "DEPRECATED: use --latest")
	shared.HideFlagFromHelp(fs.Lookup(name))
	return value
}

func flagWasProvided(fs *flag.FlagSet, name string) bool {
	if fs == nil {
		return false
	}

	used := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			used = true
		}
	})
	return used
}

const (
	legacyBuildIDWarning                     = "Warning: `--build` is deprecated. Use `--build-id`."
	legacyIDWarning                          = "Warning: `--id` as a build selector is deprecated. Use `--build-id`."
	legacyNewestWarning                      = "Warning: `--newest` is deprecated. Use `--latest`."
	legacyImplicitBuildNumberPlatformWarning = "Warning: omitting --platform with app-scoped --build-number selection is deprecated. Defaulting to IOS; pass --platform IOS explicitly."
)

func applyLegacyBuildIDAlias(buildID *string, legacyBuildID *trackedStringFlag) error {
	return applyLegacyStringAlias(buildID, legacyBuildID, "--build", "--build-id", legacyBuildIDWarning)
}

func applyLegacyIDAlias(buildID *string, legacyID *trackedStringFlag) error {
	return applyLegacyStringAlias(buildID, legacyID, "--id", "--build-id", legacyIDWarning)
}

func applyLegacyLatestAlias(latest *bool, latestProvided bool, legacyNewest *trackedBoolFlag) error {
	if latest == nil || legacyNewest == nil || !legacyNewest.Used() {
		return nil
	}
	if latestProvided && *latest != legacyNewest.value {
		return shared.UsageError("--newest conflicts with --latest; use only --latest")
	}
	if !latestProvided {
		*latest = legacyNewest.value
	}
	fmt.Fprintln(os.Stderr, legacyNewestWarning)
	return nil
}

func applyLegacyStringAlias(canonical *string, legacy *trackedStringFlag, legacyName, canonicalName, warning string) error {
	if canonical == nil || legacy == nil || !legacy.Used() {
		return nil
	}

	legacyValue := legacy.Value()
	canonicalValue := strings.TrimSpace(*canonical)
	if canonicalValue != "" && legacyValue != "" && canonicalValue != legacyValue {
		return shared.UsageErrorf("%s conflicts with %s; use only %s", legacyName, canonicalName, canonicalName)
	}
	if canonicalValue == "" {
		*canonical = legacyValue
	}

	fmt.Fprintln(os.Stderr, warning)
	return nil
}

func applyLegacyImplicitBuildNumberPlatformDefault(buildNumber, platform string) string {
	if strings.TrimSpace(buildNumber) == "" || strings.TrimSpace(platform) != "" {
		return platform
	}

	fmt.Fprintln(os.Stderr, legacyImplicitBuildNumberPlatformWarning)
	return "IOS"
}
