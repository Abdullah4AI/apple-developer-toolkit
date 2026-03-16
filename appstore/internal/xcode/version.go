package xcode

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// BumpType represents the version component to increment.
type BumpType string

const (
	BumpMajor BumpType = "major"
	BumpMinor BumpType = "minor"
	BumpPatch BumpType = "patch"
	BumpBuild BumpType = "build"
)

// ParseBumpType validates and normalizes a bump type string.
func ParseBumpType(s string) (BumpType, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "major":
		return BumpMajor, nil
	case "minor":
		return BumpMinor, nil
	case "patch":
		return BumpPatch, nil
	case "build":
		return BumpBuild, nil
	default:
		return "", fmt.Errorf("--type must be one of: major, minor, patch, build")
	}
}

// VersionInfo holds the current version and build number from an Xcode project.
type VersionInfo struct {
	Version     string `json:"version"`
	BuildNumber string `json:"buildNumber"`
	ProjectDir  string `json:"projectDir"`
	Target      string `json:"target,omitempty"`
	Modern      bool   `json:"modern"` // true if project uses MARKETING_VERSION build setting
}

// SetVersionOptions configures what to set.
type SetVersionOptions struct {
	ProjectDir  string
	Target      string
	Version     string
	BuildNumber string
}

// SetVersionResult holds the result of a set operation.
type SetVersionResult struct {
	Version     string `json:"version,omitempty"`
	BuildNumber string `json:"buildNumber,omitempty"`
	ProjectDir  string `json:"projectDir"`
}

// BumpVersionOptions configures the bump operation.
type BumpVersionOptions struct {
	ProjectDir string
	Target     string
	BumpType   BumpType
}

// BumpVersionResult holds the result of a bump operation.
type BumpVersionResult struct {
	BumpType   string `json:"bumpType"`
	OldVersion string `json:"oldVersion,omitempty"`
	NewVersion string `json:"newVersion,omitempty"`
	OldBuild   string `json:"oldBuild,omitempty"`
	NewBuild   string `json:"newBuild,omitempty"`
	ProjectDir string `json:"projectDir"`
}

func resolvedProjectDir(projectDir string) string {
	trimmed := strings.TrimSpace(projectDir)
	if trimmed == "" {
		return "."
	}
	if strings.HasSuffix(trimmed, ".xcodeproj") {
		return filepath.Dir(trimmed)
	}
	return trimmed
}

// GetVersion reads the current marketing version and build number.
func GetVersion(ctx context.Context, projectDir, target string) (*VersionInfo, error) {
	if err := requireMacOS(); err != nil {
		return nil, err
	}
	if err := requireAgvtool(); err != nil {
		return nil, err
	}

	version, err := runAgvtool(ctx, projectDir, "what-marketing-version", "-terse1")
	if err != nil {
		return nil, fmt.Errorf("failed to read marketing version: %w", err)
	}

	buildNumber, err := runAgvtool(ctx, projectDir, "what-version", "-terse")
	if err != nil {
		return nil, fmt.Errorf("failed to read build number: %w", err)
	}

	trimmedTarget := strings.TrimSpace(target)
	parsedVersion, err := parseAgvtoolVersionOutput(version, trimmedTarget)
	if err != nil {
		return nil, fmt.Errorf("failed to parse marketing version: %w", err)
	}
	parsedBuild, err := parseAgvtoolBuildOutput(buildNumber, trimmedTarget)
	if err != nil {
		return nil, fmt.Errorf("failed to parse build number: %w", err)
	}
	modern := isVariableReference(parsedVersion)

	// Modern project: agvtool returns $(MARKETING_VERSION). Resolve via xcodebuild.
	if modern {
		resolved, err := readBuildSettings(ctx, projectDir, target)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve build settings: %w", err)
		}
		if v := resolved["MARKETING_VERSION"]; v != "" {
			parsedVersion = v
		}
		if b := resolved["CURRENT_PROJECT_VERSION"]; b != "" {
			parsedBuild = b
		}
	}

	return &VersionInfo{
		Version:     parsedVersion,
		BuildNumber: parsedBuild,
		ProjectDir:  resolvedProjectDir(projectDir),
		Target:      target,
		Modern:      modern,
	}, nil
}

