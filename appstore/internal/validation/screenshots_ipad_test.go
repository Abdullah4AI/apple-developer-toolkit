package validation

import (
	"strings"
	"testing"
)

func ipadTestSets(displayTypes ...string) []ScreenshotSet {
	sets := make([]ScreenshotSet, 0, len(displayTypes))
	for _, displayType := range displayTypes {
		sets = append(sets, ScreenshotSet{
			ID:             "set-" + strings.ToLower(displayType),
			DisplayType:    displayType,
			Locale:         "en-US",
			LocalizationID: "loc-1",
			Screenshots:    validScreenshots(1),
		})
	}
	return sets
}

func TestIPadScreenshotChecks(t *testing.T) {
	supports := true
	iphoneOnly := false
	locs := []VersionLocalization{{ID: "loc-1", Locale: "en-US"}}
	twoLocs := []VersionLocalization{{ID: "loc-1", Locale: "en-US"}, {ID: "loc-2", Locale: "de-DE"}}
	secondaryIPadSet := ScreenshotSet{
		ID:             "set-de-ipad",
		DisplayType:    "APP_IPAD_PRO_3GEN_129",
		Locale:         "de-DE",
		LocalizationID: "loc-2",
		Screenshots:    validScreenshots(1),
	}

	tests := []struct {
		name          string
		platform      string
		primaryLocale string
		locs          []VersionLocalization
		sets          []ScreenshotSet
		supportsIPad  *bool
		wantID        string
		wantSeverity  Severity
		wantLocale    string
	}{
		{
			name:         "iPad build without iPad screenshots blocks",
			platform:     "IOS",
			locs:         locs,
			sets:         ipadTestSets("APP_IPHONE_65"),
			supportsIPad: &supports,
			wantID:       "screenshots.required.ipad",
			wantSeverity: SeverityError,
			wantLocale:   "en-US",
		},
		{
			name:         "iPad build with only a retired iPad slot blocks",
			platform:     "IOS",
			locs:         locs,
			sets:         ipadTestSets("APP_IPHONE_65", "APP_IPAD_PRO_129"),
			supportsIPad: &supports,
			wantID:       "screenshots.required.ipad",
			wantSeverity: SeverityError,
			wantLocale:   "en-US",
		},
		{
			name:         "iPad build with required iPad screenshots passes",
			platform:     "IOS",
			locs:         locs,
			sets:         ipadTestSets("APP_IPHONE_65", "APP_IPAD_PRO_3GEN_129"),
			supportsIPad: &supports,
		},
		{
			name:         "iPhone-only build passes without iPad screenshots",
			platform:     "IOS",
			locs:         locs,
			sets:         ipadTestSets("APP_IPHONE_65"),
			supportsIPad: &iphoneOnly,
		},
		{
			name:         "unknown device family reports info only",
			platform:     "IOS",
			locs:         locs,
			sets:         ipadTestSets("APP_IPHONE_65"),
			wantID:       "screenshots.required.ipad_unverified",
			wantSeverity: SeverityInfo,
			wantLocale:   "en-US",
		},
		{
			name:     "unknown device family with required iPad screenshots is silent",
			platform: "IOS",
			locs:     locs,
			sets:     ipadTestSets("APP_IPHONE_65", "APP_IPAD_PRO_3GEN_129"),
		},
		{
			name:         "unknown device family with only a retired iPad slot reports info",
			platform:     "IOS",
			locs:         locs,
			sets:         ipadTestSets("APP_IPHONE_65", "APP_IPAD_PRO_129"),
			wantID:       "screenshots.required.ipad_unverified",
			wantSeverity: SeverityInfo,
			wantLocale:   "en-US",
		},
		{
			name:         "unknown device family with the required set only in a secondary locale reports info",
			platform:     "IOS",
			locs:         twoLocs,
			sets:         append(ipadTestSets("APP_IPHONE_65"), secondaryIPadSet),
			wantID:       "screenshots.required.ipad_unverified",
			wantSeverity: SeverityInfo,
			wantLocale:   "en-US",
		},
		{
			name:         "non-iOS platform is skipped",
			platform:     "MAC_OS",
			locs:         locs,
			sets:         ipadTestSets("APP_DESKTOP"),
			supportsIPad: &supports,
		},
		{
			name:         "no screenshot sets defers to screenshots.required.any",
			platform:     "IOS",
			locs:         locs,
			supportsIPad: &supports,
		},
		{
			name:         "no localizations is skipped",
			platform:     "IOS",
			sets:         ipadTestSets("APP_IPHONE_65"),
			supportsIPad: &supports,
		},
		{
			name:         "required set only in a secondary locale blocks the primary locale",
			platform:     "IOS",
			locs:         twoLocs,
			sets:         append(ipadTestSets("APP_IPHONE_65"), secondaryIPadSet),
			supportsIPad: &supports,
			wantID:       "screenshots.required.ipad",
			wantSeverity: SeverityError,
			wantLocale:   "en-US",
		},
		{
			name:          "unmatched primary locale accepts any localization",
			platform:      "IOS",
			primaryLocale: "fr-FR",
			locs:          twoLocs,
			sets:          append(ipadTestSets("APP_IPHONE_65"), secondaryIPadSet),
			supportsIPad:  &supports,
		},
		{
			name:         "empty primary localization defers to localization_missing_sets",
			platform:     "IOS",
			locs:         twoLocs,
			sets:         []ScreenshotSet{{ID: "set-de", DisplayType: "APP_IPHONE_65", Locale: "de-DE", LocalizationID: "loc-2", Screenshots: validScreenshots(1)}},
			supportsIPad: &supports,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			primaryLocale := test.primaryLocale
			if primaryLocale == "" {
				primaryLocale = "en-US"
			}
			checks := ipadScreenshotChecks(test.platform, primaryLocale, test.locs, test.sets, test.supportsIPad)
			if test.wantID == "" {
				if len(checks) != 0 {
					t.Fatalf("expected no checks, got %+v", checks)
				}
				return
			}
			if len(checks) != 1 {
				t.Fatalf("expected one check, got %+v", checks)
			}
			check := checks[0]
			if check.ID != test.wantID || check.Severity != test.wantSeverity || check.Locale != test.wantLocale {
				t.Fatalf("check = %+v, want %s/%s locale %q", check, test.wantID, test.wantSeverity, test.wantLocale)
			}
			if !strings.Contains(check.Message+check.Remediation, "APP_IPAD_PRO_3GEN_129") {
				t.Fatalf("expected the required iPad display type in %+v", check)
			}
		})
	}
}

func TestValidateBlocksIPadBuildWithoutIPadScreenshots(t *testing.T) {
	supports := true
	report := Validate(Input{
		Platform:             "IOS",
		PrimaryLocale:        "en-US",
		VersionLocalizations: []VersionLocalization{{ID: "loc-1", Locale: "en-US"}},
		ScreenshotSets:       ipadTestSets("APP_IPHONE_65"),
		SupportsIPad:         &supports,
	}, false)

	if !hasCheckID(report.Checks, "screenshots.required.ipad") {
		t.Fatalf("expected screenshots.required.ipad, got %+v", report.Checks)
	}
	if report.Summary.Blocking == 0 {
		t.Fatalf("expected a blocking summary, got %+v", report.Summary)
	}
}
