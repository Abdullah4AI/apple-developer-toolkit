package asc

import "strings"

// XcodeVersionChange describes one concrete version build-setting mutation.
type XcodeVersionChange struct {
	Setting       string `json:"setting"`
	OldValue      string `json:"oldValue,omitempty"`
	NewValue      string `json:"newValue"`
	Target        string `json:"target,omitempty"`
	Configuration string `json:"configuration,omitempty"`
	Path          string `json:"path"`
	Source        string `json:"source"`
}

// XcodeSetVersionResult is the receipt for a local version edit.
type XcodeSetVersionResult struct {
	Version          string               `json:"version,omitempty"`
	BuildNumber      string               `json:"buildNumber,omitempty"`
	ProjectDir       string               `json:"projectDir"`
	Target           string               `json:"target,omitempty"`
	Configuration    string               `json:"configuration,omitempty"`
	VersionFilter    string               `json:"versionFilter,omitempty"`
	ComputedWarnings []string             `json:"computedWarnings,omitempty"`
	ChangedFiles     []string             `json:"changedFiles"`
	Changes          []XcodeVersionChange `json:"changes"`
}

// XcodeBumpVersionResult is the receipt for a local version bump.
type XcodeBumpVersionResult struct {
	BumpType         string               `json:"bumpType"`
	OldVersion       string               `json:"oldVersion,omitempty"`
	NewVersion       string               `json:"newVersion,omitempty"`
	OldBuild         string               `json:"oldBuild,omitempty"`
	NewBuild         string               `json:"newBuild,omitempty"`
	ProjectDir       string               `json:"projectDir"`
	Target           string               `json:"target,omitempty"`
	Configuration    string               `json:"configuration,omitempty"`
	VersionFilter    string               `json:"versionFilter,omitempty"`
	ComputedWarnings []string             `json:"computedWarnings,omitempty"`
	ChangedFiles     []string             `json:"changedFiles"`
	Changes          []XcodeVersionChange `json:"changes"`
}

func xcodeSetVersionRows(r *XcodeSetVersionResult) ([]string, [][]string) {
	return []string{"Version", "Build", "Target", "Configuration", "Changed Files", "Version Filter", "Warnings"}, [][]string{{r.Version, r.BuildNumber, r.Target, r.Configuration, strings.Join(r.ChangedFiles, ", "), r.VersionFilter, strings.Join(r.ComputedWarnings, "; ")}}
}

func xcodeBumpVersionRows(r *XcodeBumpVersionResult) ([]string, [][]string) {
	return []string{"Bump Type", "Old Version", "New Version", "Old Build", "New Build", "Changed Files", "Version Filter", "Warnings"}, [][]string{{r.BumpType, r.OldVersion, r.NewVersion, r.OldBuild, r.NewBuild, strings.Join(r.ChangedFiles, ", "), r.VersionFilter, strings.Join(r.ComputedWarnings, "; ")}}
}
