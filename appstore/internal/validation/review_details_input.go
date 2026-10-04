package validation

import (
	"strings"
	"unicode/utf8"
)

// NANPNationalNumberDigits is the length of every North American Numbering
// Plan number after the +1 country code: a 3-digit area code and a 7-digit
// subscriber number.
const NANPNationalNumberDigits = 10

// ReviewNotesLength counts App Review notes in Unicode characters, not bytes,
// and counts a CRLF line break once. Both choices give the lower of the
// plausible counts, so a local check built on it never rejects notes that App
// Store Connect would accept under another counting rule; App Store Connect
// stays the authority for notes the CLI lets through.
func ReviewNotesLength(notes string) int {
	return utf8.RuneCountInString(notes) - strings.Count(notes, "\r\n")
}

// NANPNationalDigitCount returns the number of digits after the +1 country
// code when phone is written as "+1" followed only by digits and the common
// separators space, hyphen, dot, and parentheses. ok is false for any other
// value, including other country codes, numbers without a plus sign, and
// numbers with extension text, so callers leave those to App Store Connect.
func NANPNationalDigitCount(phone string) (count int, ok bool) {
	rest, found := strings.CutPrefix(strings.TrimSpace(phone), "+")
	if !found {
		return 0, false
	}
	var digits strings.Builder
	for _, r := range rest {
		switch {
		case r >= '0' && r <= '9':
			digits.WriteRune(r)
		case r == ' ' || r == '-' || r == '.' || r == '(' || r == ')':
		default:
			return 0, false
		}
	}
	national, found := strings.CutPrefix(digits.String(), "1")
	if !found {
		return 0, false
	}
	return len(national), true
}