// SetVersion sets the marketing version and/or build number.
func SetVersion(ctx context.Context, opts SetVersionOptions) (*SetVersionResult, error) {
	if err := requireMacOS(); err != nil {
		return nil, err
	}
	if err := requireAgvtool(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(opts.Target) != "" {
		return nil, fmt.Errorf("--target is only supported by xcode version view; edit updates the whole project")
	}

	result := &SetVersionResult{ProjectDir: resolvedProjectDir(opts.ProjectDir)}

	versionOutput, err := runAgvtool(ctx, opts.ProjectDir, "what-marketing-version", "-terse1")
	if err != nil {
		return nil, fmt.Errorf("failed to read marketing version: %w", err)
	}
	modern := isModernAgvtoolOutput(versionOutput)

	if v := strings.TrimSpace(opts.Version); v != "" {
		if modern {
			if err := updatePbxprojSetting(opts.ProjectDir, "MARKETING_VERSION", v); err != nil {
				return nil, fmt.Errorf("failed to set marketing version: %w", err)
			}
		} else {
			if _, err := runAgvtool(ctx, opts.ProjectDir, "new-marketing-version", v); err != nil {
				return nil, fmt.Errorf("failed to set marketing version: %w", err)
			}
		}
		result.Version = v
	}

	if b := strings.TrimSpace(opts.BuildNumber); b != "" {
		if modern {
			if err := updatePbxprojSetting(opts.ProjectDir, "CURRENT_PROJECT_VERSION", b); err != nil {
				return nil, fmt.Errorf("failed to set build number: %w", err)
			}
		} else {
			if _, err := runAgvtool(ctx, opts.ProjectDir, "new-version", "-all", b); err != nil {
				return nil, fmt.Errorf("failed to set build number: %w", err)
			}
		}
		result.BuildNumber = b
	}

	return result, nil
}

// BumpVersion increments the version or build number.
func BumpVersion(ctx context.Context, opts BumpVersionOptions) (*BumpVersionResult, error) {
	if err := requireMacOS(); err != nil {
		return nil, err
	}
	if err := requireAgvtool(); err != nil {
		return nil, err
	}
	trimmedTarget := strings.TrimSpace(opts.Target)

	current, err := GetVersion(ctx, opts.ProjectDir, trimmedTarget)
	if err != nil {
		return nil, err
	}

	result := &BumpVersionResult{
		BumpType:   string(opts.BumpType),
		ProjectDir: resolvedProjectDir(opts.ProjectDir),
	}

	if opts.BumpType == BumpBuild {
		result.OldBuild = current.BuildNumber
		if current.Modern {
			newBuild, err := incrementBuildString(current.BuildNumber)
			if err != nil {
				return nil, fmt.Errorf("failed to increment build number: %w", err)
			}
			if err := updatePbxprojSetting(opts.ProjectDir, "CURRENT_PROJECT_VERSION", newBuild); err != nil {
				return nil, fmt.Errorf("failed to set build number: %w", err)
			}
			result.NewBuild = newBuild
		} else {
			if _, err := runAgvtool(ctx, opts.ProjectDir, "next-version", "-all"); err != nil {
				return nil, fmt.Errorf("failed to increment build number: %w", err)
			}
			updated, err := GetVersion(ctx, opts.ProjectDir, trimmedTarget)
			if err != nil {
				return nil, fmt.Errorf("failed to read updated build number: %w", err)
			}
			result.NewBuild = updated.BuildNumber
		}
		return result, nil
	}

	// Version bump (major/minor/patch).
	result.OldVersion = current.Version
	newVersion, err := bumpVersionString(current.Version, opts.BumpType)
	if err != nil {
		return nil, err
	}

	if current.Modern {
		if err := updatePbxprojSetting(opts.ProjectDir, "MARKETING_VERSION", newVersion); err != nil {
			return nil, fmt.Errorf("failed to set marketing version: %w", err)
		}
	} else {
		if _, err := runAgvtool(ctx, opts.ProjectDir, "new-marketing-version", newVersion); err != nil {
			return nil, fmt.Errorf("failed to set marketing version: %w", err)
		}
	}
	result.NewVersion = newVersion

	return result, nil
}

func requireMacOS() error {
	if runtimeGOOS != "darwin" {
		return fmt.Errorf("xcode version commands require macOS")
	}
	return nil
}

func requireAgvtool() error {
	_, err := lookPathFn("agvtool")
	if err != nil {
		return fmt.Errorf("agvtool not found: install Xcode command-line tools")
	}
	return nil
}

func runAgvtool(ctx context.Context, projectDir string, args ...string) (string, error) {
	cmd := commandContextFn(ctx, "agvtool", args...)
	cmd.Dir = resolvedProjectDir(projectDir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrText := strings.TrimSpace(stderr.String())
		if stderrText != "" {
			return "", fmt.Errorf("%w: %s", err, stderrText)
		}
		return "", err
	}

	return stdout.String(), nil
}

// readBuildSettings runs xcodebuild -showBuildSettings and extracts key=value pairs.
// If target is non-empty, scopes to that target for deterministic results in
// multi-target projects.
func readBuildSettings(ctx context.Context, projectDir, target string) (map[string]string, error) {
	xcodeproj, err := findXcodeproj(projectDir)
	if err != nil {
		return nil, err
	}

	args := []string{"-showBuildSettings", "-project", filepath.Base(xcodeproj)}
	if t := strings.TrimSpace(target); t != "" {
		args = append(args, "-target", t)
	}
	cmd := commandContextFn(ctx, "xcodebuild", args...)
	cmd.Dir = resolvedProjectDir(projectDir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrText := strings.TrimSpace(stderr.String())
		if stderrText != "" {
			return nil, fmt.Errorf("%w: %s", err, stderrText)
		}
		return nil, err
	}

	buildSettingsOutput := stdout.String()
	if strings.TrimSpace(target) == "" {
		targets := buildSettingsTargetNames(buildSettingsOutput)
		if len(targets) > 1 {
			return nil, fmt.Errorf("multiple Xcode targets found in build settings (%s); use --target", strings.Join(targets, ", "))
		}
	}

	settings := make(map[string]string)
	for _, line := range strings.Split(buildSettingsOutput, "\n") {
		trimmed := strings.TrimSpace(line)
		if idx := strings.Index(trimmed, " = "); idx > 0 {
			key := strings.TrimSpace(trimmed[:idx])
			value := strings.TrimSpace(trimmed[idx+3:])
			// Keep the first occurrence only — in multi-target projects,
			// the first target block is typically the main app target.
			if _, exists := settings[key]; !exists {
				settings[key] = value
			}
		}
	}
	return settings, nil
}

func buildSettingsTargetNames(output string) []string {
	seen := make(map[string]struct{})
	var targets []string
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "Build settings for action ") || !strings.HasSuffix(trimmed, ":") {
			continue
		}
		idx := strings.LastIndex(trimmed, " target ")
		if idx < 0 {
			continue
		}
		target := strings.TrimSpace(strings.TrimSuffix(trimmed[idx+len(" target "):], ":"))
		if target == "" {
			continue
		}
		if _, exists := seen[target]; exists {
			continue
		}
		seen[target] = struct{}{}
		targets = append(targets, target)
	}
	return targets
}

