package cmdtest

import (
	"strings"
	"testing"
)

func TestPricingSchedulePriceLeafHelpMentionsResolved(t *testing.T) {
	for _, path := range [][]string{
		{"pricing", "schedule", "manual-prices"},
		{"pricing", "schedule", "automatic-prices"},
	} {
		usage := usageForCommand(t, path...)
		if !strings.Contains(usage, "--resolved") {
			t.Fatalf("expected usage for %q to mention --resolved, got %q", strings.Join(path, " "), usage)
		}
	}
}
