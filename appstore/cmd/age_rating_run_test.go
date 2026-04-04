package cmd

import (
	"strings"
	"testing"
)

func TestRun_AgeRatingSetInvalidAllNoneReturnsUsage(t *testing.T) {
	resetReportFlags(t)

	stdout, stderr, exitCode := runHelpSubprocess(t, t.TempDir(),
		"age-rating", "edit",
		"--id", "age-1",
		"--all-none=maybe",
	)
	if exitCode != ExitUsage {
		t.Fatalf("expected exit code %d, got %d", ExitUsage, exitCode)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "invalid boolean value") {
		t.Fatalf("expected invalid boolean value error, got %q", stderr)
	}
}

func TestRun_AgeRatingSetAllNoneFalseReturnsUsage(t *testing.T) {
	resetReportFlags(t)

	stdout, stderr, exitCode := runHelpSubprocess(t, t.TempDir(),
		"age-rating", "edit",
		"--id", "age-1",
		"--all-none", "false",
	)
	if exitCode != ExitUsage {
		t.Fatalf("expected exit code %d, got %d", ExitUsage, exitCode)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "unexpected argument(s): false") {
		t.Fatalf("expected unexpected argument error, got %q", stderr)
	}
}
