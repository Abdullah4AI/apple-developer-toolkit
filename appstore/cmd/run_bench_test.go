package cmd

import (
	"os"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

func BenchmarkRunVersionOnlyFlag(b *testing.B) {
	shared.SetReportFormat("")
	shared.SetReportFile("")

	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		b.Fatalf("OpenFile(%q) error: %v", os.DevNull, err)
	}
	defer func() {
		_ = devNull.Close()
	}()

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	os.Stdout = devNull
	os.Stderr = devNull
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if code := Run([]string{"--version"}, "9.9.9"); code != ExitSuccess {
			b.Fatalf("Run() exit code = %d, want %d", code, ExitSuccess)
		}
	}
}
