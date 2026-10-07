package asc

import "testing"

func TestValidateIPhoneDuoDimensions(t *testing.T) {
	for _, dimensions := range [][2]int{{1398, 2034}, {2034, 1398}, {2007, 2853}, {2853, 2007}} {
		if err := ValidateScreenshotDimensionsForSize("duo.png", dimensions[0], dimensions[1], "APP_IPHONE_DUO"); err != nil {
			t.Fatal(err)
		}
	}
	if err := ValidateScreenshotDimensionsForSize("other.png", 1320, 2868, "APP_IPHONE_DUO"); err == nil {
		t.Fatal("accepted standard iPhone screenshot in Duo slot")
	}
	if got := CanonicalScreenshotDisplayTypeForAPI("APP_IPHONE_DUO"); got != "APP_IPHONE_DUO" {
		t.Fatalf("canonical type = %q", got)
	}
}
