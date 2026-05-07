//go:build integration

package auth

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/config"
)

func TestIntegrationAuthConfig(t *testing.T) {
	keyID := os.Getenv("ASC_KEY_ID")
	issuerID := os.Getenv("ASC_ISSUER_ID")
	keyPath := os.Getenv("ASC_PRIVATE_KEY_PATH")

	if keyID == "" || issuerID == "" || keyPath == "" {
		t.Skip("integration tests require ASC_KEY_ID, ASC_ISSUER_ID, ASC_PRIVATE_KEY_PATH")
	}

	// Find the asc binary
	ascBinary := findASCBinary(t)

	t.Run("auth_init_local_creates_config", func(t *testing.T) {
		tempDir := t.TempDir()

		// Run auth init --local in temp directory
		cmd := exec.Command(ascBinary, "auth", "init", "--local")
		cmd.Dir = tempDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("auth init --local failed: %v\nOutput: %s", err, output)
		}

		// Verify config file was created at .asc/config.json
		configPath := filepath.Join(tempDir, ".asc", "config.json")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Fatalf("auth init --local did not create config file at %s", configPath)
		}

		// Verify config is valid JSON with expected structure
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("failed to read config: %v", err)
		}

		var cfg config.Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			t.Fatalf("config is not valid JSON: %v", err)
		}

		// Verify output is JSON with config_path
		var result struct {
			ConfigPath string `json:"config_path"`
			Created    bool   `json:"created"`
		}
		if err := json.Unmarshal(output, &result); err != nil {
			t.Fatalf("auth init output is not valid JSON: %v\nOutput: %s", err, output)
		}
		if !result.Created {
			t.Fatal("auth init output should have created=true")
		}
	})

	t.Run("auth_init_force_overwrites", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create initial config with --local
		cmd := exec.Command(ascBinary, "auth", "init", "--local")
		cmd.Dir = tempDir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("auth init --local failed: %v\nOutput: %s", err, output)
		}

		// Try without --force (should fail)
		cmd = exec.Command(ascBinary, "auth", "init", "--local")
		cmd.Dir = tempDir
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatal("auth init without --force should fail when config exists")
		}
		if !strings.Contains(string(output), "already exists") {
			t.Fatalf("expected 'already exists' error, got: %s", output)
		}

		// Try with --force (should succeed)
		cmd = exec.Command(ascBinary, "auth", "init", "--local", "--force")
		cmd.Dir = tempDir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("auth init --local --force failed: %v\nOutput: %s", err, output)
		}
	})

	t.Run("auth_login_bypass_keychain_local", func(t *testing.T) {
		tempDir := t.TempDir()

		// Login with bypass-keychain --local writes to .asc/config.json in cwd
		cmd := exec.Command(
			ascBinary, "auth", "login",
			"--bypass-keychain",
			"--local",
			"--name", "TestKey",
			"--key-id", keyID,
			"--issuer-id", issuerID,
			"--private-key", keyPath,
		)
		cmd.Dir = tempDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("auth login --bypass-keychain --local failed: %v\nOutput: %s", err, output)
		}

		// Verify config file was created at .asc/config.json
		configPath := filepath.Join(tempDir, ".asc", "config.json")
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("failed to read config at %s: %v", configPath, err)
		}

		var cfg config.Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			t.Fatalf("config is not valid JSON: %v", err)
		}

		if cfg.KeyID != keyID {
			t.Fatalf("KeyID mismatch: got %q want %q", cfg.KeyID, keyID)
		}
		if cfg.IssuerID != issuerID {
			t.Fatalf("IssuerID mismatch: got %q want %q", cfg.IssuerID, issuerID)
		}
		if cfg.PrivateKeyPath != keyPath {
			t.Fatalf("PrivateKeyPath mismatch: got %q want %q", cfg.PrivateKeyPath, keyPath)
		}
		if cfg.DefaultKeyName != "TestKey" {
			t.Fatalf("DefaultKeyName mismatch: got %q want %q", cfg.DefaultKeyName, "TestKey")
		}
	})

	t.Run("auth_status_reads_config", func(t *testing.T) {
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "config.json")

		// Create config with credentials
		cfg := &config.Config{
			KeyID:          keyID,
			IssuerID:       issuerID,
			PrivateKeyPath: keyPath,
			DefaultKeyName: "ConfigKey",
		}
		if err := config.SaveAt(configPath, cfg); err != nil {
			t.Fatalf("failed to save config: %v", err)
		}

		// Check auth status
		cmd := exec.Command(ascBinary, "auth", "status")
		cmd.Env = append(
			os.Environ(),
			"ASC_CONFIG_PATH="+configPath,
			"ASC_BYPASS_KEYCHAIN=1",
		)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("auth status failed: %v\nOutput: %s", err, output)
		}

		if !strings.Contains(string(output), "ConfigKey") {
			t.Fatalf("auth status should show ConfigKey, got: %s", output)
		}
		if !strings.Contains(string(output), keyID) {
			t.Fatalf("auth status should show key ID, got: %s", output)
		}
	})

	t.Run("auth_switch_sets_default_and_profile_override", func(t *testing.T) {
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "config.json")

		cfg := &config.Config{
			DefaultKeyName: "personal",
			Keys: []config.Credential{
				{
					Name:           "personal",
					KeyID:          keyID,
					IssuerID:       issuerID,
					PrivateKeyPath: keyPath,
				},
				{
					Name:           "client",
					KeyID:          keyID,
					IssuerID:       issuerID,
					PrivateKeyPath: keyPath,
				},
			},
		}
		if err := config.SaveAt(configPath, cfg); err != nil {
			t.Fatalf("failed to save config: %v", err)
		}

		cmd := exec.Command(ascBinary, "auth", "switch", "--name", "client")
		cmd.Env = append(
			filterEnv(os.Environ(), "ASC_CONFIG_PATH", "ASC_BYPASS_KEYCHAIN"),
			"ASC_CONFIG_PATH="+configPath,
			"ASC_BYPASS_KEYCHAIN=1",
		)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("auth switch failed: %v\nOutput: %s", err, output)
		}

		updated, err := config.LoadAt(configPath)
		if err != nil {
			t.Fatalf("failed to read config: %v", err)
		}
		if updated.DefaultKeyName != "client" {
			t.Fatalf("expected DefaultKeyName=client, got %q", updated.DefaultKeyName)
		}

		cmd = exec.Command(ascBinary, "--profile", "client", "apps", "list", "--limit", "1")
		cmd.Env = append(
			filterEnv(
				os.Environ(),
				"ASC_KEY_ID", "ASC_ISSUER_ID", "ASC_PRIVATE_KEY_PATH",
				"ASC_CONFIG_PATH", "ASC_BYPASS_KEYCHAIN",
			),
			"ASC_CONFIG_PATH="+configPath,
			"ASC_BYPASS_KEYCHAIN=1",
		)
		output, err = cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("apps list with --profile failed: %v\nOutput: %s", err, output)
		}
		if !strings.Contains(string(output), `"type":"apps"`) {
			t.Fatalf("expected apps response, got: %s", output)
		}
	})

	t.Run("config_credentials_work_for_api", func(t *testing.T) {
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "config.json")

		// Create config with credentials
		cfg := &config.Config{
			KeyID:          keyID,
			IssuerID:       issuerID,
			PrivateKeyPath: keyPath,
			DefaultKeyName: "APITestKey",
		}
		if err := config.SaveAt(configPath, cfg); err != nil {
			t.Fatalf("failed to save config: %v", err)
		}

		// Try to list apps using config credentials
		cmd := exec.Command(ascBinary, "apps", "list")
		cmd.Env = filterEnv(
			os.Environ(),
			"ASC_KEY_ID", "ASC_ISSUER_ID", "ASC_PRIVATE_KEY_PATH",
		)
		cmd.Env = append(cmd.Env, "ASC_CONFIG_PATH="+configPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("apps list with config credentials failed: %v\nOutput: %s", err, output)
		}

		// Verify we got valid JSON response with apps
		if !strings.Contains(string(output), `"type":"apps"`) {
			t.Fatalf("expected apps response, got: %s", output)
		}
	})

	t.Run("config_app_id_fallback", func(t *testing.T) {
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "config.json")
		appID := os.Getenv("ASC_APP_ID")
		if appID == "" {
			t.Skip("requires ASC_APP_ID")
		}

		// Create config with app_id
		cfg := &config.Config{
			KeyID:          keyID,
			IssuerID:       issuerID,
			PrivateKeyPath: keyPath,
			DefaultKeyName: "AppIDTestKey",
			AppID:          appID,
		}
		if err := config.SaveAt(configPath, cfg); err != nil {
			t.Fatalf("failed to save config: %v", err)
		}

		// Try to list builds without --app flag (should use config app_id)
		cmd := exec.Command(ascBinary, "builds", "list", "--limit", "1")
		cmd.Env = filterEnv(
			os.Environ(),
			"ASC_KEY_ID", "ASC_ISSUER_ID", "ASC_PRIVATE_KEY_PATH", "ASC_APP_ID",
		)
		cmd.Env = append(cmd.Env, "ASC_CONFIG_PATH="+configPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("builds list with config app_id failed: %v\nOutput: %s", err, output)
		}

		// Verify we got valid JSON response
		if !strings.Contains(string(output), `"type":"builds"`) && !strings.Contains(string(output), `"data":[]`) {
			t.Fatalf("expected builds response, got: %s", output)
		}
	})

	t.Run("auth_logout_clears_credentials_preserves_settings", func(t *testing.T) {
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "config.json")

		// Create config with credentials AND other settings
		cfg := &config.Config{
			KeyID:          keyID,
			IssuerID:       issuerID,
			PrivateKeyPath: keyPath,
			DefaultKeyName: "LogoutTestKey",
			AppID:          "12345",
			VendorNumber:   "67890",
			Timeout: func() config.DurationValue {
				value, err := config.ParseDurationValue("60s")
				if err != nil {
					t.Fatalf("ParseDurationValue(\"60s\") error: %v", err)
				}
				return value
			}(),
		}
		if err := config.SaveAt(configPath, cfg); err != nil {
			t.Fatalf("failed to save config: %v", err)
		}

		// Logout
		cmd := exec.Command(ascBinary, "auth", "logout")
		cmd.Env = append(os.Environ(), "ASC_CONFIG_PATH="+configPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("auth logout failed: %v\nOutput: %s", err, output)
		}

		// Verify config still exists
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("config file should still exist: %v", err)
		}

		// Verify credentials are cleared but settings are preserved
		var loadedCfg config.Config
		if err := json.Unmarshal(data, &loadedCfg); err != nil {
			t.Fatalf("failed to parse config: %v", err)
		}

		// Credentials should be cleared
		if loadedCfg.KeyID != "" || loadedCfg.IssuerID != "" || loadedCfg.PrivateKeyPath != "" {
			t.Fatal("credentials should be cleared after logout")
		}

		// Settings should be preserved
		if loadedCfg.AppID != "12345" {
			t.Fatalf("AppID should be preserved, got %q", loadedCfg.AppID)
		}
		if loadedCfg.VendorNumber != "67890" {
			t.Fatalf("VendorNumber should be preserved, got %q", loadedCfg.VendorNumber)
		}
		if loadedCfg.Timeout.String() != "60s" {
			t.Fatalf("Timeout should be preserved, got %q", loadedCfg.Timeout.String())
		}
	})
}

