package auth

import (
	"encoding/base64"
	"encoding/pem"
	"regexp"
	"strings"
)

var flattenedPrivateKeyPEM = regexp.MustCompile(`^-----BEGIN ([A-Z0-9 ]*PRIVATE KEY)-----([A-Za-z0-9+/=\s]*)-----END ([A-Z0-9 ]*PRIVATE KEY)-----$`)

// NormalizePrivateKeyPEM restores the line structure that CI secret stores and
// shells strip from an inline key: escaped newlines, a key flattened onto one
// line, or a body whose BEGIN and END lines were dropped. A value it cannot
// repair is returned with only its escaped newlines replaced, so that key
// parsing reports it.
func NormalizePrivateKeyPEM(value string) string {
	normalized := strings.ReplaceAll(value, `\n`, "\n")
	if block, _ := pem.Decode([]byte(normalized)); block != nil {
		return normalized
	}

	label, body := "PRIVATE KEY", strings.TrimSpace(normalized)
	if match := flattenedPrivateKeyPEM.FindStringSubmatch(body); match != nil {
		if match[1] != match[3] {
			return normalized
		}
		label, body = match[1], match[2]
	}
	der, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(body), ""))
	if err != nil || len(der) == 0 {
		return normalized
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: label, Bytes: der}))
}
