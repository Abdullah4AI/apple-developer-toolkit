package shared

import (
	"testing"
	"time"
)

func TestPricingDateUsesUSPacificCalendarDay(t *testing.T) {
	tests := []struct {
		now  string
		want string
	}{
		// Between UTC midnight and US Pacific midnight, App Store Connect is
		// still on the previous day.
		{now: "2026-10-01T00:30:00Z", want: "2026-09-30"},
		{now: "2026-10-01T06:59:59Z", want: "2026-09-30"},
		{now: "2026-10-01T07:00:00Z", want: "2026-10-01"},
		// Standard time: Pacific midnight is 08:00 UTC.
		{now: "2026-12-01T07:59:59Z", want: "2026-11-30"},
		{now: "2026-12-01T08:00:00Z", want: "2026-12-01"},
		// The input's own location must not matter.
		{now: "2026-10-01T09:30:00+09:00", want: "2026-09-30"},
	}

	for _, test := range tests {
		t.Run(test.now, func(t *testing.T) {
			now, err := time.Parse(time.RFC3339, test.now)
			if err != nil {
				t.Fatalf("parse now: %v", err)
			}
			got := PricingDate(now)
			if got.Location() != time.UTC || got.Hour() != 0 || got.Minute() != 0 || got.Second() != 0 || got.Nanosecond() != 0 {
				t.Fatalf("PricingDate(%s) = %s, want midnight UTC", test.now, got)
			}
			if formatted := got.Format("2006-01-02"); formatted != test.want {
				t.Fatalf("PricingDate(%s) = %s, want %s", test.now, formatted, test.want)
			}
		})
	}
}

func TestPriceActiveOnTreatsEndDateAsExclusive(t *testing.T) {
	date := func(value string) *time.Time {
		parsed, err := time.Parse("2006-01-02", value)
		if err != nil {
			t.Fatalf("parse %q: %v", value, err)
		}
		return &parsed
	}
	today := *date("2026-09-30")

	tests := []struct {
		name  string
		start *time.Time
		end   *time.Time
		want  bool
	}{
		{name: "unbounded", want: true},
		{name: "started earlier", start: date("2026-01-01"), want: true},
		{name: "starts today", start: date("2026-09-30"), want: true},
		{name: "starts tomorrow", start: date("2026-10-01"), want: false},
		{name: "ends tomorrow", end: date("2026-10-01"), want: true},
		{name: "ends today", end: date("2026-09-30"), want: false},
		{name: "ended yesterday", end: date("2026-09-29"), want: false},
		{name: "starts and ends today", start: date("2026-09-30"), end: date("2026-09-30"), want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := PriceActiveOn(test.start, test.end, today); got != test.want {
				t.Fatalf("PriceActiveOn() = %t, want %t", got, test.want)
			}
		})
	}
}