// filterEnv removes specified environment variables from the env slice
func filterEnv(env []string, remove ...string) []string {
	removeSet := make(map[string]bool)
	for _, key := range remove {
		removeSet[key] = true
	}

	filtered := make([]string, 0, len(env))
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) > 0 && !removeSet[parts[0]] {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// findASCBinary locates the asc binary for integration tests.
// It searches from the current directory up to the project root.
// If the binary is not found, it attempts to build it automatically.
func findASCBinary(t *testing.T) string {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}

	// Find project root (contains go.mod)
	projectRoot := ""
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			projectRoot = dir
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	if projectRoot == "" {
		t.Fatal("could not find project root (go.mod)")
	}

	binaryPath := filepath.Join(projectRoot, "asc")

	// Check if binary exists
	if info, err := os.Stat(binaryPath); err == nil && !info.IsDir() {
		return binaryPath
	}

	// Binary not found - try to build it
	t.Log("asc binary not found, attempting to build...")
	cmd := exec.Command("go", "build", "-o", "asc", ".")
	cmd.Dir = projectRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build asc binary: %v\nOutput: %s", err, output)
	}

	// Verify binary was created
	if info, err := os.Stat(binaryPath); err != nil || info.IsDir() {
		t.Fatal("asc binary not found after build")
	}

	return binaryPath
}
