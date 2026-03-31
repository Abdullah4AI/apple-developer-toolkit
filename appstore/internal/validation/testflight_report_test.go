package validation

import "testing"

func TestValidateTestFlight_MissingBuild(t *testing.T) {
	report := ValidateTestFlight(TestFlightInput{
		AppID:   "app-1",
		BuildID: "build-1",
		Build:   nil,
	}, false)

	if !hasCheckID(report.Checks, "testflight.build.missing") {
		t.Fatalf("expected testflight.build.missing check, got %v", report.Checks)
	}
}

func TestValidateTestFlight_BuildAppMismatch(t *testing.T) {
	report := ValidateTestFlight(TestFlightInput{
		AppID:      "app-1",
		BuildID:    "build-1",
		Build:      &Build{ID: "build-1", ProcessingState: "VALID"},
		BuildAppID: "app-2",
	}, false)

	if !hasCheckID(report.Checks, "testflight.build.app_mismatch") {
		t.Fatalf("expected testflight.build.app_mismatch check, got %v", report.Checks)
	}
}

func TestValidateTestFlight_MissingBetaReviewDetails(t *testing.T) {
	report := ValidateTestFlight(TestFlightInput{
		AppID:   "app-1",
		BuildID: "build-1",
		Build:   &Build{ID: "build-1", ProcessingState: "VALID"},
	}, false)

	if !hasCheckID(report.Checks, "testflight.review_details.missing") {
		t.Fatalf("expected testflight.review_details.missing check, got %v", report.Checks)
	}
}

func TestValidateTestFlight_MissingWhatsNew(t *testing.T) {
	report := ValidateTestFlight(TestFlightInput{
		AppID:            "app-1",
		AppPrimaryLocale: "en-US",
		BuildID:          "build-1",
		Build:            &Build{ID: "build-1", ProcessingState: "VALID"},
		BetaReviewDetails: &BetaReviewDetails{
			ID:               "beta-detail-1",
			ContactFirstName: "A",
			ContactLastName:  "B",
			ContactEmail:     "a@example.com",
			ContactPhone:     "123",
		},
		BetaBuildLocalizations: []BetaBuildLocalization{
			{Locale: "en-US", WhatsNew: ""},
		},
	}, false)

	if !hasCheckID(report.Checks, "testflight.whats_new.missing") {
		t.Fatalf("expected testflight.whats_new.missing check, got %v", report.Checks)
	}
}

func TestValidateTestFlight_Pass(t *testing.T) {
	report := ValidateTestFlight(TestFlightInput{
		AppID:            "app-1",
		AppPrimaryLocale: "en-US",
		BuildID:          "build-1",
		Build:            &Build{ID: "build-1", ProcessingState: "VALID", UsesNonExemptEncryption: boolPtr(false)},
		BuildAppID:       "app-1",
		BetaReviewDetails: &BetaReviewDetails{
			ID:               "beta-detail-1",
			ContactFirstName: "A",
			ContactLastName:  "B",
			ContactEmail:     "a@example.com",
			ContactPhone:     "123",
		},
		BetaBuildLocalizations: []BetaBuildLocalization{
			{Locale: "en-US", WhatsNew: "Test this build"},
		},
	}, false)

	if len(report.Checks) != 0 {
		t.Fatalf("expected no checks, got %d (%v)", len(report.Checks), report.Checks)
	}
}

func boolPtr(v bool) *bool {
	return &v
}
