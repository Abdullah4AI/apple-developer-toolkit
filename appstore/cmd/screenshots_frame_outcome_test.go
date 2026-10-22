package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

func TestRunScreenshotsFrameInputFailuresAreClassified(t *testing.T) {
	dir := t.TempDir()
	missingScreenshot := filepath.Join(dir, "missing.png")
	screenshot := writeLocalInputFixture(t, filepath.Join(dir, "screen.png"), "junk\n")
	missingConfig := filepath.Join(dir, "missing.yaml")
	_, lstatErr := os.Lstat(missingScreenshot)
	if lstatErr == nil {
		t.Fatalf("os.Lstat(%q) unexpectedly succeeded", missingScreenshot)
	}
	_, statErr := os.Stat(missingConfig)
	if statErr == nil {
		t.Fatalf("os.Stat(%q) unexpectedly succeeded", missingConfig)
	}

	tests := []localInputOutcomeCase{
		{
			name:             "missing input",
			args:             []string{"screenshots", "frame", "--input", missingScreenshot},
			wantStderrPrefix: "Error: screenshots frame: read input screenshot: " + lstatErr.Error() + "\n",
			wantDiagnosis:    shared.DiagnosticFileNotFound,
			wantParameter:    "--input",
		},
		{
			// --resume opens the input through the snapshot path instead.
			name:             "resume missing input",
			args:             []string{"screenshots", "frame", "--input", missingScreenshot, "--resume"},
			isolatedWorkDir:  true,
			wantStderrPrefix: "Error: screenshots frame: snapshot input: open frame input: ",
			wantDiagnosis:    shared.DiagnosticFileNotFound,
			wantParameter:    "--input",
		},
		{
			name:             "missing config",
			args:             []string{"screenshots", "frame", "--config", missingConfig},
			wantStderrPrefix: "Error: screenshots frame: read config file: " + statErr.Error() + "\n",
			wantDiagnosis:    shared.DiagnosticFileNotFound,
			wantParameter:    "--config",
		},
		{
			name:             "without koubou",
			args:             []string{"screenshots", "frame", "--input", screenshot, "--output-dir", filepath.Join(dir, "framed")},
			env:              map[string]string{"PATH": t.TempDir()},
			wantStderrPrefix: "Error: screenshots frame: kou binary not found; install pinned Koubou ",
			wantDiagnosis:    shared.DiagnosticDependencyFailed,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runLocalInputOutcomeCase(t, test)
		})
	}
}
