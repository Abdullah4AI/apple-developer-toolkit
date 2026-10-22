package cmd

import (
	"path/filepath"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/rootfs"
)

func TestRunArtifactInfoInputFailuresAreClassified(t *testing.T) {
	dir := t.TempDir()
	missingIPA := filepath.Join(dir, "missing.ipa")
	junkIPA := writeLocalInputFixture(t, filepath.Join(dir, "junk.ipa"), "junk\n")
	file, openErr := rootfs.OpenFile(missingIPA)
	if openErr == nil {
		_ = file.Close()
		t.Fatalf("rootfs.OpenFile(%q) unexpectedly succeeded", missingIPA)
	}

	tests := []localInputOutcomeCase{
		{
			name:             "missing path",
			args:             []string{"ipa-info", "--path", missingIPA},
			wantStderrPrefix: "Error: ipa-info: " + openErr.Error() + "\n",
			wantReceipt:      true,
			wantDiagnosis:    shared.DiagnosticFileNotFound,
			wantParameter:    "--path",
		},
		{
			name:             "undecodable path",
			args:             []string{"ipa-info", "--path", junkIPA},
			wantStderrPrefix: "Error: ipa-info: open IPA: truncated ZIP directory\n",
			wantReceipt:      true,
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
