package web

import (
	"os"
	"testing"

	"github.com/99designs/keyring"
)

func TestMain(m *testing.M) {
	sessionKeyringOpen = func() (keyring.Keyring, error) {
		panic("internal/web test opened the system keychain for web sessions; install a fake keyring")
	}
	passwordKeyringOpen = func() (keyring.Keyring, error) {
		panic("internal/web test opened the system keychain for web passwords; install a fake keyring")
	}
	os.Exit(m.Run())
}