// findXcodeproj resolves an explicit .xcodeproj path or finds one in a project dir.
// Returns an error if zero or multiple .xcodeproj directories are found.
func findXcodeproj(projectDir string) (string, error) {
	trimmedDir := strings.TrimSpace(projectDir)
	if trimmedDir == "" {
		trimmedDir = "."
	}
	if strings.HasSuffix(trimmedDir, ".xcodeproj") {
		info, err := os.Stat(trimmedDir)
		if err != nil {
			return "", fmt.Errorf("failed to read Xcode project %s: %w", trimmedDir, err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("%s is not an .xcodeproj directory", trimmedDir)
		}
		return trimmedDir, nil
	}

	entries, err := os.ReadDir(trimmedDir)
	if err != nil {
		return "", fmt.Errorf("failed to read project directory: %w", err)
	}
	var matches []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasSuffix(entry.Name(), ".xcodeproj") {
			matches = append(matches, entry.Name())
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no .xcodeproj found in %s", trimmedDir)
	case 1:
		return filepath.Join(trimmedDir, matches[0]), nil
	default:
		return "", fmt.Errorf("multiple .xcodeproj found in %s (%s); use --project to pick one", trimmedDir, strings.Join(matches, ", "))
	}
}

// findPbxprojPath finds the project.pbxproj inside the .xcodeproj.
func findPbxprojPath(projectDir string) (string, error) {
	xcodeproj, err := findXcodeproj(projectDir)
	if err != nil {
		return "", err
	}
	pbxproj := filepath.Join(xcodeproj, "project.pbxproj")
	if _, err := os.Stat(pbxproj); err != nil {
		return "", fmt.Errorf("project.pbxproj not found in %s", xcodeproj)
	}
	return pbxproj, nil
}

// updatePbxprojSetting replaces all occurrences of a build setting in project.pbxproj.
// Matches lines like: MARKETING_VERSION = 1.2.3;
// Note: this updates all targets/configs, matching agvtool's behavior. The --target
// flag scopes reads (xcodebuild -showBuildSettings) but writes are project-wide.
func updatePbxprojSetting(projectDir, setting, newValue string) error {
	pbxprojPath, err := findPbxprojPath(projectDir)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(pbxprojPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", pbxprojPath, err)
	}

	oldContent := string(data)
	lines := strings.Split(oldContent, "\n")
	var replaced int

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, setting+" = ") && strings.HasSuffix(trimmed, ";") {
			// Preserve original indentation.
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = indent + setting + " = " + newValue + ";"
			replaced++
		}
	}

	if replaced == 0 {
		return fmt.Errorf("%s not found in %s", setting, pbxprojPath)
	}

	return os.WriteFile(pbxprojPath, []byte(strings.Join(lines, "\n")), 0o600)
}

