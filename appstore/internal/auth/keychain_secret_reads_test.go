package auth

import (
	"errors"
	"os"
	"testing"

	"github.com/99designs/keyring"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/config"
)

type staleListingKeyring struct {
	keyring.Keyring
	keys []string
}

func (k staleListingKeyring) Keys() ([]string, error) {
	return append([]string(nil), k.keys...), nil
}

// Every keychain secret read is a separate macOS authorization prompt when
// the running binary is not yet trusted for that item. Resolving one profile
// must therefore read exactly the selected item's secret.
func withCountingKeyring(t *testing.T) *countingKeyring {
	t.Helper()
	withSeparateKeyrings(t)
	counting := &countingKeyring{inner: keyring.NewArrayKeyring(nil)}
	keyringOpener = func() (keyring.Keyring, error) {
		return counting, nil
	}
	return counting
}

func assertSecretReads(t *testing.T, counting *countingKeyring, want ...string) {
	t.Helper()
	if len(counting.getKeys) != len(want) {
		t.Fatalf("expected keychain secret reads %v, got %v", want, counting.getKeys)
	}
	for i, key := range want {
		if counting.getKeys[i] != key {
			t.Fatalf("expected keychain secret reads %v, got %v", want, counting.getKeys)
		}
	}
}

func TestGetCredentialsWithSource_ExplicitProfileReadsOnlySelectedSecret(t *testing.T) {
	counting := withCountingKeyring(t)
	storeCredentialInKeyring(t, counting, "alpha", "ALPHA-KEY", "ALPHA-ISSUER", "/tmp/alpha.p8")
	storeCredentialInKeyring(t, counting, "beta", "BETA-KEY", "BETA-ISSUER", "/tmp/beta.p8")

	cfg, source, err := GetCredentialsWithSource("beta")
	if err != nil {
		t.Fatalf("GetCredentialsWithSource(beta) error: %v", err)
	}
	if source != "keychain" {
		t.Fatalf("expected keychain source, got %q", source)
	}
	if cfg.KeyID != "BETA-KEY" || cfg.DefaultKeyName != "beta" {
		t.Fatalf("unexpected credential resolved: %+v", cfg)
	}
	assertSecretReads(t, counting, keyringKey("beta"))
}

func TestGetCredentialsWithSource_DefaultNameReadsOnlyDefaultSecret(t *testing.T) {
	counting := withCountingKeyring(t)
	storeCredentialInKeyring(t, counting, "alpha", "ALPHA-KEY", "ALPHA-ISSUER", "/tmp/alpha.p8")
	storeCredentialInKeyring(t, counting, "beta", "BETA-KEY", "BETA-ISSUER", "/tmp/beta.p8")
	if err := saveDefaultName("alpha"); err != nil {
		t.Fatalf("saveDefaultName() error: %v", err)
	}

	cfg, source, err := GetCredentialsWithSource("")
	if err != nil {
		t.Fatalf("GetCredentialsWithSource(default) error: %v", err)
	}
	if source != "keychain" {
		t.Fatalf("expected keychain source, got %q", source)
	}
	if cfg.KeyID != "ALPHA-KEY" || cfg.DefaultKeyName != "alpha" {
		t.Fatalf("unexpected credential resolved: %+v", cfg)
	}
	assertSecretReads(t, counting, keyringKey("alpha"))
}

func TestGetCredentialsWithSource_SingleCredentialReadsOneSecret(t *testing.T) {
	counting := withCountingKeyring(t)
	storeCredentialInKeyring(t, counting, "only", "ONLY-KEY", "ONLY-ISSUER", "/tmp/only.p8")

	cfg, source, err := GetCredentialsWithSource("")
	if err != nil {
		t.Fatalf("GetCredentialsWithSource(default) error: %v", err)
	}
	if source != "keychain" {
		t.Fatalf("expected keychain source, got %q", source)
	}
	if cfg.KeyID != "ONLY-KEY" || cfg.DefaultKeyName != "only" {
		t.Fatalf("unexpected credential resolved: %+v", cfg)
	}
	assertSecretReads(t, counting, keyringKey("only"))
}

func TestGetCredentialsWithSource_AmbiguousDefaultReadsNoSecret(t *testing.T) {
	counting := withCountingKeyring(t)
	storeCredentialInKeyring(t, counting, "alpha", "ALPHA-KEY", "ALPHA-ISSUER", "/tmp/alpha.p8")
	storeCredentialInKeyring(t, counting, "beta", "BETA-KEY", "BETA-ISSUER", "/tmp/beta.p8")

	_, _, err := GetCredentialsWithSource("")
	if !errors.Is(err, ErrDefaultCredentialsNotFound) {
		t.Fatalf("expected ErrDefaultCredentialsNotFound, got %v", err)
	}
	assertSecretReads(t, counting)
}

func TestGetCredentialsWithSource_UnknownProfileReadsNoSecret(t *testing.T) {
	counting := withCountingKeyring(t)
	storeCredentialInKeyring(t, counting, "alpha", "ALPHA-KEY", "ALPHA-ISSUER", "/tmp/alpha.p8")

	_, _, err := GetCredentialsWithSource("missing")
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
	assertSecretReads(t, counting)
}

func TestGetCredentialsWithSource_DisappearingOnlyCredentialFallsBackToConfig(t *testing.T) {
	current, _ := withSeparateKeyrings(t)
	keyringOpener = func() (keyring.Keyring, error) {
		return staleListingKeyring{
			Keyring: current,
			keys:    []string{keyringKey("stale")},
		}, nil
	}

	configPath := os.Getenv("ASC_CONFIG_PATH")
	if configPath == "" {
		t.Fatal("expected ASC_CONFIG_PATH to be set")
	}
	if err := config.SaveAt(configPath, &config.Config{
		Keys: []config.Credential{{
			Name:           "fallback",
			KeyID:          "FALLBACK-KEY",
			IssuerID:       "FALLBACK-ISSUER",
			PrivateKeyPath: "/tmp/fallback.p8",
		}},
	}); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	cfg, source, err := GetCredentialsWithSource("")
	if err != nil {
		t.Fatalf("GetCredentialsWithSource(default) error: %v", err)
	}
	if source != "config" {
		t.Fatalf("expected config source, got %q", source)
	}
	if cfg.KeyID != "FALLBACK-KEY" || cfg.DefaultKeyName != "fallback" {
		t.Fatalf("unexpected credential resolved: %+v", cfg)
	}
}
