package asc

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestScreenshotFrameBatchResultRendersSummaryAndFiles(t *testing.T) {
	result := &ScreenshotFrameBatchResult{
		InputDir:   "/raw",
		OutputDir:  "/framed",
		Device:     "ipad-pro-13",
		FrameColor: "space-gray",
		Resume:     true,
		Total:      2,
		Framed:     1,
		Failed:     1,
		Files: []ScreenshotFrameBatchFile{
			{Input: "/raw/a.png", Path: "/framed/a-ipad-pro-13.png", Status: ScreenshotFrameBatchStatusFramed, DisplayType: "APP_IPAD_PRO_3GEN_129", Width: 2064, Height: 2752},
			{Input: "/raw/b.png", Path: "/framed/b-ipad-pro-13.png", Status: ScreenshotFrameBatchStatusFailed, Error: "koubou generation failed:\n font missing"},
		},
	}

	var tables [][][]string
	var headers [][]string
	if err := renderByRegistry(result, func(h []string, rows [][]string) {
		headers = append(headers, h)
		tables = append(tables, rows)
	}); err != nil {
		t.Fatal(err)
	}
	if len(tables) != 2 {
		t.Fatalf("rendered %d tables, want summary and files", len(tables))
	}
	if !slices.Equal(tables[0][0], []string{"/raw", "/framed", "ipad-pro-13", "space-gray", "true", "2", "1", "0", "1"}) {
		t.Fatalf("summary row = %v (headers %v)", tables[0][0], headers[0])
	}
	if !slices.Equal(tables[1][0], []string{"/raw/a.png", "framed", "/framed/a-ipad-pro-13.png", "APP_IPAD_PRO_3GEN_129", "2064x2752", ""}) {
		t.Fatalf("framed row = %v", tables[1][0])
	}
	if tables[1][1][5] != "koubou generation failed: font missing" {
		t.Fatalf("failed row error = %q", tables[1][1][5])
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"inputDir", "outputDir", "device", "frameColor", "resume", "total", "framed", "skipped", "failed", "files"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("receipt JSON missing %q: %s", key, data)
		}
	}
}
