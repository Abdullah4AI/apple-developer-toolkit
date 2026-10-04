package shared

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

// PreflightReviewScreenshot checks the bytes of an in-app purchase or
// subscription App Review screenshot that are about to be uploaded. A format
// problem App Store Connect would reject at delivery is returned as an error
// before any request. Documented but unconfirmed constraints (size and alpha)
// and a payload that does not fully decode are printed to stderr as warnings.
func PreflightReviewScreenshot(path string, file io.ReaderAt, size int64) error {
	warnings, err := asc.CheckReviewScreenshotImage(path, io.NewSectionReader(file, 0, size))
	if err != nil {
		return err
	}
	if warning := asc.ReviewScreenshotDecodeWarning(path, file, size); warning != "" {
		warnings = append(warnings, warning)
	}
	for _, warning := range warnings {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
	}
	return nil
}

// ReviewScreenshotUsageError reports a review screenshot rejected before any
// request. It prints message as one "Error:" line and returns a usage error
// (exit 2) that does not wrap flag.ErrHelp, so the command's full usage page
// does not follow and bury the one actionable line. parameter names the flag
// that supplied the screenshot for diagnostics. Commands under a re-parented
// tree pass message through RewriteUsageMessage first.
func ReviewScreenshotUsageError(parameter, message string) error {
	message = strings.TrimSpace(SanitizeTerminal(message))
	fmt.Fprintf(os.Stderr, "Error: %s\n", message)
	return WithDiagnostic(NewReportedUsageError(UsageErrorInvalidValue, message), DiagnosticInvalidInput, parameter)
}
