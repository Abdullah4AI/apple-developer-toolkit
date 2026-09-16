package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"

	rootcmd "github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
	webcmd "github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/web"
	webcore "github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/web"
)

func TestRootUsageIncludesWebSessionGroup(t *testing.T) {
	root := RootCommand("1.2.3")
	usage := root.UsageFunc(root)

	if !strings.Contains(usage, "WEB SESSION COMMANDS") {
		t.Fatalf("expected web session group in root usage, got %q", usage)
	}
	if !strings.Contains(usage, "  web:") {
		t.Fatalf("expected web command in root usage, got %q", usage)
	}
}

func TestWebCommandUsesProductionHelpContract(t *testing.T) {
	root := RootCommand("1.2.3")
	webCmd := findSubcommand(root, "web")
	if webCmd == nil {
		t.Fatal("expected web command")
		return
	}

	usage := webCmd.UsageFunc(webCmd)
	if !strings.Contains(usage, "WEB SESSION WORKFLOWS") {
		t.Fatalf("expected web-session help heading, got %q", usage)
	}
	lowerUsage := strings.ToLower(usage)
	for _, token := range []string{"unofficial", "discouraged", "private", "risk"} {
		if !strings.Contains(lowerUsage, token) {
			continue
		}
		t.Fatalf("expected %q not to appear in web usage, got %q", token, usage)
	}
}

func TestWebAppsCreateSubcommandIsRegistered(t *testing.T) {
	root := RootCommand("1.2.3")
	if sub := findSubcommand(root, "web", "apps", "create"); sub == nil {
		t.Fatalf("expected web apps create to be registered")
	}
}

func TestWebRemovedAppsListSubcommandIsRegistered(t *testing.T) {
	root := RootCommand("1.2.3")
	if sub := findSubcommand(root, "web", "removed-apps", "list"); sub == nil {
		t.Fatalf("expected web removed-apps list to be registered")
	}
}

func TestWebAppsDeleteSubcommandIsRegistered(t *testing.T) {
	root := RootCommand("1.2.3")
	if sub := findSubcommand(root, "web", "apps", "delete"); sub == nil {
		t.Fatalf("expected web apps delete to be registered")
	}
}

func TestWebAppsMedicalDeviceSetSubcommandIsRegistered(t *testing.T) {
	root := RootCommand("1.2.3")
	if sub := findSubcommand(root, "web", "apps", "medical-device", "set"); sub == nil {
		t.Fatalf("expected web apps medical-device set to be registered")
	}
}

func TestWebAppsMedicalDeviceViewSubcommandIsRegistered(t *testing.T) {
	root := RootCommand("1.2.3")
	if sub := findSubcommand(root, "web", "apps", "medical-device", "view"); sub == nil {
		t.Fatalf("expected web apps medical-device view to be registered")
	}
}

func TestWebAppsDeclarationsListSubcommandIsRegistered(t *testing.T) {
	root := RootCommand("1.2.3")
	if sub := findSubcommand(root, "web", "apps", "declarations", "list"); sub == nil {
		t.Fatalf("expected web apps declarations list to be registered")
	}
}

func TestWebBundleIDCapabilitiesSyncAppClipSubcommandIsRegistered(t *testing.T) {
	root := RootCommand("1.2.3")
	sub := findSubcommand(root, "web", "bundle-ids", "capabilities", "sync-app-clip")
	if sub == nil {
		t.Fatalf("expected web bundle-ids capabilities sync-app-clip to be registered")
	}
	for _, flagName := range []string{"bundle-id", "parent-bundle-id", "capability", "settings-json", "confirm", "apple-id", "output"} {
		if sub.FlagSet.Lookup(flagName) == nil {
			t.Fatalf("expected --%s flag", flagName)
		}
	}
	if !strings.Contains(sub.ShortUsage, "--confirm") {
		t.Fatalf("expected --confirm in short usage, got %q", sub.ShortUsage)
	}
}

