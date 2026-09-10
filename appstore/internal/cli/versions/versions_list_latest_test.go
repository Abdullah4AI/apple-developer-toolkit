package versions

import (
	"testing"
)

func TestVersionsListLatestFlagIsRegistered(t *testing.T) {
	t.Parallel()

	flag := VersionsListCommand().FlagSet.Lookup("latest")
	if flag == nil {
		t.Fatal("expected --latest flag")
	}
}
