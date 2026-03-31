package shared

import (
	"net/url"
	"strings"
)

// MergeNextURLQuery reapplies required query parameters to a validated next URL.
func MergeNextURLQuery(next string, additions url.Values) (string, error) {
	next = strings.TrimSpace(next)
	if next == "" {
		return "", nil
	}
	if err := validateNextURL(next); err != nil {
		return "", err
	}

	parsed, err := url.Parse(next)
	if err != nil {
		return "", err
	}

	query := parsed.Query()
	for key, values := range additions {
		query.Del(key)
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			query.Add(key, value)
		}
	}

	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}
