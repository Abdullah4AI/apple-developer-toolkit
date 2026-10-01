package pricing

import (
	"testing"
	"time"
)

func TestHasCurrentOrScheduledPrice(t *testing.T) {
	now := time.Date(2026, time.September, 30, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		entries []appPriceEntry
		want    bool
	}{
		{name: "no prices", want: false},
		{name: "current free price", entries: []appPriceEntry{newAppPriceEntry("USA", "free", "2026-01-01", "", true)}, want: true},
		{name: "price starting later", entries: []appPriceEntry{newAppPriceEntry("USA", "paid", "2026-10-15", "", true)}, want: true},
		{name: "price ending today", entries: []appPriceEntry{newAppPriceEntry("USA", "paid", "2026-01-01", "2026-09-30", true)}, want: false},
		{name: "price ending tomorrow", entries: []appPriceEntry{newAppPriceEntry("USA", "paid", "2026-01-01", "2026-10-01", true)}, want: true},
		{name: "ended price", entries: []appPriceEntry{newAppPriceEntry("USA", "paid", "2026-01-01", "2026-09-29", true)}, want: false},
		{name: "other territory only", entries: []appPriceEntry{newAppPriceEntry("CAN", "paid", "2026-01-01", "", true)}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := hasCurrentOrScheduledPrice(test.entries, "usa", now); got != test.want {
				t.Fatalf("hasCurrentOrScheduledPrice() = %t, want %t", got, test.want)
			}
		})
	}
}
