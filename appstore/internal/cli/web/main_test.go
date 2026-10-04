package web

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain isolates the web session cache so package tests that exercise the
// real store resolvers never read or write the developer's own cached
// sessions or open the native keychain.
func TestMain(m *testing.M) {
	tempDir, err := os.MkdirTemp("", "asc-web-cli-test-*")
	if err != nil {
		panic(err)
	}
	_ = os.Setenv("HOME", tempDir)
	// A temporary HOME alone does not isolate stored config: the upward
	// .asc/config.json search starts at the package directory, so a checkout
	// inside the developer's home reaches their real ~/.asc/config.json.
	// Pin the config path to a file that never exists.
	_ = os.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "config.json"))
	_ = os.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	_ = os.Setenv("ASC_WEB_SESSION_CACHE_DIR", tempDir)
	_ = os.Setenv("ASC_WEB_SESSION_CACHE_BACKEND", "file")
	// The Apple ID environment fallback must not leak in from the developer's
	// own shell: a test that does not set it expects no Apple ID at all.
	_ = os.Unsetenv(webAppleIDEnv)

	code := m.Run()

	_ = os.RemoveAll(tempDir)
	os.Exit(code)
}
