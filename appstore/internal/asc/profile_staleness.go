package asc

import (
	"strings"
	"time"
)

// ProfileExpirationPassed reports whether a profile expirationDate value is
// before now. A date-only value stays valid through the end of that UTC day.
// Empty or unparseable values are treated as not expired so a malformed date
// never marks a profile for deletion.
func ProfileExpirationPassed(value string, now time.Time) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.Before(now)
	}
	day, err := time.Parse("2006-01-02", value)
	if err != nil {
		return false
	}
	return !now.Before(day.AddDate(0, 0, 1))
}

// ProfileIsStale reports whether a profile is INVALID or past its expiration
// date.
func ProfileIsStale(attrs ProfileAttributes, now time.Time) bool {
	if attrs.ProfileState == ProfileStateInvalid {
		return true
	}
	return ProfileExpirationPassed(attrs.ExpirationDate, now)
}
