package validation

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// SummarizeDeepChecks counts deep checks by terminal status.
func SummarizeDeepChecks(checks []DeepCheck) DeepSummary {
	summary := DeepSummary{}
	for _, check := range checks {
		switch check.Status {
		case DeepStatusPassed:
			summary.Passed++
		case DeepStatusBlocked:
			summary.Blocked++
		case DeepStatusUnverified:
			summary.Unverified++
		case DeepStatusNotApplicable:
			summary.NotApplicable++
		}
	}
	return summary
}

// ApplyDeepValidation merges deep evidence into a public readiness report and
// rebuilds all derived counts and remediation steps.
func ApplyDeepValidation(report Report, deep DeepReport, findings []CheckResult) Report {
	privacyVerified := false
	for _, check := range deep.Checks {
		if check.ID == DeepCheckPrivacyPublishState && (check.Status == DeepStatusPassed || check.Status == DeepStatusBlocked) {
			privacyVerified = true
			break
		}
	}
	checks := make([]CheckResult, 0, len(report.Checks)+len(findings))
	for _, check := range report.Checks {
		if check.ID == privacyPublishStateUnverifiedID && privacyVerified {
			continue
		}
		checks = append(checks, check)
	}
	checks = append(checks, findings...)

	for index := range checks {
		if checks[index].Resolution != nil {
			continue
		}
		if strings.TrimSpace(checks[index].Remediation) == "" && checks[index].Severity == SeverityInfo {
			continue
		}
		checks[index].Resolution = resolutionForPublicFinding(report, checks[index])
	}

	deep.Summary = SummarizeDeepChecks(deep.Checks)
	report.Checks = checks
	report.Summary = summarize(checks, report.Strict)
	report.Remediation = BuildRemediation(checks, report.Strict)
	report.Deep = &deep
	return report
}

func resolutionForPublicFinding(report Report, check CheckResult) *Resolution {
	appURL := defaultManualResolution(report.AppID, check.ID).AppStoreConnectURL
	command := publicAPIResolutionCommand(report, check)
	if command == "" {
		return &Resolution{Fixability: FixabilityManual, AppStoreConnectURL: appURL}
	}
	return &Resolution{
		Fixability:         FixabilityAPIFixable,
		Commands:           []string{command},
		AppStoreConnectURL: appURL,
	}
}

func publicAPIResolutionCommand(report Report, check CheckResult) string {
	resourceID := strings.TrimSpace(check.ResourceID)
	field := strings.TrimSpace(check.Field)
	switch check.ID {
	case "legal.required.copyright":
		if versionID := strings.TrimSpace(report.VersionID); versionID != "" {
			return fmt.Sprintf("asc versions update --version-id %q --copyright %q", versionID, fmt.Sprintf("%d Your Company", time.Now().UTC().Year()))
		}
	case "content_rights.missing", "content_rights.invalid":
		if appID := strings.TrimSpace(report.AppID); appID != "" {
			return fmt.Sprintf("asc apps update --id %q --content-rights %q", appID, "DECLARATION")
		}
	case "review_details.missing":
		if versionID := strings.TrimSpace(report.VersionID); versionID != "" {
			return fmt.Sprintf("asc review details-create --version-id %q --contact-first-name %q --contact-last-name %q --contact-email %q --contact-phone %q", versionID, "FIRST_NAME", "LAST_NAME", "EMAIL", "PHONE")
		}
	case "categories.primary_missing":
		if appID := strings.TrimSpace(report.AppID); appID != "" {
			command := fmt.Sprintf("asc categories set --app %q", appID)
			if resourceID != "" {
				command += fmt.Sprintf(" --app-info %q", resourceID)
			}
			return command + fmt.Sprintf(" --primary %q", "CATEGORY_ID")
		}
	case "build.required.missing":
		if versionID := strings.TrimSpace(report.VersionID); versionID != "" {
			return fmt.Sprintf("asc versions attach-build --version-id %q --build-id %q", versionID, "BUILD_ID")
		}
	}

	if resourceID == "" {
		return ""
	}
	flag, example := localizationResolutionField(field)
	switch check.ResourceType {
	case "appStoreVersionLocalization":
		if flag != "" {
			return fmt.Sprintf("asc localizations update --id %q --%s %q", resourceID, flag, example)
		}
	case "appInfoLocalization":
		if flag != "" {
			return fmt.Sprintf("asc localizations update --type app-info --id %q --%s %q", resourceID, flag, example)
		}
	case "appStoreReviewDetail":
		if flag, example := reviewDetailsResolutionField(field); flag != "" {
			return fmt.Sprintf("asc review details-update --id %q --%s %q", resourceID, flag, example)
		}
	}
	return ""
}

func localizationResolutionField(field string) (string, string) {
	switch field {
	case "description":
		return "description", "DESCRIPTION"
	case "keywords":
		return "keywords", "KEYWORDS"
	case "supportUrl":
		return "support-url", "https://example.com/support"
	case "whatsNew":
		return "whats-new", "WHAT'S NEW"
	case "marketingUrl":
		return "marketing-url", "https://example.com"
	case "name":
		return "name", "APP NAME"
	case "subtitle":
		return "subtitle", "SUBTITLE"
	case "privacyPolicyUrl":
		return "privacy-policy-url", "https://example.com/privacy"
	case "privacyChoicesUrl":
		return "privacy-choices-url", "https://example.com/privacy-choices"
	default:
		return "", ""
	}
}

func reviewDetailsResolutionField(field string) (string, string) {
	switch field {
	case "contactFirstName":
		return "contact-first-name", "FIRST_NAME"
	case "contactLastName":
		return "contact-last-name", "LAST_NAME"
	case "contactEmail":
		return "contact-email", "EMAIL"
	case "contactPhone":
		return "contact-phone", "PHONE"
	case "demoAccountName":
		return "demo-account-name", "DEMO_ACCOUNT_NAME"
	case "demoAccountPassword":
		return "demo-account-password", "DEMO_ACCOUNT_PASSWORD"
	default:
		return "", ""
	}
}

func defaultManualResolution(appID, checkID string) *Resolution {
	appURL := appStoreConnectAppURL(appID, "")
	switch {
	case strings.HasPrefix(checkID, "availability."), strings.HasPrefix(checkID, "pricing."):
		appURL = appStoreConnectAppURL(appID, "appstore/pricing")
	case strings.HasPrefix(checkID, "review_details."):
		appURL = appStoreConnectAppURL(appID, "appstore/review")
	case strings.HasPrefix(checkID, "privacy."):
		appURL = appStoreConnectAppURL(appID, "appPrivacy")
	}
	return &Resolution{
		Fixability:         FixabilityManual,
		AppStoreConnectURL: appURL,
	}
}

func appStoreConnectAppURL(appID, suffix string) string {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return ""
	}
	base := "https://appstoreconnect.apple.com/apps/" + url.PathEscape(appID)
	if strings.TrimSpace(suffix) == "" {
		return base
	}
	return base + "/" + strings.TrimPrefix(strings.TrimSpace(suffix), "/")
}
