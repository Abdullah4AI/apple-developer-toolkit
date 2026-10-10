//go:build !integration

package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/99designs/keyring"
)

var testConfigPath string

func TestMain(m *testing.M) {
	tempDir, err := os.MkdirTemp("", "asc-auth-test-*")
	if err != nil {
		panic(err)
	}
	testConfigPath = filepath.Join(tempDir, "config.json")

	_ = os.Setenv("ASC_CONFIG_PATH", testConfigPath)
	_ = os.Setenv("HOME", tempDir)

	previousKeyringOpener := keyringOpener
	previousLegacyKeyringOpener := legacyKeyringOpener
	previousLegacyMarkerPath := legacyKeychainEmptyMarkerPath
	legacyKeychainEmptyMarkerPath = func() (string, error) { return "", nil }

	kr := keyring.NewArrayKeyring([]keyring.Item{})
	legacyKr := keyring.NewArrayKeyring([]keyring.Item{})

	keyringOpener = func() (keyring.Keyring, error) {
		return kr, nil
	}
	legacyKeyringOpener = func() (keyring.Keyring, error) {
		return legacyKr, nil
	}

	code := m.Run()

	keyringOpener = previousKeyringOpener
	legacyKeyringOpener = previousLegacyKeyringOpener
	legacyKeychainEmptyMarkerPath = previousLegacyMarkerPath
	_ = os.RemoveAll(tempDir)

	os.Exit(code)
}
