package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

func TestRunProfilesLocalInstallInputFailuresAreClassified(t *testing.T) {
	dir := t.TempDir()
	// --install-dir is required off macOS; the failures under test happen
	// before anything is written there.
	installDir := filepath.Join(dir, "installed")
	if err := os.Mkdir(installDir, 0o700); err != nil {
		t.Fatalf("Mkdir(%q) error: %v", installDir, err)
	}
	missingProfile := filepath.Join(dir, "missing.mobileprovision")
	junkProfile := writeLocalInputFixture(t, filepath.Join(dir, "junk.mobileprovision"), "junk\n")
	_, openErr := os.Open(missingProfile)
	if openErr == nil {
		t.Fatalf("os.Open(%q) unexpectedly succeeded", missingProfile)
	}

	tests := []localInputOutcomeCase{
		{
			name:             "missing path",
			args:             []string{"profiles", "local", "install", "--path", missingProfile, "--install-dir", installDir},
			wantStderrPrefix: "Error: profiles local install: open input: " + openErr.Error() + "\n",
			wantDiagnosis:    shared.DiagnosticFileNotFound,
			wantParameter:    "--path",
		},
		{
			name:             "undecodable path",
			args:             []string{"profiles", "local", "install", "--path", junkProfile, "--install-dir", installDir},
			wantStderrPrefix: "Error: profiles local install: decode embedded plist: ",
			wantDiagnosis:    shared.DiagnosticFileInvalidFormat,
			wantParameter:    "--path",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runLocalInputOutcomeCase(t, test)
		})
	}
}