func TestWebBundleIDCapabilitiesSyncAppClipMissingConfirmFailsBeforeSession(t *testing.T) {
	restore := webcmd.SetResolveWebSession(func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
		t.Fatal("web session must not be resolved when --confirm is missing")
		return nil, "", nil
	})
	defer restore()
	restoreSync := webcmd.SetSyncAppClipBundleIDCapability(func(context.Context, *webcore.Client, webcore.AppClipBundleIDCapabilitySyncRequest) (*webcore.AppClipBundleIDCapabilitySyncResult, error) {
		t.Fatal("sync must not run when --confirm is missing")
		return nil, nil
	})
	defer restoreSync()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"web", "bundle-ids", "capabilities", "sync-app-clip", "--bundle-id", "clip-1", "--parent-bundle-id", "parent-1", "--capability", "PUSH_NOTIFICATIONS"}); err != nil {
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
	if !strings.Contains(stderr, "Warning: web bundle-ids capabilities sync-app-clip now requires --confirm") {
		t.Fatalf("expected migration warning, got %q", stderr)
	}
	if !strings.Contains(stderr, "Error: --confirm is required") {
		t.Fatalf("expected missing --confirm error, got %q", stderr)
	}
}

func TestWebBundleIDCapabilitiesEnableSubcommandIsRegistered(t *testing.T) {
	root := RootCommand("1.2.3")
	sub := findSubcommand(root, "web", "bundle-ids", "capabilities", "enable")
	if sub == nil {
		t.Fatalf("expected web bundle-ids capabilities enable to be registered")
	}
	for _, flagName := range []string{"bundle-id", "capability", "confirm", "apple-id", "output"} {
		if sub.FlagSet.Lookup(flagName) == nil {
			t.Fatalf("expected --%s flag", flagName)
		}
	}
}

func TestWebSandboxCreateSubcommandIsRegistered(t *testing.T) {
	root := RootCommand("1.2.3")
	if sub := findSubcommand(root, "web", "sandbox", "create"); sub == nil {
		t.Fatalf("expected web sandbox create to be registered")
	}
}

func TestWebAuthCapabilitiesSubcommandIsRegistered(t *testing.T) {
	root := RootCommand("1.2.3")
	if sub := findSubcommand(root, "web", "auth", "capabilities"); sub == nil {
		t.Fatalf("expected web auth capabilities to be registered")
	}
}

func TestWebXcodeCloudWorkflowsCreateSubcommandIsRegistered(t *testing.T) {
	root := RootCommand("1.2.3")
	if sub := findSubcommand(root, "web", "xcode-cloud", "workflows", "create"); sub == nil {
		t.Fatalf("expected web xcode-cloud workflows create to be registered")
	}
}

func TestWebXcodeCloudWorkflowsListSubcommandIsRegistered(t *testing.T) {
	root := RootCommand("1.2.3")
	sub := findSubcommand(root, "web", "xcode-cloud", "workflows", "list")
	if sub == nil {
		t.Fatalf("expected web xcode-cloud workflows list to be registered")
	}
	if sub.FlagSet.Lookup("product-id") == nil {
		t.Fatal("expected --product-id flag on web xcode-cloud workflows list")
	}
	if sub.FlagSet.Lookup("paginate") != nil {
		t.Fatal("did not expect --paginate on web xcode-cloud workflows list; the CI client does not page")
	}
}

func TestWebXcodeCloudWorkflowsListMissingRequiredFlags(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"web", "xcode-cloud", "workflows", "list"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "Error: --product-id is required") {
		t.Fatalf("expected missing --product-id error, got %q", stderr)
	}
}

func TestWebXcodeCloudWorkflowsListRejectsPositionalArguments(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"web", "xcode-cloud", "workflows", "list", "--product-id", "prod-1", "extra"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "web xcode-cloud workflows list does not accept positional arguments") {
		t.Fatalf("expected positional-args error, got %q", stderr)
	}
}

func TestWebAppsCreateMissingRequiredFlags(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"web", "apps", "create", "--name", "My App"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "Error: missing required flags: --bundle-id, --sku") {
		t.Fatalf("expected aggregated missing-flags error, got %q", stderr)
	}
}

func TestWebAppsCreateExposesPasswordCompatibilityFlag(t *testing.T) {
	root := RootCommand("1.2.3")
	cmd := findSubcommand(root, "web", "apps", "create")
	if cmd == nil {
		t.Fatal("expected web apps create command")
		return
	}
	if cmd.FlagSet.Lookup("password") == nil {
		t.Fatal("expected temporary password compatibility flag on web apps create")
	}
}

