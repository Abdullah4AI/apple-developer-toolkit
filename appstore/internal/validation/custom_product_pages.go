package validation

import (
	"fmt"
	"strings"
)

// customProductPageKeywordChecks warns about custom product page search
// keywords that are missing from the same-locale keywords of the version.
// Apple removes those keywords from the page when the version goes live.
func customProductPageKeywordChecks(versionLocalizations []VersionLocalization, pages []CustomProductPageLocalization, skipReason string) []CheckResult {
	if reason := strings.TrimSpace(skipReason); reason != "" {
		return []CheckResult{{
			ID:          "custom_product_pages.keywords.unverified",
			Severity:    SeverityInfo,
			Field:       "searchKeywords",
			Message:     "Could not verify custom product page search keywords against this version's keywords",
			Remediation: reason,
		}}
	}
	if len(pages) == 0 {
		return nil
	}

	versionKeywords := make(map[string]map[string]struct{}, len(versionLocalizations))
	for _, loc := range versionLocalizations {
		locale := strings.ToLower(strings.TrimSpace(loc.Locale))
		if versionKeywords[locale] == nil {
			versionKeywords[locale] = make(map[string]struct{})
		}
		for _, keyword := range strings.Split(loc.Keywords, ",") {
			if key := normalizeSearchKeyword(keyword); key != "" {
				versionKeywords[locale][key] = struct{}{}
			}
		}
	}

	checks := make([]CheckResult, 0)
	for _, page := range pages {
		locale := strings.TrimSpace(page.Locale)
		keywords := versionKeywords[strings.ToLower(locale)]
		for _, keyword := range page.SearchKeywords {
			key := normalizeSearchKeyword(keyword)
			if key == "" {
				continue
			}
			if _, ok := keywords[key]; ok {
				continue
			}
			keyword = strings.TrimSpace(keyword)
			checks = append(checks, CheckResult{
				ID:           "custom_product_pages.keywords.missing_from_version",
				Severity:     SeverityWarning,
				Locale:       locale,
				Field:        "searchKeywords",
				ResourceType: "appCustomProductPageLocalization",
				ResourceID:   strings.TrimSpace(page.LocalizationID),
				Message:      fmt.Sprintf("custom product page %q search keyword %q is not in this version's %s keywords", page.PageName, keyword, locale),
				Remediation:  fmt.Sprintf("Add %q to this version's %s keywords, or remove it from custom product page %q", keyword, locale, page.PageName),
			})
		}
	}
	return checks
}

func normalizeSearchKeyword(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}
