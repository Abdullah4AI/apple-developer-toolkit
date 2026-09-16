package validate

import (
	"strings"
	"testing"
)

func TestValidateHelpExplainsDefaultVersionSelection(t *testing.T) {
	help := ValidateCommand().LongHelp
	for _, want := range []string{
		"When --version and --version-id are omitted",
		"newest active editable App Store version",
		"DEVELOPER_REMOVED_FROM_SALE version",
		"falls back to the newest live version",
		"does not mean the live version is ready for submission",
		"Pass --platform",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("help missing %q:\n%s", want, help)
		}
	}
}
