package asc

import "fmt"

// ScreenshotFrameBatchFile is one input of a `screenshots frame --input-dir` run.
type ScreenshotFrameBatchFile struct {
	Input       string `json:"input"`
	Path        string `json:"path"`
	Status      string `json:"status"`
	FramePath   string `json:"framePath,omitempty"`
	DisplayType string `json:"displayType,omitempty"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	Error       string `json:"error,omitempty"`
}

// Batch file statuses reported by `screenshots frame --input-dir`.
const (
	ScreenshotFrameBatchStatusFramed  = "framed"
	ScreenshotFrameBatchStatusSkipped = "skipped"
	ScreenshotFrameBatchStatusFailed  = "failed"
)

// ScreenshotFrameBatchResult is the receipt for `screenshots frame --input-dir`.
type ScreenshotFrameBatchResult struct {
	InputDir   string                     `json:"inputDir"`
	OutputDir  string                     `json:"outputDir"`
	Device     string                     `json:"device"`
	FrameColor string                     `json:"frameColor,omitempty"`
	Resume     bool                       `json:"resume"`
	Total      int                        `json:"total"`
	Framed     int                        `json:"framed"`
	Skipped    int                        `json:"skipped"`
	Failed     int                        `json:"failed"`
	Files      []ScreenshotFrameBatchFile `json:"files"`
}

func screenshotFrameBatchSummaryRows(result *ScreenshotFrameBatchResult) ([]string, [][]string) {
	return []string{"Input Dir", "Output Dir", "Device", "Frame Color", "Resume", "Total", "Framed", "Skipped", "Failed"},
		[][]string{{
			compactWhitespace(result.InputDir),
			compactWhitespace(result.OutputDir),
			result.Device,
			result.FrameColor,
			fmt.Sprint(result.Resume),
			fmt.Sprint(result.Total),
			fmt.Sprint(result.Framed),
			fmt.Sprint(result.Skipped),
			fmt.Sprint(result.Failed),
		}}
}

func screenshotFrameBatchFileRows(files []ScreenshotFrameBatchFile) ([]string, [][]string) {
	rows := make([][]string, 0, len(files))
	for _, file := range files {
		size := ""
		if file.Width > 0 && file.Height > 0 {
			size = fmt.Sprintf("%dx%d", file.Width, file.Height)
		}
		rows = append(rows, []string{
			compactWhitespace(file.Input),
			file.Status,
			compactWhitespace(file.Path),
			file.DisplayType,
			size,
			compactWhitespace(file.Error),
		})
	}
	return []string{"Input", "Status", "Path", "Display Type", "Size", "Error"}, rows
}

func renderScreenshotFrameBatchResult(result *ScreenshotFrameBatchResult, render func([]string, [][]string)) error {
	headers, rows := screenshotFrameBatchSummaryRows(result)
	render(headers, rows)
	if len(result.Files) > 0 {
		headers, rows = screenshotFrameBatchFileRows(result.Files)
		render(headers, rows)
	}
	return nil
}
