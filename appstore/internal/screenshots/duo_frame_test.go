package screenshots

import "testing"

func TestDisplayTypeForIPhoneDuoDimensions(t *testing.T) {
	for _, dimensions := range [][2]int{{1398, 2034}, {2034, 1398}, {2007, 2853}, {2853, 2007}} {
		got, ok := displayTypeForDimensions(dimensions[0], dimensions[1])
		if !ok || got != "APP_IPHONE_DUO" {
			t.Fatalf("%v inferred %q, %v", dimensions, got, ok)
		}
	}
}
