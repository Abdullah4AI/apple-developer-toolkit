package ads

import (
	"os"
	"testing"
)

// TestMain disables retry backoff by default; retry tests opt back in with t.Setenv.
func TestMain(m *testing.M) {
	_ = os.Setenv("ASC_MAX_RETRIES", "0")
	os.Exit(m.Run())
}
