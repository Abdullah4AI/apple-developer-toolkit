package validation

import (
	"strings"
	"testing"
)

func TestReviewNotesLengthCountsCharactersNotBytes(t *testing.T) {
	tests := []struct {
		name  string
		notes string
		want  int
	}{
		{name: "empty", notes: "", want: 0},
		{name: "ascii", notes: "Guest flow", want: 10},
		{name: "two-byte characters", notes: strings.Repeat("é", LimitReviewNotes), want: LimitReviewNotes},
		{name: "three-byte characters", notes: "No login — tap Start", want: 20},
		{name: "four-byte character", notes: "🙂", want: 1},
		{name: "lf line break", notes: "a\nb", want: 3},
		{name: "crlf line break counts once", notes: "a\r\nb", want: 3},
		{name: "lone cr", notes: "a\rb", want: 3},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ReviewNotesLength(test.notes); got != test.want {
				t.Fatalf("ReviewNotesLength(%q) = %d, want %d", test.notes, got, test.want)
			}
		})
	}
}

func TestReviewNotesLimitBoundary(t *testing.T) {
	if LimitReviewNotes != 4000 {
		t.Fatalf("LimitReviewNotes = %d, want 4000", LimitReviewNotes)
	}
	atLimit := strings.Repeat("a", LimitReviewNotes)
	if got := ReviewNotesLength(atLimit); got != LimitReviewNotes {
		t.Fatalf("length at limit = %d, want %d", got, LimitReviewNotes)
	}
	overLimit := atLimit + "a"
	if got := ReviewNotesLength(overLimit); got != LimitReviewNotes+1 {
		t.Fatalf("length over limit = %d, want %d", got, LimitReviewNotes+1)
	}
	crlfAtLimit := strings.Repeat("a\r\n", LimitReviewNotes/2)
	if got := ReviewNotesLength(crlfAtLimit); got != LimitReviewNotes {
		t.Fatalf("CRLF text length = %d, want %d", got, LimitReviewNotes)
	}
}

func TestNANPNationalDigitCount(t *testing.T) {
	tests := []struct {
		name      string
		phone     string
		wantCount int
		wantOK    bool
	}{
		{name: "compact ten digits", phone: "+14085550100", wantCount: 10, wantOK: true},
		{name: "spaced ten digits", phone: "+1 408 555 0100", wantCount: 10, wantOK: true},
		{name: "punctuated ten digits", phone: "+1 (408) 555-0100", wantCount: 10, wantOK: true},
		{name: "dotted ten digits", phone: "+1.408.555.0100", wantCount: 10, wantOK: true},
		{name: "dashed after country code", phone: "+1-408-555-0100", wantCount: 10, wantOK: true},
		{name: "space after plus", phone: "+ 1 408 555 0100", wantCount: 10, wantOK: true},
		{name: "seven digits", phone: "+1 555 0100", wantCount: 7, wantOK: true},
		{name: "nine digits", phone: "+1 408 555 010", wantCount: 9, wantOK: true},
		{name: "eleven digits", phone: "+1 972 097 57881", wantCount: 11, wantOK: true},
		{name: "eleven digits compact", phone: "+197209757881", wantCount: 11, wantOK: true},
		{name: "country code only", phone: "+1", wantCount: 0, wantOK: true},
		{name: "other country code", phone: "+44 844 209 0611", wantOK: false},
		{name: "other country code with short number", phone: "+503 317 8562", wantOK: false},
		{name: "no plus sign", phone: "4085550100", wantOK: false},
		{name: "no plus sign with leading one", phone: "1 408 555 0100", wantOK: false},
		{name: "extension text is left to App Store Connect", phone: "+1 408 555 0100 ext 12", wantOK: false},
		{name: "extension marker is left to App Store Connect", phone: "+1 408 555 0100 x12", wantOK: false},
		{name: "letters are left to App Store Connect", phone: "+1 800 FLOWERS", wantOK: false},
		{name: "second plus sign", phone: "+1 +408 555 0100", wantOK: false},
		{name: "empty", phone: "", wantOK: false},
		{name: "plus only", phone: "+", wantOK: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			count, ok := NANPNationalDigitCount(test.phone)
			if ok != test.wantOK {
				t.Fatalf("NANPNationalDigitCount(%q) ok = %v, want %v", test.phone, ok, test.wantOK)
			}
			if ok && count != test.wantCount {
				t.Fatalf("NANPNationalDigitCount(%q) = %d, want %d", test.phone, count, test.wantCount)
			}
		})
	}
}
