package cmd

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/telemetry"
)

func TestRunMissingPrivateKeyFileIsAuthErrorWithDiagnostic(t *testing.T) {
	resetReportFlags(t)
	resetSelectedProfile(t)
	tempDir := t.TempDir()
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "config.json"))
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "KEY123")
	t.Setenv("ASC_ISSUER_ID", "ISSUER123")
	t.Setenv("ASC_PRIVATE_KEY_PATH", filepath.Join(tempDir, "AuthKey_KEY123.p8"))
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")

	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
	var gotExitCode int
	var gotContext telemetry.EventContext
	emitTelemetry = func(_ string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
		gotExitCode = exitCode
		gotContext = eventContext
	}

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"apps", "list"}, "4.0.0"); code != ExitAuth {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitAuth)
		}
	})

	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "invalid private key") || !strings.Contains(stderr, "asc auth login") {
		t.Fatalf("stderr = %q, want the private key error and an auth login hint", stderr)
	}
	if gotExitCode != ExitAuth ||
		gotContext.OutcomeKind != telemetry.OutcomeAuthError ||
		gotContext.DiagnosticCode != string(shared.DiagnosticFileNotFound) ||
		gotContext.FailureParameter != "--private-key" {
		t.Fatalf("unexpected telemetry: exit=%d context=%+v", gotExitCode, gotContext)
	}
}
