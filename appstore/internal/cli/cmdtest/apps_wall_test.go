package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func findSubcommand(root *ffcli.Command, path ...string) *ffcli.Command {
	cmd := root
	for _, part := range path {
		var next *ffcli.Command
		for _, sub := range cmd.Subcommands {
			if sub.Name == part {
				next = sub
				break
			}
		}
		if next == nil {
			return nil
		}
		cmd = next
	}
	return cmd
}

func TestAppsWallFlagDefaults(t *testing.T) {
	root := RootCommand("1.2.3")
	cmd := findSubcommand(root, "apps", "wall")
	if cmd == nil {
		t.Fatal("expected apps wall command")
	}

	outputFlag := cmd.FlagSet.Lookup("output")
	if outputFlag == nil {
		t.Fatal("expected --output flag")
	}
	if got := outputFlag.DefValue; got != "table" {
		t.Fatalf("expected --output default table, got %q", got)
	}

	sortFlag := cmd.FlagSet.Lookup("sort")
	if sortFlag == nil {
		t.Fatal("expected --sort flag")
	}
	if got := sortFlag.DefValue; got != "name" {
		t.Fatalf("expected --sort default name, got %q", got)
	}
}

func TestAppsWallSubmitCommandExists(t *testing.T) {
	root := RootCommand("1.2.3")
	cmd := findSubcommand(root, "apps", "wall", "submit")
	if cmd == nil {
		t.Fatal("expected apps wall submit command")
	}

	outputFlag := cmd.FlagSet.Lookup("output")
	if outputFlag == nil {
		t.Fatal("expected --output flag")
	}
	if got := outputFlag.DefValue; got != "json" {
		t.Fatalf("expected --output default json, got %q", got)
	}
}

func TestAppsWallSubmitRequiresConfirmUnlessDryRun(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"apps", "wall", "submit",
			"--app", "1234567890",
			"--platform", "iOS,macOS",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--confirm is required unless --dry-run is set") {
		t.Fatalf("expected confirm guidance in stderr, got %q", stderr)
	}
}

func TestAppsWallSubmitRejectsParentWallFlags(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"apps", "wall",
			"--output", "markdown",
			"submit",
			"--app", "1234567890",
			"--platform", "iOS",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "apps wall submit does not accept parent wall flags") {
		t.Fatalf("expected parent flag guidance in stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, "--output") {
		t.Fatalf("expected offending flag in stderr, got %q", stderr)
	}
}

func TestAppsShowcaseRemoved(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"apps", "showcase"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp for removed subcommand, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, `unknown subcommand "showcase"`) {
		t.Fatalf("expected unknown subcommand error, got %q", stderr)
	}
}

func TestAppsWallMarkdownColumnsExcludeIcon(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "wall.json")
	sourceJSON := `[
		{"app":"Alpha App","link":"https://example.com/alpha","creator":"Alpha Creator","platform":["iOS"]},
		{"app":"Beta Mac","link":"https://example.com/beta","creator":"Beta Creator","platform":["macOS"]}
	]`
	if err := os.WriteFile(sourcePath, []byte(sourceJSON), 0o600); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	t.Setenv("ASC_WALL_SOURCE", sourcePath)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"apps", "wall", "--output", "markdown"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "| App") || !strings.Contains(stdout, "| Link") || !strings.Contains(stdout, "| Creator") || !strings.Contains(stdout, "| Platform") {
		t.Fatalf("expected markdown columns App/Link/Creator/Platform, got %q", stdout)
	}
	if strings.Contains(stdout, "| Icon |") {
		t.Fatalf("did not expect icon column, got %q", stdout)
	}
}

func TestAppsWallCommunityUsesConfiguredSource(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "wall.json")
	sourceJSON := `[
		{"app":"Alpha App","link":"https://example.com/alpha","creator":"Alpha Creator","platform":["iOS"]},
		{"app":"Zeta App","link":"https://example.com/zeta","creator":"Zeta Creator","platform":["iOS"]},
		{"app":"Beta Mac","link":"https://example.com/beta","creator":"Beta Creator","platform":["macOS"]}
	]`
	if err := os.WriteFile(sourcePath, []byte(sourceJSON), 0o600); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	t.Setenv("ASC_WALL_SOURCE", sourcePath)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"apps", "wall",
			"--output", "json",
			"--include-platforms", "iOS",
			"--sort", "-name",
			"--limit", "1",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var out struct {
		Data []struct {
			Name        string   `json:"name"`
			Creator     string   `json:"creator"`
			AppStoreURL string   `json:"appStoreUrl"`
			Platform    []string `json:"platform"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("parse json output: %v\nstdout: %s", err, stdout)
	}
	if len(out.Data) != 1 {
		t.Fatalf("expected one filtered entry, got %d", len(out.Data))
	}
	if out.Data[0].Name != "Zeta App" {
		t.Fatalf("expected Zeta App after -name sort with limit 1, got %q", out.Data[0].Name)
	}
	if out.Data[0].Creator != "Zeta Creator" {
		t.Fatalf("expected creator Zeta Creator, got %q", out.Data[0].Creator)
	}
	if out.Data[0].AppStoreURL != "https://example.com/zeta" {
		t.Fatalf("expected zeta link, got %q", out.Data[0].AppStoreURL)
	}
	if len(out.Data[0].Platform) != 1 || out.Data[0].Platform[0] != "IOS" {
		t.Fatalf("expected normalized IOS platform, got %+v", out.Data[0].Platform)
	}
}

func TestAppsWallCommunityMissingSourceError(t *testing.T) {
	t.Setenv("ASC_WALL_SOURCE", filepath.Join(t.TempDir(), "missing-wall.json"))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"apps", "wall"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected command error, got nil")
	}
	if !strings.Contains(runErr.Error(), "apps wall: failed to read community wall source") {
		t.Fatalf("expected source read error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}
