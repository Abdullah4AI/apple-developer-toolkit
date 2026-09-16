package cmdtest

import (
	"os"
	"path/filepath"
	"testing"

	webcli "github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/web"
)

var testConfigPath string

func TestMain(m *testing.M) {
	tempDir, err := os.MkdirTemp("", "asc-cmdtest-*")
	if err != nil {
		panic(err)
	}
	testConfigPath = filepath.Join(tempDir, "config.json")
	testStdin, err := os.Open(os.DevNull)
	if err != nil {
		panic(err)
	}
	originalStdin := os.Stdin
	os.Stdin = testStdin
	restoreControllingTTY := webcli.DisableControllingTTYForTesting()

	_ = os.Setenv("ASC_CONFIG_PATH", testConfigPath)
	_ = os.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	_ = os.Setenv("ASC_MAX_RETRIES", "0")
	_ = os.Setenv("ASC_TELEMETRY_DISABLED", "1")
	_ = os.Setenv("HOME", tempDir)
	// The Apple ID environment fallback for "asc web" commands is a
	// process-wide input: a developer or CI host that exports it would
	// otherwise select an account for every session-resolving command under
	// test. Tests that want it set it themselves.
	_ = os.Unsetenv(webAppleIDEnvNameForTest())

	code := m.Run()

	restoreControllingTTY()
	os.Stdin = originalStdin
	_ = testStdin.Close()
	_ = os.RemoveAll(tempDir)
	os.Exit(code)
}
