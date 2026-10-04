package auth

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/config"
)

// isolateConfigStoreHome points HOME at a temp directory and moves the working
// directory to a temp repo so the global path and the upward local search both
// stay inside the test.
func isolateConfigStoreHome(t *testing.T) (globalPath, workDir string) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	workDir = t.TempDir()
	if err := os.Mkdir(filepath.Join(workDir, ".git"), 0o755); err != nil {
		t.Fatalf("Mkdir(.git) error: %v", err)
	}
	t.Chdir(workDir)
	return filepath.Join(home, ".asc", "config.json"), workDir
}

func assertConfigCredentialNames(t *testing.T, path string, want ...string) {
	t.Helper()

	cfg, err := config.LoadAt(path)
	if err != nil {
		t.Fatalf("LoadAt(%s) error: %v", path, err)
	}
	if len(cfg.Keys) != len(want) {
		t.Fatalf("credentials in %s = %+v, want names %v", path, cfg.Keys, want)
	}
	for index, name := range want {
		if cfg.Keys[index].Name != name {
			t.Fatalf("credentials in %s = %+v, want names %v", path, cfg.Keys, want)
		}
	}
}

func assertConfigMissing(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no config at %s, got stat error %v", path, err)
	}
}

func TestStoreCredentialsConfigWritesConfigPathOverride(t *testing.T) {
	globalPath, _ := isolateConfigStoreHome(t)
	overridePath := filepath.Join(t.TempDir(), "isolated", "config.json")
	t.Setenv("ASC_CONFIG_PATH", overridePath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	if err := StoreCredentialsConfigWithKeyType("Isolated", "KEY123", "ISS456", "/tmp/AuthKey.p8", config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentialsConfigWithKeyType() error: %v", err)
	}

	assertConfigCredentialNames(t, overridePath, "Isolated")
	assertConfigMissing(t, globalPath)

	credentials, err := ListCredentials()
	if err != nil {
		t.Fatalf("ListCredentials() error: %v", err)
	}
	if len(credentials) != 1 || credentials[0].Name != "Isolated" || credentials[0].SourcePath != overridePath {
		t.Fatalf("ListCredentials() = %+v, want Isolated from %s", credentials, overridePath)
	}
}

func TestStoreCredentialsConfigWithoutOverrideKeepsGlobalConfig(t *testing.T) {
	globalPath, workDir := isolateConfigStoreHome(t)
	t.Setenv("ASC_CONFIG_PATH", "")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	localPath := filepath.Join(workDir, ".asc", "config.json")
	if err := config.SaveAt(localPath, &config.Config{AppID: "123"}); err != nil {
		t.Fatalf("SaveAt(local) error: %v", err)
	}

	if err := StoreCredentialsConfigWithKeyType("Global", "KEY123", "ISS456", "/tmp/AuthKey.p8", config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentialsConfigWithKeyType() error: %v", err)
	}

	assertConfigCredentialNames(t, globalPath, "Global")
	assertConfigCredentialNames(t, localPath)
}

func TestStoreCredentialsKeychainIgnoresConfigPathOverride(t *testing.T) {
	globalPath, _ := isolateConfigStoreHome(t)
	withArrayKeyring(t)
	overridePath := os.Getenv("ASC_CONFIG_PATH")

	if err := StoreCredentialsWithKeyType("Keychain", "KEY123", "ISS456", "/tmp/AuthKey.p8", config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentialsWithKeyType() error: %v", err)
	}

	credentials, err := ListCredentials()
	if err != nil {
		t.Fatalf("ListCredentials() error: %v", err)
	}
	if len(credentials) != 1 || credentials[0].Name != "Keychain" || credentials[0].Source != "keychain" {
		t.Fatalf("ListCredentials() = %+v, want Keychain from the keychain", credentials)
	}
	if cfg, err := config.LoadAt(overridePath); err == nil && len(cfg.Keys) != 0 {
		t.Fatalf("expected no config credentials at %s, got %+v", overridePath, cfg.Keys)
	}
	assertConfigMissing(t, globalPath)
}