// isVariableReference checks if a value is an Xcode variable like $(MARKETING_VERSION).
func isVariableReference(value string) bool {
	return strings.Contains(value, "$(")
}

func isModernAgvtoolOutput(output string) bool {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		value := line
		if idx := strings.Index(line, "="); idx >= 0 {
			value = strings.TrimSpace(line[idx+1:])
		}
		if isVariableReference(value) {
			return true
		}
	}
	return false
}

// parseAgvtoolVersionOutput extracts the version from agvtool output.
// `agvtool what-marketing-version -terse1` outputs lines like "=1.2.3" or "TargetName=1.2.3".
func parseAgvtoolVersionOutput(output, target string) (string, error) {
	return parseAgvtoolValueOutput(output, target)
}

// parseAgvtoolBuildOutput extracts the build number from agvtool output.
// `agvtool what-version -terse` outputs just the number or target-scoped lines.
func parseAgvtoolBuildOutput(output, target string) (string, error) {
	return parseAgvtoolValueOutput(output, target)
}

func parseAgvtoolValueOutput(output, target string) (string, error) {
	lines := strings.Split(output, "\n")
	trimmedTarget := strings.TrimSpace(target)

	var fallback string
	seenTargets := make(map[string]struct{})
	var targetNames []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if idx := strings.Index(line, "="); idx >= 0 {
			name := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			if name != "" {
				if trimmedTarget != "" && name == trimmedTarget {
					return value, nil
				}
				if _, exists := seenTargets[name]; !exists {
					seenTargets[name] = struct{}{}
					targetNames = append(targetNames, name)
				}
				continue
			}
			if fallback == "" {
				fallback = value
			}
			continue
		}
		if fallback == "" {
			fallback = line
		}
	}

	if trimmedTarget != "" {
		if len(targetNames) > 0 {
			return "", fmt.Errorf("target %q not found in agvtool output", trimmedTarget)
		}
		return fallback, nil
	}

	if len(targetNames) > 1 {
		return "", fmt.Errorf("multiple target values found (%s); use --target", strings.Join(targetNames, ", "))
	}
	if len(targetNames) == 1 {
		prefix := targetNames[0] + "="
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, prefix) {
				return strings.TrimSpace(line[len(prefix):]), nil
			}
		}
	}

	return fallback, nil
}

// bumpVersionString increments a semver-style version string.
func bumpVersionString(current string, bumpType BumpType) (string, error) {
	current = strings.TrimSpace(current)
	if current == "" {
		return "", fmt.Errorf("current version is empty")
	}

	parts := strings.Split(current, ".")
	components := make([]int, len(parts))
	for i, p := range parts {
		val, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return "", fmt.Errorf("version %q is not a valid numeric version", current)
		}
		components[i] = val
	}

	switch bumpType {
	case BumpMajor:
		if len(components) < 1 {
			return "", fmt.Errorf("version %q has no major component", current)
		}
		components[0]++
		for i := 1; i < len(components); i++ {
			components[i] = 0
		}
	case BumpMinor:
		if len(components) < 2 {
			return "", fmt.Errorf("version %q needs at least major.minor format for minor bump", current)
		}
		components[1]++
		for i := 2; i < len(components); i++ {
			components[i] = 0
		}
	case BumpPatch:
		if len(components) < 3 {
			return "", fmt.Errorf("version %q needs major.minor.patch format for patch bump", current)
		}
		components[2]++
	default:
		return "", fmt.Errorf("unsupported bump type %q for version bump", bumpType)
	}

	result := make([]string, len(components))
	for i, v := range components {
		result[i] = strconv.Itoa(v)
	}
	return strings.Join(result, "."), nil
}

// incrementBuildString increments a numeric build string by 1.
func incrementBuildString(current string) (string, error) {
	current = strings.TrimSpace(current)
	if current == "" {
		return "", fmt.Errorf("build number is empty")
	}

	// Support dotted build numbers (e.g. 1.2.3 → 1.2.4).
	parts := strings.Split(current, ".")
	last := parts[len(parts)-1]
	val, err := strconv.Atoi(last)
	if err != nil {
		return "", fmt.Errorf("build number %q is not numeric", current)
	}
	parts[len(parts)-1] = strconv.Itoa(val + 1)
	return strings.Join(parts, "."), nil
}
