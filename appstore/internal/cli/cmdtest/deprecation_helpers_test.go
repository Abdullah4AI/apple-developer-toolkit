package cmdtest

import (
	"strings"
	"testing"
)

const (
	feedbackRootDeprecationWarning = "Warning: `asc feedback` is deprecated. Use `asc testflight feedback list`."
	crashesRootDeprecationWarning  = "Warning: `asc crashes` is deprecated. Use `asc testflight crashes list`."
)

func requireStderrContainsWarning(t *testing.T, stderr, warning string) {
	t.Helper()
	if !strings.Contains(stderr, warning) {
		t.Fatalf("expected stderr to contain warning %q, got %q", warning, stderr)
	}
}
