package migrate

import (
	"slices"
	"sort"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

type ImportMetadataDefaults struct {
	Version *asc.AppStoreVersionLocalizationAttributes `json:"version,omitempty"`
	AppInfo *asc.AppInfoLocalizationAttributes         `json:"appInfo,omitempty"`
	version preparedVersionLocalization
	appInfo preparedAppInfoLocalization
}

func appendDefaultVersionLocales(local []preparedVersionLocalization, defaults *ImportMetadataDefaults, remote map[string]string) []preparedVersionLocalization {
	if defaults.Version == nil {
		return local
	}
	locales := make([]string, 0, len(remote))
	for locale := range remote {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	for _, locale := range locales {
		if locale == "" || slices.ContainsFunc(local, func(item preparedVersionLocalization) bool { return item.localization.Locale == locale }) {
			continue
		}
		value := defaults.version
		value.localization.Locale = locale
		value.localization.defaultFiles = versionLocalizationFiles(defaults.version.localization)
		value.attributes.Locale = locale
		local = append(local, value)
	}
	return local
}
