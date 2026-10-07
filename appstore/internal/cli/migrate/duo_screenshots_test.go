package migrate

import "testing"

func TestInferIPhoneDuoScreenshotDisplayType(t *testing.T) {
	for _, dimensions := range [][2]int{{1398, 2034}, {2034, 1398}, {2007, 2853}, {2853, 2007}} {
		got := inferDisplayTypeFromDimensions(dimensions[0], dimensions[1])
		if got != "APP_IPHONE_DUO" {
			t.Fatalf("%v inferred %q", dimensions, got)
		}
	}
	for _, name := range []string{"iphone duo-01.png", "iphone_duo-01.png", "app_iphone_duo-01.png", "iPhoneDuo-01.png"} {
		if got := inferDisplayTypeFromFilename(name); got != "APP_IPHONE_DUO" {
			t.Fatalf("%q inferred %q", name, got)
		}
	}
}
