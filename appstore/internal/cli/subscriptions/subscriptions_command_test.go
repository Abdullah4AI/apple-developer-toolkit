package subscriptions

import (
	"strings"
	"testing"
)

func TestSubscriptionsPricesListCommand_HasResolvedFlag(t *testing.T) {
	cmd := SubscriptionsPricesListCommand()

	if cmd.FlagSet.Lookup("resolved") == nil {
		t.Fatal("expected --resolved flag")
	}
	if !strings.Contains(cmd.LongHelp, "--resolved") {
		t.Fatalf("expected long help to mention --resolved, got %q", cmd.LongHelp)
	}
}
