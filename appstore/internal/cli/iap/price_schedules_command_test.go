package iap

import (
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func TestIAPPriceSchedulePriceCommands_HaveResolvedFlag(t *testing.T) {
	for _, tc := range []struct {
		name string
		cmd  func() *ffcli.Command
	}{
		{name: "manual", cmd: IAPPriceSchedulesManualPricesCommand},
		{name: "automatic", cmd: IAPPriceSchedulesAutomaticPricesCommand},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := tc.cmd()
			if cmd.FlagSet.Lookup("resolved") == nil {
				t.Fatal("expected --resolved flag")
			}
			if !strings.Contains(cmd.LongHelp, "--resolved") {
				t.Fatalf("expected long help to mention --resolved, got %q", cmd.LongHelp)
			}
		})
	}
}
