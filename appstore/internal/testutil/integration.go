// Package testutil provides shared helpers for integration and unit tests.
package testutil

import (
	"os"
	"strings"
	"testing"
)

// SkipUnlessIntegration skips the current test unless the ASC_INTEGRATION_TEST
// environment variable is set to "true". Use this in integration tests that
// hit the real App Store Connect API so they are skipped during normal `go test`
// runs and only execute when explicitly opted in.
//
// Example:
//
//	func TestLiveEndpoint(t *testing.T) {
//	    testutil.SkipUnlessIntegration(t)
//	    // ... test against real API
//	}
func SkipUnlessIntegration(t *testing.T) {
	t.Helper()
	if !isIntegrationEnabled() {
		t.Skip("set ASC_INTEGRATION_TEST=true to run integration tests")
	}
}

// isIntegrationEnabled returns true when the ASC_INTEGRATION_TEST env var
// is explicitly set to a truthy value.
func isIntegrationEnabled() bool {
	value := strings.TrimSpace(os.Getenv("ASC_INTEGRATION_TEST"))
	switch strings.ToLower(value) {
	case "true", "1", "yes":
		return true
	default:
		return false
	}
}
