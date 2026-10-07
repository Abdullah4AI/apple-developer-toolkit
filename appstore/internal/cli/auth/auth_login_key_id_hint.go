package auth

import (
	"path/filepath"
	"regexp"
	"strings"
)

var authKeyFileNamePattern = regexp.MustCompile(`^AuthKey_([A-Z0-9]{10})\.p8$`)

func keyIDHintFromKeyPath(keyPath string) string {
	match := authKeyFileNamePattern.FindStringSubmatch(filepath.Base(strings.TrimSpace(keyPath)))
	if match == nil {
		return ""
	}
	return match[1]
}
