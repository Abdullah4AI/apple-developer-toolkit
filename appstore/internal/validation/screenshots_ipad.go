package validation

import (
	"fmt"
	"strings"
)

// RequiredIPadScreenshotDisplayType is the iPad display type App Store Connect
// requires before an app that runs on iPad can be submitted for review.
const RequiredIPadScreenshotDisplayType = "APP_IPAD_PRO_3GEN_129"

// ipadScreenshotChecks reports a missing required iPad screenshot set.
//
// The App Store Connect API does not expose whether a build supports iPad:
// builds and build bundles carry no device-family attribute, and build bundle
// file sizes list iPad device models for iPhone-only apps too, because iPhone
// apps install on iPad. supportsIPad therefore comes from the IPA's
// UIDeviceFamily when the caller inspected one, and is nil otherwise. Only a
// known iPad build blocks; an unknown device family is informational so
// iPhone-only apps never fail validation.
//
// Other localizations fall back to the primary locale's screenshots, so the
// required set must exist in the primary localization. When the primary
// localization cannot be identified, any localization satisfies it.
func ipadScreenshotChecks(platform, primaryLocale string, versionLocs []VersionLocalization, sets []ScreenshotSet, supportsIPad *bool) []CheckResult {
	if !strings.EqualFold(strings.TrimSpace(platform), "IOS") {
		return nil
	}
	// Without localizations or screenshot sets, the presence checks already
	// report a clearer blocking error.
	if len(versionLocs) == 0 || len(sets) == 0 {
		return nil
	}

	var primary *VersionLocalization
	if locale := strings.TrimSpace(primaryLocale); locale != "" {
		for index := range versionLocs {
			if strings.EqualFold(versionLocs[index].Locale, locale) && strings.TrimSpace(versionLocs[index].ID) != "" {
				primary = &versionLocs[index]
				break
			}
		}
	}

	// Only the required display type in the required scope satisfies Apple; a
	// retired iPad slot or a set in a secondary locale does not.
	hasRequiredIPad := false
	hasIPhone := false
	requiredScopeSets := 0
	for _, set := range sets {
		inRequiredScope := primary == nil || strings.TrimSpace(set.LocalizationID) == strings.TrimSpace(primary.ID)
		if inRequiredScope {
			requiredScopeSets++
		}
		displayType := strings.ToUpper(strings.TrimSpace(set.DisplayType))
		switch {
		case displayType == RequiredIPadScreenshotDisplayType:
			hasRequiredIPad = hasRequiredIPad || inRequiredScope
		case strings.HasPrefix(displayType, "APP_IPHONE_"):
			hasIPhone = true
		}
	}

	// An empty primary localization is already reported by
	// screenshots.required.localization_missing_sets.
	if hasRequiredIPad || requiredScopeSets == 0 {
		return nil
	}

	if supportsIPad == nil {
		if !hasIPhone {
			return nil
		}
		check := CheckResult{
			ID:           "screenshots.required.ipad_unverified",
			Severity:     SeverityInfo,
			ResourceType: "appScreenshotSet",
			Message:      fmt.Sprintf("iPhone screenshots exist but no %s screenshot set was found; whether the build runs on iPad could not be determined from the App Store Connect API", RequiredIPadScreenshotDisplayType),
			Remediation:  fmt.Sprintf("If the app supports iPad (UIDeviceFamily includes 2), upload %s screenshots before submitting; rerun with --ipa PATH to check the build", RequiredIPadScreenshotDisplayType),
		}
		if primary != nil {
			check.Locale = primary.Locale
			check.ResourceType = "appStoreVersionLocalization"
			check.ResourceID = primary.ID
			check.Message = fmt.Sprintf("iPhone screenshots exist but the primary locale has no %s screenshot set; whether the build runs on iPad could not be determined from the App Store Connect API", RequiredIPadScreenshotDisplayType)
		}
		return []CheckResult{check}
	}

	if !*supportsIPad {
		return nil
	}
	check := CheckResult{
		ID:           "screenshots.required.ipad",
		Severity:     SeverityError,
		ResourceType: "appScreenshotSet",
		Message:      fmt.Sprintf("the build supports iPad (UIDeviceFamily includes 2) but no %s screenshot set was found", RequiredIPadScreenshotDisplayType),
		Remediation:  fmt.Sprintf("Upload %s screenshots, or remove iPad from the app's supported devices and upload a new build", RequiredIPadScreenshotDisplayType),
	}
	if primary != nil {
		check.Locale = primary.Locale
		check.ResourceType = "appStoreVersionLocalization"
		check.ResourceID = primary.ID
		check.Message = fmt.Sprintf("the build supports iPad (UIDeviceFamily includes 2) but the primary locale has no %s screenshot set", RequiredIPadScreenshotDisplayType)
		check.Remediation = fmt.Sprintf("Upload %s screenshots for the primary locale, or remove iPad from the app's supported devices and upload a new build", RequiredIPadScreenshotDisplayType)
	}
	return []CheckResult{check}
}
