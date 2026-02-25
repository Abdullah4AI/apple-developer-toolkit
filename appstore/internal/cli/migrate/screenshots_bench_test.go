package migrate

import "testing"

func BenchmarkInferScreenshotDisplayTypeFromDimensions(b *testing.B) {
	const path = "example-iphone-6.9.png"
	const width = 1320
	const height = 2868

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		displayType, err := inferScreenshotDisplayTypeFromDimensions(path, width, height)
		if err != nil {
			b.Fatalf("inferScreenshotDisplayTypeFromDimensions() error: %v", err)
		}
		if displayType != "APP_IPHONE_69" {
			b.Fatalf("displayType = %q, want %q", displayType, "APP_IPHONE_69")
		}
	}
}
