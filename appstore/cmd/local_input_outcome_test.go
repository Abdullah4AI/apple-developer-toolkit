package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/telemetry"
)

// localInputOutcomeCase is a failure caused only by the caller's local input
// files. It must keep its message and exit 1 while telemetry reports an
// expected negative with the diagnostic instead of an internal error.
type localInputOutcomeCase struct {
	name             string
	args             []string
	env              map[string]string
	isolatedWorkDir  bool
	wantStderrPrefix string
	wantReceipt      bool
	wantDiagnosis    shared.DiagnosticCode
	wantParameter    string
}

func runLocalInputOutcomeCase(t *testing.T, test localInputOutcomeCase) {
	t.Helper()
	resetReportFlags(t)
	tempDir := t.TempDir()
	t.Setenv("HOME", filepath.Join(tempDir, "home"))
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "config.json"))
	for key, value := range test.env {
		t.Setenv(key, value)
	}
	if test.isolatedWorkDir {
		t.Chdir(tempDir)
	}

	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
	var gotContext telemetry.EventContext
	emitTelemetry = func(_ string, _ string, _ time.Duration, _ int, eventContext telemetry.EventContext) {
		gotContext = eventContext
	}

	var code int
	stdout, stderr := captureCommandOutput(t, func() {
		code = Run(test.args, "5.15.0")
	})

	if code != ExitError {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, ExitError, stderr)
	}
	if !strings.HasPrefix(stderr, test.wantStderrPrefix) {
		t.Fatalf("stderr = %q, want prefix %q", stderr, test.wantStderrPrefix)
	}
	if strings.Count(stderr, "Error:") != 1 {
		t.Fatalf("stderr = %q, want exactly one error line", stderr)
	}
	if test.wantReceipt {
		var receipt struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
			t.Fatalf("stdout = %q, want JSON receipt: %v", stdout, err)
		}
		if receipt.Status != "unreadable" {
			t.Fatalf("receipt status = %q, want unreadable", receipt.Status)
		}
	} else if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if gotContext.OutcomeKind != telemetry.OutcomeExpectedNegative ||
		gotContext.DiagnosticCode != string(test.wantDiagnosis) ||
		gotContext.FailureParameter != test.wantParameter {
		t.Fatalf("telemetry context = %+v, want outcome %q, diagnostic %q, parameter %q",
			gotContext, telemetry.OutcomeExpectedNegative, test.wantDiagnosis, test.wantParameter)
	}
}

// writeLocalInputFixture writes a private (0600) fixture file.
func writeLocalInputFixture(t *testing.T, path string, data string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error: %v", path, err)
	}
	return path
}
