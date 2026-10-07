//go:build windows

package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/99designs/keyring"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/config"
)

// This opt-in CI smoke uses only synthetic keys and owned credential targets.
// Each CLI invocation is a fresh process; auth unit-test keyring mocks cannot
// substitute for Windows Credential Manager. No network validation is used.
func TestWindowsCredentialManagerNativeRoundTrip(t *testing.T) {
	if os.Getenv("ASC_WINDOWS_CREDENTIAL_SMOKE") != "1" {
		t.Skip("native Windows credential smoke is enabled explicitly by Windows CI")
	}
	binary, err := filepath.Abs(os.Getenv("ASC_WINDOWS_SMOKE_BINARY"))
	if err != nil || os.Getenv("ASC_WINDOWS_SMOKE_BINARY") == "" {
		t.Fatal("ASC_WINDOWS_SMOKE_BINARY must identify the built CLI")
	}
	kr, err := keyring.Open(keyring.Config{ServiceName: "asc", AllowedBackends: []keyring.BackendType{keyring.WinCredBackend}})
	if err != nil {
		t.Fatal(err)
	}
	profiles := []string{fmt.Sprintf("asc-ci-%d-native", time.Now().UnixNano()), fmt.Sprintf("asc-ci-%d-migrate", time.Now().UnixNano())}
	for _, name := range profiles {
		if _, err := kr.Get("asc:credential:" + name); !errors.Is(err, keyring.ErrKeyNotFound) {
			t.Fatal("fixture target must be absent before the smoke test")
		}
	}
	t.Cleanup(func() {
		for _, name := range profiles {
			if err := kr.Remove("asc:credential:" + name); err != nil && !errors.Is(err, keyring.ErrKeyNotFound) {
				t.Errorf("clean up owned fixture: %v", err)
			}
		}
	})
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	var childEnv []string
	for _, value := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(value), "ASC_") && !strings.HasPrefix(value, "DO_NOT_TRACK=") {
			childEnv = append(childEnv, value)
		}
	}
	childEnv = append(childEnv, "ASC_CONFIG_PATH="+configPath, "ASC_BYPASS_KEYCHAIN=0", "DO_NOT_TRACK=1")
	run := func(args ...string) []byte {
		t.Helper()
		cmd := exec.Command(binary, args...)
		cmd.Env, cmd.Dir = childEnv, dir
		output, err := cmd.Output()
		if err != nil {
			// Never print JWTs or credential blobs on failure.
			t.Fatalf("CLI %s failed: %v", args[0], err)
		}
		return output
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	keyPath := filepath.Join(dir, "fixture.p8")
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	for index, name := range profiles {
		args := []string{"auth", "login", "--name", name, "--key-id", fmt.Sprintf("WINDOWS00%d", index), "--issuer-id", "00000000-0000-4000-8000-000000000000", "--private-key", keyPath}
		if index == 1 {
			run(append(args, "--bypass-keychain")...)
			cfg, err := config.LoadAt(configPath)
			if err != nil || cfg == nil || len(cfg.Keys) != 1 {
				t.Fatal("bypass login must create the config-backed fixture")
			}
		}
		run(args...)
		item, err := kr.Get("asc:credential:" + name)
		if err != nil || !strings.Contains(string(item.Data), "private_key_pem") {
			t.Fatal("native login did not preserve encrypted key material in the existing WinCred namespace")
		}
		cfg, err := config.LoadAt(configPath)
		if err != nil || cfg == nil || len(cfg.Keys) != 0 || cfg.KeyID != "" || cfg.IssuerID != "" || cfg.PrivateKeyPath != "" {
			t.Fatal("native login retained config-backed credential fields")
		}
	}
	if err := os.Remove(keyPath); err != nil {
		t.Fatal(err)
	}
	for index, name := range profiles {
		run("auth", "switch", "--name", name)
		for range 2 {
			var status authStatusOutput
			if err := json.Unmarshal(run("auth", "status", "--output", "json"), &status); err != nil {
				t.Fatal(err)
			}
			foundDefault := false
			for _, credential := range status.Credentials {
				if credential.Name == name && credential.IsDefault && credential.StoredIn == "keychain" {
					foundDefault = true
				}
			}
			if !foundDefault {
				t.Fatal("fresh process lost the selected native profile")
			}
			var token struct {
				Token string `json:"token"`
				KeyID string `json:"keyId"`
			}
			if err := json.Unmarshal(run("auth", "token", "--confirm", "--output", "json"), &token); err != nil {
				t.Fatal(err)
			}
			parts := strings.Split(token.Token, ".")
			if len(parts) != 3 || token.KeyID != fmt.Sprintf("WINDOWS00%d", index) {
				t.Fatal("fresh process resolved the wrong native credential")
			}
			signature, err := base64.RawURLEncoding.DecodeString(parts[2])
			if err != nil || len(signature) != 64 {
				t.Fatal("invalid ES256 signature")
			}
			digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
			if !ecdsa.Verify(&key.PublicKey, digest[:], new(big.Int).SetBytes(signature[:32]), new(big.Int).SetBytes(signature[32:])) {
				t.Fatal("token did not use the key retained by Credential Manager")
			}
		}
	}
	for index, name := range profiles {
		run("auth", "logout", "--name", name, "--confirm")
		if _, err := kr.Get("asc:credential:" + name); !errors.Is(err, keyring.ErrKeyNotFound) {
			t.Fatal("named logout retained its owned native credential")
		}
		if index == 0 {
			if _, err := kr.Get("asc:credential:" + profiles[1]); err != nil {
				t.Fatal("named logout removed another profile")
			}
		}
	}
}
