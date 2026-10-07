package cmd

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/telemetry"
)

func TestRunBuildsUploadMissingIPAIsFileNotFoundValidation(t *testing.T) {
	resetReportFlags(t)
	t.Setenv("ASC_APP_ID", "")
	missingPath := filepath.Join(t.TempDir(), "missing.ipa")

	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		t.Fatal("unexpected client creation for missing IPA")
		return nil, nil
	}))

	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
	var gotContext telemetry.EventContext
	emitTelemetry = func(_ string, _ string, _ time.Duration, _ int, eventContext telemetry.EventContext) {
		gotContext = eventContext
	}

	_, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"builds", "upload", "--app", "123456789", "--ipa", missingPath}, "4.0.0"); code != ExitError {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitError)
		}
	})

	if want := fmt.Sprintf("builds upload: --ipa file not found: %q", missingPath); !strings.Contains(stderr, want) {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	if gotContext.OutcomeKind != telemetry.OutcomeExpectedNegative ||
		gotContext.FailureStage != telemetry.FailureStageValidation ||
		gotContext.DiagnosticCode != string(shared.DiagnosticFileNotFound) ||
		gotContext.FailureParameter != "--ipa" {
		t.Fatalf("telemetry context = %+v, want expected_negative validation file_not_found for --ipa", gotContext)
	}
}
