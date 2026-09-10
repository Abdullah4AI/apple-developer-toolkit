package validate

import (
	"strings"
	"testing"
)

func TestValidateHelpDocumentsPlaceholderWarningScope(t *testing.T) {
	cmd := ValidateCommand()
	for _, want := range []string{
		"Placeholder copy in localized listing fields",
		"warning; --strict to block",
		"localized TODO copy without marker punctuation",
		"shorter Lorem Ipsum product wording",
	} {
		if !strings.Contains(cmd.LongHelp, want) {
			t.Fatalf("LongHelp missing %q:\n%s", want, cmd.LongHelp)
		}
	}
}

func TestValidateCommandAcceptsOptInURLChecks(t *testing.T) {
	cmd := ValidateCommand()
	if err := cmd.Parse([]string{"--app", "app-1", "--version-id", "version-1", "--check-urls"}); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	flagDef := cmd.FlagSet.Lookup("check-urls")
	if flagDef == nil {
		t.Fatal("--check-urls flag is not registered")
	}
}

func TestValidateURLChecksAreTopLevelOnly(t *testing.T) {
	cmd := ValidateCommand()
	err := cmd.ParseAndRun(t.Context(), []string{"--check-urls", "testflight", "--app", "app-1", "--build-id", "build-1"})
	if err == nil || !strings.Contains(err.Error(), "--check-urls is only valid for asc validate") {
		t.Fatalf("error = %v, want top-level-only URL-check diagnostic", err)
	}
}
