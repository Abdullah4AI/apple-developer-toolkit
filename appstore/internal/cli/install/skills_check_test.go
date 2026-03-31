package install

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/config"
)

func TestSkillsAutoCheckEnabled(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "default enabled", value: "", want: true},
		{name: "true", value: "true", want: true},
		{name: "yes", value: "yes", want: true},
		{name: "y", value: "y", want: true},
		{name: "on", value: "on", want: true},
		{name: "one", value: "1", want: true},
		{name: "false", value: "false", want: false},
		{name: "no", value: "no", want: false},
		{name: "n", value: "n", want: false},
		{name: "off", value: "off", want: false},
		{name: "zero", value: "0", want: false},
		{name: "invalid falls back to enabled", value: "maybe", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := skillsAutoCheckEnabled(tt.value); got != tt.want {
				t.Fatalf("skillsAutoCheckEnabled(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestShouldRunSkillsCheck(t *testing.T) {
	now := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
	if !shouldRunSkillsCheck(now, "") {
		t.Fatal("expected empty timestamp to trigger check")
	}
	if !shouldRunSkillsCheck(now, "not-a-time") {
		t.Fatal("expected invalid timestamp to trigger check")
	}

	recent := now.Add(-2 * time.Hour).Format(skillsCheckedAtLayout)
	if shouldRunSkillsCheck(now, recent) {
		t.Fatal("expected recent timestamp to skip check")
	}

	old := now.Add(-26 * time.Hour).Format(skillsCheckedAtLayout)
	if !shouldRunSkillsCheck(now, old) {
		t.Fatal("expected old timestamp to trigger check")
	}
}

func TestSkillsOutputHasUpdates(t *testing.T) {
	if skillsOutputHasUpdates("all skills are up to date") {
		t.Fatal("expected up-to-date output to report no updates")
	}
	if skillsOutputHasUpdates("no update available") {
		t.Fatal("expected singular no-update output to report no updates")
	}
	if !skillsOutputHasUpdates("2 updates available") {
		t.Fatal("expected updates-available output to report updates")
	}
	if !skillsOutputHasUpdates("Update available for find-skills") {
		t.Fatal("expected singular update output to report updates")
	}
}

func TestMaybeCheckForSkillUpdates_NotifiesAndPersistsTimestamp(t *testing.T) {
	origLoad := loadConfigForSkillsCheck
	origPersist := persistSkillsCheckedAtForCheck
	origNow := nowForSkillsCheck
	origRun := runSkillsCheckCommand
	origProgress := progressEnabledForCheck
	t.Cleanup(func() {
		loadConfigForSkillsCheck = origLoad
		persistSkillsCheckedAtForCheck = origPersist
		nowForSkillsCheck = origNow
		runSkillsCheckCommand = origRun
		progressEnabledForCheck = origProgress
	})

	t.Setenv(skillsAutoCheckEnvVar, "true")
	t.Setenv("CI", "")

	cfg := &config.Config{}
	loadConfigForSkillsCheck = func() (*config.Config, error) { return cfg, nil }

	savedAt := ""
	persistSkillsCheckedAtForCheck = func(value string) error {
		savedAt = strings.TrimSpace(value)
		return nil
	}

	fixedNow := time.Date(2026, 3, 5, 12, 30, 0, 0, time.UTC)
	nowForSkillsCheck = func() time.Time { return fixedNow }
	runSkillsCheckCommand = func(ctx context.Context) (string, error) {
		return "2 updates available", nil
	}
	progressEnabledForCheck = func() bool { return true }

	stderr := captureStderr(t, func() {
		MaybeCheckForSkillUpdates(context.Background())
	})

	if savedAt != fixedNow.Format(skillsCheckedAtLayout) {
		t.Fatalf("SkillsCheckedAt = %q, want %q", savedAt, fixedNow.Format(skillsCheckedAtLayout))
	}
	if !strings.Contains(stderr, "npx skills update") {
		t.Fatalf("expected notification in stderr, got %q", stderr)
	}
}

func TestMaybeCheckForSkillUpdates_SkipsWhenCheckedRecently(t *testing.T) {
	origLoad := loadConfigForSkillsCheck
	origPersist := persistSkillsCheckedAtForCheck
	origNow := nowForSkillsCheck
	origRun := runSkillsCheckCommand
	origProgress := progressEnabledForCheck
	t.Cleanup(func() {
		loadConfigForSkillsCheck = origLoad
		persistSkillsCheckedAtForCheck = origPersist
		nowForSkillsCheck = origNow
		runSkillsCheckCommand = origRun
		progressEnabledForCheck = origProgress
	})

	t.Setenv(skillsAutoCheckEnvVar, "true")
	t.Setenv("CI", "")

	fixedNow := time.Date(2026, 3, 5, 15, 0, 0, 0, time.UTC)
	nowForSkillsCheck = func() time.Time { return fixedNow }
	loadConfigForSkillsCheck = func() (*config.Config, error) {
		return &config.Config{SkillsCheckedAt: fixedNow.Add(-1 * time.Hour).Format(skillsCheckedAtLayout)}, nil
	}
	persistSkillsCheckedAtForCheck = func(value string) error {
		t.Fatal("persist should not be called for recent checks")
		return nil
	}

	called := false
	runSkillsCheckCommand = func(ctx context.Context) (string, error) {
		called = true
		return "", nil
	}
	progressEnabledForCheck = func() bool { return true }

	MaybeCheckForSkillUpdates(context.Background())
	if called {
		t.Fatal("expected skills check command to be skipped")
	}
}

func TestMaybeCheckForSkillUpdates_DoesNotPersistWhenCheckCanceled(t *testing.T) {
	origLoad := loadConfigForSkillsCheck
	origPersist := persistSkillsCheckedAtForCheck
	origNow := nowForSkillsCheck
	origRun := runSkillsCheckCommand
	origProgress := progressEnabledForCheck
	t.Cleanup(func() {
		loadConfigForSkillsCheck = origLoad
		persistSkillsCheckedAtForCheck = origPersist
		nowForSkillsCheck = origNow
		runSkillsCheckCommand = origRun
		progressEnabledForCheck = origProgress
	})

	t.Setenv(skillsAutoCheckEnvVar, "true")
	t.Setenv("CI", "")
	loadConfigForSkillsCheck = func() (*config.Config, error) { return &config.Config{}, nil }
	nowForSkillsCheck = func() time.Time { return time.Date(2026, 3, 5, 16, 0, 0, 0, time.UTC) }
	progressEnabledForCheck = func() bool { return true }
	runSkillsCheckCommand = func(ctx context.Context) (string, error) {
		return "", context.Canceled
	}

	persistCalled := false
	persistSkillsCheckedAtForCheck = func(value string) error {
		persistCalled = true
		return nil
	}

	MaybeCheckForSkillUpdates(context.Background())
	if persistCalled {
		t.Fatal("expected canceled check not to persist timestamp")
	}
}

func TestMaybeCheckForSkillUpdates_DoesNotPersistWhenCheckerUnavailable(t *testing.T) {
	origLoad := loadConfigForSkillsCheck
	origPersist := persistSkillsCheckedAtForCheck
	origNow := nowForSkillsCheck
	origRun := runSkillsCheckCommand
	origProgress := progressEnabledForCheck
	t.Cleanup(func() {
		loadConfigForSkillsCheck = origLoad
		persistSkillsCheckedAtForCheck = origPersist
		nowForSkillsCheck = origNow
		runSkillsCheckCommand = origRun
		progressEnabledForCheck = origProgress
	})

	t.Setenv(skillsAutoCheckEnvVar, "true")
	t.Setenv("CI", "")
	loadConfigForSkillsCheck = func() (*config.Config, error) { return &config.Config{}, nil }
	nowForSkillsCheck = func() time.Time { return time.Date(2026, 3, 5, 16, 30, 0, 0, time.UTC) }
	progressEnabledForCheck = func() bool { return true }
	runSkillsCheckCommand = func(ctx context.Context) (string, error) {
		return "", errSkillsCheckUnavailable
	}

	persistCalled := false
	persistSkillsCheckedAtForCheck = func(value string) error {
		persistCalled = true
		return nil
	}

	MaybeCheckForSkillUpdates(context.Background())
	if persistCalled {
		t.Fatal("expected unavailable checker not to persist timestamp")
	}
}

func TestMaybeCheckForSkillUpdates_PersistsOnNonContextFailure(t *testing.T) {
	origLoad := loadConfigForSkillsCheck
	origPersist := persistSkillsCheckedAtForCheck
	origNow := nowForSkillsCheck
	origRun := runSkillsCheckCommand
	origProgress := progressEnabledForCheck
	t.Cleanup(func() {
		loadConfigForSkillsCheck = origLoad
		persistSkillsCheckedAtForCheck = origPersist
		nowForSkillsCheck = origNow
		runSkillsCheckCommand = origRun
		progressEnabledForCheck = origProgress
	})

	t.Setenv(skillsAutoCheckEnvVar, "true")
	t.Setenv("CI", "")
	loadConfigForSkillsCheck = func() (*config.Config, error) { return &config.Config{}, nil }
	fixedNow := time.Date(2026, 3, 5, 17, 0, 0, 0, time.UTC)
	nowForSkillsCheck = func() time.Time { return fixedNow }
	progressEnabledForCheck = func() bool { return true }
	runSkillsCheckCommand = func(ctx context.Context) (string, error) {
		return "", errors.New("exec failure")
	}

	savedAt := ""
	persistSkillsCheckedAtForCheck = func(value string) error {
		savedAt = strings.TrimSpace(value)
		return nil
	}

	MaybeCheckForSkillUpdates(context.Background())
	if savedAt != fixedNow.Format(skillsCheckedAtLayout) {
		t.Fatalf("expected persistence on non-context failure, got %q", savedAt)
	}
}

func TestMaybeCheckForSkillUpdates_SkipsWhenDisabled(t *testing.T) {
	origLoad := loadConfigForSkillsCheck
	origProgress := progressEnabledForCheck
	t.Cleanup(func() {
		loadConfigForSkillsCheck = origLoad
		progressEnabledForCheck = origProgress
	})

	t.Setenv(skillsAutoCheckEnvVar, "false")
	progressEnabledForCheck = func() bool { return true }
	loadCalled := false
	loadConfigForSkillsCheck = func() (*config.Config, error) {
		loadCalled = true
		return nil, errors.New("should not load")
	}

	MaybeCheckForSkillUpdates(context.Background())
	if loadCalled {
		t.Fatal("expected config load to be skipped when disabled")
	}
}

func TestMaybeCheckForSkillUpdates_RunsByDefaultWhenUnset(t *testing.T) {
	origLoad := loadConfigForSkillsCheck
	origProgress := progressEnabledForCheck
	t.Cleanup(func() {
		loadConfigForSkillsCheck = origLoad
		progressEnabledForCheck = origProgress
	})

	t.Setenv(skillsAutoCheckEnvVar, "")
	t.Setenv("CI", "")
	progressEnabledForCheck = func() bool { return true }
	loadCalled := false
	loadConfigForSkillsCheck = func() (*config.Config, error) {
		loadCalled = true
		return nil, errors.New("load called as expected")
	}

	MaybeCheckForSkillUpdates(context.Background())
	if !loadCalled {
		t.Fatal("expected config load to run when auto-check env var is unset")
	}
}

func TestDefaultRunSkillsCheckCommand_UsesSkillsBinaryCheckCommand(t *testing.T) {
	origLookup := lookupSkillsCheckCLI
	origLookupNpx := lookupNpx
	t.Cleanup(func() {
		lookupSkillsCheckCLI = origLookup
		lookupNpx = origLookupNpx
	})

	mockSkills := filepath.Join(t.TempDir(), "skills-mock.sh")
	if err := os.WriteFile(mockSkills, []byte("#!/bin/sh\necho \"$@\"\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := os.Chmod(mockSkills, 0o755); err != nil {
		t.Fatalf("Chmod() error: %v", err)
	}

	lookupSkillsCheckCLI = func(file string) (string, error) {
		if file != "skills" {
			t.Fatalf("lookupSkillsCheckCLI called with %q, want skills", file)
		}
		return mockSkills, nil
	}
	lookupNpx = func(file string) (string, error) {
		t.Fatalf("lookupNpx should not be called when skills binary is available")
		return "", errors.New("unexpected call")
	}

	output, err := defaultRunSkillsCheckCommand(context.Background())
	if err != nil {
		t.Fatalf("defaultRunSkillsCheckCommand() error: %v", err)
	}
	if !strings.Contains(output, "check") {
		t.Fatalf("expected check invocation, got %q", output)
	}
	if strings.Contains(output, "--no") {
		t.Fatalf("expected no npx flags in invocation, got %q", output)
	}
}

func TestDefaultRunSkillsCheckCommand_MissingSkillsCLIIsNoop(t *testing.T) {
	origLookup := lookupSkillsCheckCLI
	origLookupNpx := lookupNpx
	t.Cleanup(func() {
		lookupSkillsCheckCLI = origLookup
		lookupNpx = origLookupNpx
	})

	lookupSkillsCheckCLI = func(file string) (string, error) {
		return "", errors.New("not found")
	}
	lookupNpx = func(file string) (string, error) {
		return "", errors.New("npx not found")
	}

	output, err := defaultRunSkillsCheckCommand(context.Background())
	if !errors.Is(err, errSkillsCheckUnavailable) {
		t.Fatalf("expected errSkillsCheckUnavailable when check command is unavailable, got %v", err)
	}
	if output != "" {
		t.Fatalf("expected empty output when skills is unavailable, got %q", output)
	}
}

func TestDefaultRunSkillsCheckCommand_FallsBackToNpxOffline(t *testing.T) {
	origLookup := lookupSkillsCheckCLI
	origLookupNpx := lookupNpx
	t.Cleanup(func() {
		lookupSkillsCheckCLI = origLookup
		lookupNpx = origLookupNpx
	})

	mockNpx := filepath.Join(t.TempDir(), "npx-mock.sh")
	if err := os.WriteFile(mockNpx, []byte("#!/bin/sh\necho \"$@\"\necho \"$npm_config_offline\"\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := os.Chmod(mockNpx, 0o755); err != nil {
		t.Fatalf("Chmod() error: %v", err)
	}

	lookupSkillsCheckCLI = func(file string) (string, error) {
		return "", errors.New("not found")
	}
	lookupNpx = func(file string) (string, error) {
		if file != "npx" {
			t.Fatalf("lookupNpx called with %q, want npx", file)
		}
		return mockNpx, nil
	}

	output, err := defaultRunSkillsCheckCommand(context.Background())
	if err != nil {
		t.Fatalf("defaultRunSkillsCheckCommand() error: %v", err)
	}
	if !strings.Contains(output, "--offline --yes skills check") {
		t.Fatalf("expected --offline --yes skills check invocation, got %q", output)
	}
	if !strings.Contains(output, "\ntrue\n") && !strings.HasSuffix(output, "\ntrue") {
		t.Fatalf("expected npm_config_offline=true in fallback environment, got %q", output)
	}
}

func TestDefaultRunSkillsCheckCommand_OfflineCacheMissIsUnavailable(t *testing.T) {
	origLookup := lookupSkillsCheckCLI
	origLookupNpx := lookupNpx
	t.Cleanup(func() {
		lookupSkillsCheckCLI = origLookup
		lookupNpx = origLookupNpx
	})

	mockNpx := filepath.Join(t.TempDir(), "npx-mock.sh")
	script := "#!/bin/sh\necho \"npm ERR! code ENOTCACHED\" 1>&2\nexit 1\n"
	if err := os.WriteFile(mockNpx, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := os.Chmod(mockNpx, 0o755); err != nil {
		t.Fatalf("Chmod() error: %v", err)
	}

	lookupSkillsCheckCLI = func(file string) (string, error) {
		return "", errors.New("not found")
	}
	lookupNpx = func(file string) (string, error) {
		return mockNpx, nil
	}

	output, err := defaultRunSkillsCheckCommand(context.Background())
	if !errors.Is(err, errSkillsCheckUnavailable) {
		t.Fatalf("expected errSkillsCheckUnavailable for ENOTCACHED fallback, got %v", err)
	}
	if !strings.Contains(strings.ToLower(output), "enotcached") {
		t.Fatalf("expected ENOTCACHED output, got %q", output)
	}
}

func TestDefaultRunSkillsCheckCommand_UsesNonProjectWorkingDirectory(t *testing.T) {
	origLookup := lookupSkillsCheckCLI
	t.Cleanup(func() {
		lookupSkillsCheckCLI = origLookup
	})

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	projectDir := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("Chdir(projectDir) error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	mockSkills := filepath.Join(t.TempDir(), "skills-mock.sh")
	if err := os.WriteFile(mockSkills, []byte("#!/bin/sh\nprintf \"%s\\n\" \"$PWD\"\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := os.Chmod(mockSkills, 0o755); err != nil {
		t.Fatalf("Chmod() error: %v", err)
	}

	lookupSkillsCheckCLI = func(file string) (string, error) {
		return mockSkills, nil
	}

	output, err := defaultRunSkillsCheckCommand(context.Background())
	if err != nil {
		t.Fatalf("defaultRunSkillsCheckCommand() error: %v", err)
	}
	workingDir := strings.TrimSpace(output)
	normalizedWorkingDir := workingDir
	if resolved, resolveErr := filepath.EvalSymlinks(workingDir); resolveErr == nil {
		normalizedWorkingDir = resolved
	}
	normalizedHomeDir := homeDir
	if resolved, resolveErr := filepath.EvalSymlinks(homeDir); resolveErr == nil {
		normalizedHomeDir = resolved
	}
	if normalizedWorkingDir != normalizedHomeDir {
		t.Fatalf("expected command working directory %q, got %q", normalizedHomeDir, normalizedWorkingDir)
	}
	normalizedProjectDir := projectDir
	if resolved, resolveErr := filepath.EvalSymlinks(projectDir); resolveErr == nil {
		normalizedProjectDir = resolved
	}
	if normalizedWorkingDir == normalizedProjectDir {
		t.Fatalf("expected command not to run in project directory %q", normalizedProjectDir)
	}
}

func TestDefaultRunSkillsCheckCommand_SkipsProjectLocalSkillsBinary(t *testing.T) {
	origLookup := lookupSkillsCheckCLI
	origLookupNpx := lookupNpx
	t.Cleanup(func() {
		lookupSkillsCheckCLI = origLookup
		lookupNpx = origLookupNpx
	})

	repoRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoRoot, ".git"), []byte("gitdir"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	workingDir := filepath.Join(repoRoot, "subdir")
	if err := os.MkdirAll(workingDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}
	if err := os.Chdir(workingDir); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	localSkills := filepath.Join(repoRoot, "node_modules", ".bin", "skills")
	if err := os.MkdirAll(filepath.Dir(localSkills), 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(localSkills, []byte("#!/bin/sh\necho should-not-run\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := os.Chmod(localSkills, 0o755); err != nil {
		t.Fatalf("Chmod() error: %v", err)
	}

	lookupSkillsCheckCLI = func(file string) (string, error) {
		return localSkills, nil
	}
	lookupNpx = func(file string) (string, error) {
		return "", errors.New("npx unavailable")
	}

	output, err := defaultRunSkillsCheckCommand(context.Background())
	if !errors.Is(err, errSkillsCheckUnavailable) {
		t.Fatalf("expected errSkillsCheckUnavailable for skipped local binary fallback failure, got %v", err)
	}
	if output != "" {
		t.Fatalf("expected project-local skills binary to be skipped, got %q", output)
	}
}

func TestShouldSkipProjectLocalSkillsBinary_NoRepoRootDoesNotSkip(t *testing.T) {
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}
	workingDir := t.TempDir()
	if err := os.Chdir(workingDir); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	binaryPath := filepath.Join(workingDir, "bin", "skills")
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if shouldSkipProjectLocalSkillsBinary(binaryPath) {
		t.Fatal("expected no-repo-root working directory not to skip binary path")
	}
}

func TestDefaultPersistSkillsCheckedAt_PreservesUnknownFields(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", cfgPath)

	initial := `{
  "key_id": "ABC123",
  "custom_future_key": "keep-me",
  "custom_nested": {"enabled": true},
  "skills_checked_at": "2026-03-01T00:00:00Z"
}`
	if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	want := "2026-03-05T14:00:00Z"
	if err := defaultPersistSkillsCheckedAt(want); err != nil {
		t.Fatalf("defaultPersistSkillsCheckedAt() error: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	var got string
	if err := json.Unmarshal(doc["skills_checked_at"], &got); err != nil {
		t.Fatalf("unmarshal skills_checked_at error: %v", err)
	}
	if got != want {
		t.Fatalf("skills_checked_at = %q, want %q", got, want)
	}

	if _, ok := doc["custom_future_key"]; !ok {
		t.Fatal("expected custom_future_key to be preserved")
	}
	if _, ok := doc["custom_nested"]; !ok {
		t.Fatal("expected custom_nested to be preserved")
	}
}

func TestDefaultPersistSkillsCheckedAt_PreservesTopLevelKeyOrder(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", cfgPath)

	initial := `{
  "z_key": "z",
  "skills_checked_at": "2026-03-01T00:00:00Z",
  "a_key": "a"
}`
	if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	if err := defaultPersistSkillsCheckedAt("2026-03-05T18:00:00Z"); err != nil {
		t.Fatalf("defaultPersistSkillsCheckedAt() error: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	content := string(data)
	zIndex := strings.Index(content, `"z_key"`)
	skillsIndex := strings.Index(content, `"skills_checked_at"`)
	aIndex := strings.Index(content, `"a_key"`)
	if zIndex == -1 || skillsIndex == -1 || aIndex == -1 {
		t.Fatalf("expected keys in output, got %q", content)
	}
	if zIndex >= skillsIndex || skillsIndex >= aIndex {
		t.Fatalf("expected top-level key order to be preserved, got %q", content)
	}
}

func TestDefaultPersistSkillsCheckedAt_NullJSONCreatesObject(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", cfgPath)

	if err := os.WriteFile(cfgPath, []byte("null"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	want := "2026-03-05T15:00:00Z"
	if err := defaultPersistSkillsCheckedAt(want); err != nil {
		t.Fatalf("defaultPersistSkillsCheckedAt() error: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	var got string
	if err := json.Unmarshal(doc["skills_checked_at"], &got); err != nil {
		t.Fatalf("unmarshal skills_checked_at error: %v", err)
	}
	if got != want {
		t.Fatalf("skills_checked_at = %q, want %q", got, want)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}
	os.Stderr = w

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		_ = r.Close()
		done <- buf.String()
	}()

	defer func() {
		os.Stderr = oldStderr
		_ = w.Close()
	}()

	fn()
	_ = w.Close()
	return <-done
}
