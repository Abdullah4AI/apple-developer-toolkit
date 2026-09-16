package cmdtest

import (
	"errors"
	"strings"
	"testing"

	rootcmd "github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

// TestGameCenterRelationshipReplacementRequiresConfirm locks the 5.0.0
// contract: replacing Game Center group or leaderboard-set relationships
// without --confirm is a usage error before authentication or HTTP, not a
// warning that continues.
func TestGameCenterRelationshipReplacementRequiresConfirm(t *testing.T) {
	setupUsageExitCodeEnv(t)

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "groups achievements set",
			args: []string{"game-center", "groups", "achievements", "set", "--group-id", "group-1", "--ids", "achievement-1"},
		},
		{
			name: "groups leaderboards set",
			args: []string{"game-center", "groups", "leaderboards", "set", "--group-id", "group-1", "--ids", "leaderboard-1"},
		},
		{
			name: "leaderboard-sets members set",
			args: []string{"game-center", "leaderboard-sets", "members", "set", "--set-id", "set-1", "--leaderboard-ids", "leaderboard-1"},
		},
		{
			name: "leaderboard-sets v2 members set",
			args: []string{"game-center", "leaderboard-sets", "v2", "members", "set", "--set-id", "set-1", "--leaderboard-ids", "leaderboard-1"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factoryCalled := false
			restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				factoryCalled = true
				return nil, errors.New("poison client factory called")
			})
			t.Cleanup(restore)

			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(test.args, "5.0.0")
			})

			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			assertUsageDiagnosticFirstLine(t, stderr, "--confirm is required")
			if strings.Contains(stderr, "Warning:") {
				t.Fatalf("stderr = %q, must not carry the retired compatibility warning", stderr)
			}
			if factoryCalled {
				t.Fatal("client factory called before --confirm validation")
			}
		})
	}
}

// TestGameCenterRemovedFlagsAreUnknown locks the 5.0.0 removal of flags that
// 4.x kept registered only to return migration guidance: the pagination flags
// on the singleton `details list` and --challenge-enabled on `details
// create|update`. None of them is registered any more, so the failure carries
// the migration step and makes no HTTP request.
func TestGameCenterRemovedFlagsAreUnknown(t *testing.T) {
	setupUsageExitCodeEnv(t)

	tests := []struct {
		name     string
		command  string
		args     []string
		flag     string
		guidance string
	}{
		{name: "details list --limit", command: "asc game-center details list", args: []string{"game-center", "details", "list", "--app", "app-1", "--limit", "5"}, flag: "--limit", guidance: "each app has a single Game Center detail"},
		{name: "details list --next", command: "asc game-center details list", args: []string{"game-center", "details", "list", "--app", "app-1", "--next", "https://api.appstoreconnect.apple.com/v1/gameCenterDetails?cursor=x"}, flag: "--next", guidance: "each app has a single Game Center detail"},
		{name: "details list --paginate", command: "asc game-center details list", args: []string{"game-center", "details", "list", "--app", "app-1", "--paginate"}, flag: "--paginate", guidance: "each app has a single Game Center detail"},
		{name: "details create --challenge-enabled", command: "asc game-center details create", args: []string{"game-center", "details", "create", "--app", "app-1", "--challenge-enabled", "true"}, flag: "--challenge-enabled", guidance: "App Store Connect no longer accepts `challengeEnabled`"},
		{name: "details update --challenge-enabled", command: "asc game-center details update", args: []string{"game-center", "details", "update", "--id", "detail-1", "--challenge-enabled", "true"}, flag: "--challenge-enabled", guidance: "App Store Connect no longer accepts `challengeEnabled`"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factoryCalled := false
			restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				factoryCalled = true
				return nil, errors.New("poison client factory called")
			})
			t.Cleanup(restore)

			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(test.args, "5.0.0")
			})

			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			want := "Error: `" + test.flag + "` was removed in 5.0.0; " + test.guidance + " (see migrate-to-5-0)\nFor help:\n  " + test.command + " --help\n"
			if stderr != want {
				t.Fatalf("stderr = %q, want removed-flag guidance %q", stderr, want)
			}
			if factoryCalled {
				t.Fatalf("client factory called for removed %s", test.flag)
			}
		})
	}
}

// TestGameCenterRemovedFlagsAreUnregistered guards the flag sets directly so a
// re-registration of a removed flag fails even if the parse path changes.
func TestGameCenterRemovedFlagsAreUnregistered(t *testing.T) {
	root := RootCommand("5.0.0")

	tests := []struct {
		path  []string
		flags []string
	}{
		{path: []string{"game-center", "details", "list"}, flags: []string{"limit", "next", "paginate"}},
		{path: []string{"game-center", "details", "create"}, flags: []string{"challenge-enabled"}},
		{path: []string{"game-center", "details", "update"}, flags: []string{"challenge-enabled"}},
	}

	for _, test := range tests {
		command := findSubcommand(root, test.path...)
		if command == nil {
			t.Fatalf("command %q not found", strings.Join(test.path, " "))
		}
		for _, name := range test.flags {
			if command.FlagSet.Lookup(name) != nil {
				t.Fatalf("removed flag --%s is still registered on %q", name, strings.Join(test.path, " "))
			}
		}
		usage := command.UsageFunc(command)
		if strings.Contains(usage, "Deprecated") || strings.Contains(usage, "deprecation window") {
			t.Fatalf("%q help still carries retired deprecation text:\n%s", strings.Join(test.path, " "), usage)
		}
	}
}

// TestGameCenterGroupChallengesSetIsRemoved locks the 5.0.0 removal of the
// unsupported `groups challenges set` stub. App Store Connect exposes group
// challenge relationships read-only, so the verb is gone rather than a
// registered command that always failed.
func TestGameCenterGroupChallengesSetIsRemoved(t *testing.T) {
	setupUsageExitCodeEnv(t)

	root := RootCommand("5.0.0")
	if command := findSubcommand(root, "game-center", "groups", "challenges", "set"); command != nil {
		t.Fatal("removed command `asc game-center groups challenges set` is still registered")
	}
	challenges := findSubcommand(root, "game-center", "groups", "challenges")
	if challenges == nil || findSubcommand(challenges, "list") == nil {
		t.Fatal("`asc game-center groups challenges list` must stay registered")
	}
	if strings.Contains(challenges.LongHelp, "challenges set") {
		t.Fatalf("challenges group help still advertises the removed setter: %q", challenges.LongHelp)
	}
	groups := findSubcommand(root, "game-center", "groups")
	if strings.Contains(groups.LongHelp, "challenges set") {
		t.Fatalf("groups help still advertises the removed setter: %q", groups.LongHelp)
	}

	factoryCalled := false
	restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		factoryCalled = true
		return nil, errors.New("poison client factory called")
	})
	t.Cleanup(restore)

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"game-center", "groups", "challenges", "set", "--group-id", "group-1", "--ids", "challenge-1"}, "5.0.0")
	})

	if code != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.HasPrefix(stderr, "Error: unknown command `asc game-center groups challenges set`\n") {
		t.Fatalf("stderr = %q, want generic unknown-command failure", stderr)
	}
	if strings.Contains(strings.ToLower(stderr), "deprecated") {
		t.Fatalf("stderr = %q, must not carry retired deprecation guidance", stderr)
	}
	if factoryCalled {
		t.Fatal("client factory called for removed command")
	}
}
