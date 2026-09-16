package cmdtest

import (
	"io"
	"strings"
	"testing"
)

// buildIDOnlyCommands lists the TestFlight and build-selector commands whose
// hidden `--build` alias was removed in 5.0.0. Only `--build-id` remains.
var buildIDOnlyCommands = [][]string{
	{"testflight", "review", "submit"},
	{"testflight", "review", "submissions", "list"},
	{"testflight", "distribution", "view"},
	{"testflight", "notifications", "send"},
	{"testflight", "testers", "list"},
	{"testflight", "testers", "export"},
	{"testflight", "testers", "add-builds"},
	{"testflight", "testers", "remove-builds"},
	{"testflight", "config", "export"},
	{"testflight", "crashes", "list"},
	{"testflight", "feedback", "list"},
	{"builds", "info"},
	{"builds", "wait"},
	{"builds", "dsyms"},
	{"builds", "expire"},
	{"builds", "update"},
	{"builds", "add-groups"},
	{"builds", "remove-groups"},
	{"builds", "individual-testers", "list"},
	{"builds", "individual-testers", "add"},
	{"builds", "individual-testers", "remove"},
	{"builds", "metrics", "beta-usages"},
	{"builds", "icons", "list"},
	{"builds", "links", "view"},
	{"builds", "app", "view"},
	{"builds", "pre-release-version", "view"},
	{"builds", "beta-app-review-submission", "view"},
	{"builds", "build-beta-detail", "view"},
	{"builds", "app-encryption-declaration", "view"},
	{"builds", "test-notes", "list"},
	{"builds", "test-notes", "view"},
	{"builds", "test-notes", "create"},
	{"builds", "test-notes", "update"},
	{"builds", "test-notes", "delete"},
	{"publish", "testflight"},
	{"validate", "testflight"},
	{"release", "stage"},
	{"build-localizations", "list"},
	{"build-localizations", "create"},
	{"build-bundles", "list"},
	{"performance", "metrics", "view"},
	{"performance", "diagnostics", "list"},
	{"performance", "download"},
}

func TestBuildIDOnlyCommandsDoNotRegisterLegacyBuildAlias(t *testing.T) {
	root := RootCommand("1.2.3")

	for _, path := range buildIDOnlyCommands {
		t.Run(strings.Join(path, " "), func(t *testing.T) {
			command := findSubcommand(root, path...)
			if command == nil {
				t.Fatalf("command %q not found", strings.Join(path, " "))
			}
			if command.FlagSet.Lookup("build-id") == nil {
				t.Fatalf("canonical flag --build-id not registered on %q", strings.Join(path, " "))
			}
			if command.FlagSet.Lookup("build") != nil {
				t.Fatalf("removed alias --build must not be registered on %q", strings.Join(path, " "))
			}
			if command.FlagSet.Lookup("newest") != nil {
				t.Fatalf("removed alias --newest must not be registered on %q", strings.Join(path, " "))
			}
		})
	}
}

func TestTestFlightGroupsViewRejectsRemovedGroupIDAlias(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	command := findSubcommand(root, "testflight", "groups", "view")
	if command == nil {
		t.Fatal("testflight groups view not found")
	}
	if command.FlagSet.Lookup("id") == nil {
		t.Fatal("canonical flag --id not registered on testflight groups view")
	}
	if command.FlagSet.Lookup("group-id") != nil {
		t.Fatal("removed alias --group-id must not be registered on testflight groups view")
	}

	assertRemovedFlagGuidance(t, []string{"testflight", "groups", "view", "--group-id", "group-1"}, "--group-id")
}

func TestBuildIDOnlyCommandsRejectLegacyBuildAliasAsUnknownFlag(t *testing.T) {
	for _, path := range buildIDOnlyCommands {
		t.Run(strings.Join(path, " "), func(t *testing.T) {
			args := append(append([]string{}, path...), "--build", "BUILD_123")
			assertRemovedFlagGuidance(t, args, "--build")
		})
	}
}