func TestWebSandboxCreateMissingRequiredFlags(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"web", "sandbox", "create"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "Error: --first-name is required") {
		t.Fatalf("expected missing --first-name error, got %q", stderr)
	}
}

func TestWebXcodeCloudWorkflowsCreateMissingRequiredFlags(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"web", "xcode-cloud", "workflows", "create"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "Error: --product-id is required") {
		t.Fatalf("expected missing --product-id error, got %q", stderr)
	}
}

func TestWebAuthLoginOmitsPlaintextPasswordAndRemovedTwoFactorCodeFlags(t *testing.T) {
	root := RootCommand("1.2.3")
	cmd := findSubcommand(root, "web", "auth", "login")
	if cmd == nil {
		t.Fatal("expected web auth login command")
		return
	}
	if got, want := cmd.ShortUsage, "asc web auth login [--apple-id EMAIL] [--public-provider-id TEAM_ID]"; got != want {
		t.Fatalf("ShortUsage = %q, want %q", got, want)
	}
	if cmd.FlagSet.Lookup("password") != nil {
		t.Fatal("did not expect --password flag on web auth login")
	}
	if cmd.FlagSet.Lookup("password-stdin") != nil {
		t.Fatal("did not expect --password-stdin flag on web auth login")
	}
	if cmd.FlagSet.Lookup("two-factor-code") != nil {
		t.Fatal("removed --two-factor-code alias is still registered on web auth login")
	}
	if cmd.FlagSet.Lookup("two-factor-code-command") == nil {
		t.Fatal("expected --two-factor-code-command flag on web auth login")
	}
	if strings.Contains(cmd.LongHelp, "--two-factor-code ") || strings.Contains(cmd.LongHelp, "--two-factor-code\n") || strings.Contains(cmd.LongHelp, "deprecated") {
		t.Fatalf("web auth login help still documents the removed --two-factor-code alias: %q", cmd.LongHelp)
	}
	for _, phrase := range []string{
		"Phone-code fallback (including SMS):",
		"interactive: if Apple offers a registered phone fallback",
		"enter an incorrect trusted-device code once",
		"Apple then delivers a phone verification code and asc prompts again",
		"automated: asc reruns the configured 2FA code command after phone fallback",
	} {
		if !strings.Contains(cmd.LongHelp, phrase) {
			t.Fatalf("expected %q in web auth login help, got %q", phrase, cmd.LongHelp)
		}
	}
}

func TestWebCommandsOmitRemovedTwoFactorCodeAlias(t *testing.T) {
	for _, path := range [][]string{
		{"web", "apps", "create"},
		{"web", "sandbox", "create"},
	} {
		t.Run(strings.Join(path, " "), func(t *testing.T) {
			root := RootCommand("1.2.3")
			cmd := findSubcommand(root, path...)
			if cmd == nil {
				t.Fatalf("expected %s command", strings.Join(path, " "))
				return
			}
			if cmd.FlagSet.Lookup("two-factor-code") != nil {
				t.Fatalf("removed --two-factor-code alias is still registered on %s", strings.Join(path, " "))
			}
			if cmd.FlagSet.Lookup("two-factor-code-command") == nil {
				t.Fatalf("expected --two-factor-code-command flag on %s", strings.Join(path, " "))
			}
			if strings.Contains(cmd.LongHelp, "--two-factor-code ") || strings.Contains(cmd.LongHelp, "deprecated compatibility alias") {
				t.Fatalf("%s help still documents the removed --two-factor-code alias: %q", strings.Join(path, " "), cmd.LongHelp)
			}
		})
	}
}

func TestWebAuthLoginRejectsRemovedTwoFactorCodeFlagAsUnknown(t *testing.T) {
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"web", "auth", "login", "--apple-id", "user@example.com", "--two-factor-code", "123456"}, "1.2.3")
	})
	if code != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, rootcmd.ExitUsage, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "`--two-factor-code` was removed in 5.0.0") {
		t.Fatalf("stderr = %q, want removed-flag diagnostic", stderr)
	}
	if !strings.Contains(stderr, "--two-factor-code-command") {
		t.Fatalf("stderr = %q, want --two-factor-code-command suggestion", stderr)
	}
	if strings.Contains(stderr, "deprecated") {
		t.Fatalf("stderr still carries deprecation wording: %q", stderr)
	}
}
