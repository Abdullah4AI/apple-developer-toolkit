package versions

import (
	"strings"
	"testing"
)

// Apple reports some live versions only through appVersionState
// READY_FOR_DISTRIBUTION, so the help must not send users to READY_FOR_SALE
// alone to find the live version.
func TestVersionsListHelpFindsLiveVersionsUnderBothStateSpellings(t *testing.T) {
	help := VersionsListCommand().LongHelp
	for _, want := range []string{
		"appVersionState READY_FOR_DISTRIBUTION",
		"appStoreState\nREADY_FOR_SALE",
		`asc versions list --app "123456789" --state READY_FOR_DISTRIBUTION --latest`,
		`asc versions list --app "123456789" --state READY_FOR_SALE --paginate`,
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("expected versions list help to contain %q, got:\n%s", want, help)
		}
	}
	// READY_FOR_SALE can still be reported by replaced versions, so --latest
	// on that state could keep a stale version and drop the live one.
	for _, unwanted := range []string{
		"combine it with --state READY_FOR_SALE",
		"--state READY_FOR_SALE --latest",
	} {
		if strings.Contains(help, unwanted) {
			t.Fatalf("help must not recommend %q:\n%s", unwanted, help)
		}
	}
}
