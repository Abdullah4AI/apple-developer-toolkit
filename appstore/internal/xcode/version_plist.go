package xcode

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bitrise-io/go-plist"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/infoplist"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/rootfs"
)

func (project *structuredVersionProject) versionInfoPlistWarnings(opts SetVersionOptions, selected []*versionConfiguration, configFiles, lexicalConfigPaths map[string][]string) []string {
	resolver := newSigningSettingResolver(project, configFiles, opts.AllowExternalXCConfig, lexicalConfigPaths)
	root, rootErr := rootfs.New(project.rootDir)
	if rootErr == nil {
		defer root.Close()
	}
	var warnings []string
	for _, configuration := range effectiveMutationConfigurations(selected) {
		scope := fmt.Sprintf("target %q configuration %q", configuration.target, configuration.name)
		path, _, err := resolver.resolveSetting(configuration, "INFOPLIST_FILE")
		if errors.Is(err, errVersionSettingNotFound) || (err == nil && strings.TrimSpace(path) == "") {
			continue
		}
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("could not verify Info.plist for %s: %q", scope, err.Error()))
			continue
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(project.rootDir, path)
		}
		var data []byte
		err = rootErr
		if err == nil {
			data, err = root.ReadFileLimited(path, infoplist.MaxBytes)
		}
		if err == nil {
			err = infoplist.ValidateStructure(data)
		}
		var values map[string]any
		if err == nil {
			_, err = plist.Unmarshal(data, &values)
		}
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("could not verify Info.plist %q for %s: %q", path, scope, err.Error()))
			continue
		}
		archiveAdvice := "Verify the final archive's bundle version."
		generated, _, _ := resolver.resolveSetting(configuration, "GENERATE_INFOPLIST_FILE")
		if strings.EqualFold(strings.TrimSpace(generated), "YES") {
			archiveAdvice = "GENERATE_INFOPLIST_FILE is YES; generated and source values may be combined. Verify the final archive's bundle version."
		}
		for _, check := range []struct{ key, setting, value string }{
			{"CFBundleShortVersionString", marketingVersionSetting, strings.TrimSpace(opts.Version)},
			{"CFBundleVersion", currentProjectSetting, strings.TrimSpace(opts.BuildNumber)},
		} {
			if check.value == "" {
				continue
			}
			raw, exists := values[check.key]
			if !exists {
				continue
			}
			literal, ok := raw.(string)
			if !ok {
				warnings = append(warnings, fmt.Sprintf("could not verify Info.plist %q for %s: %s is not a string", path, scope, check.key))
				continue
			}
			if strings.Contains(literal, "$(") || strings.Contains(literal, "${") {
				continue
			}
			warnings = append(warnings, fmt.Sprintf("Info.plist %q for %s contains literal %s=%q rather than a build-setting reference; %s was set to %q. %s", path, scope, check.key, literal, check.setting, check.value, archiveAdvice))
		}
	}
	return warnings
}
