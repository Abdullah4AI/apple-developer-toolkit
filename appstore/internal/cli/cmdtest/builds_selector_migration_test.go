package cmdtest

import (
	"strings"
	"testing"

	rootcmd "github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
)

// assertRemovedFlagGuidance runs the CLI entrypoint and asserts the removed-flag
// usage path: exit code 2, empty stdout, and no deprecation copy.
func assertRemovedFlagGuidance(t *testing.T, args []string, flagName string) {
	t.Helper()

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run(args, "1.2.3")
	})

	if code != 2 {
		t.Fatalf("expected exit code 2 for removed flag %s, got %d (stderr %q)", flagName, code, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: `"+flagName+"` was removed in 5.0.0") {
		t.Fatalf("expected removed-flag guidance for %s, got %q", flagName, stderr)
	}
	if strings.Contains(stderr, "is deprecated") {
		t.Fatalf("removed flag %s must not emit deprecation guidance, got %q", flagName, stderr)
	}
}

func TestBuildsRemovedSelectorAliasesHaveGuidance(t *testing.T) {
	tests := []struct {
		name string
		args []string
		flag string
	}{
		{
			name: "wait build alias",
			args: []string{"builds", "wait", "--build", "BUILD_123"},
			flag: "--build",
		},
		{
			name: "wait newest alias",
			args: []string{"builds", "wait", "--app", "APP_123", "--newest"},
			flag: "--newest",
		},
		{
			name: "info build alias",
			args: []string{"builds", "info", "--build", "BUILD_123"},
			flag: "--build",
		},
		{
			name: "dsyms build alias",
			args: []string{"builds", "dsyms", "--build", "BUILD_123"},
			flag: "--build",
		},
		{
			name: "expire build alias",
			args: []string{"builds", "expire", "--build", "BUILD_123", "--confirm"},
			flag: "--build",
		},
		{
			name: "list app-id alias",
			args: []string{"builds", "list", "--app-id", "123456789"},
			flag: "--app-id",
		},
		{
			name: "app view id alias",
			args: []string{"builds", "app", "view", "--id", "BUILD_123"},
			flag: "--id",
		},
		{
			name: "pre-release-version view id alias",
			args: []string{"builds", "pre-release-version", "view", "--id", "BUILD_123"},
			flag: "--id",
		},
		{
			name: "beta-app-review-submission view id alias",
			args: []string{"builds", "beta-app-review-submission", "view", "--id", "BUILD_123"},
			flag: "--id",
		},
		{
			name: "app-encryption-declaration view id alias",
			args: []string{"builds", "app-encryption-declaration", "view", "--id", "BUILD_123"},
			flag: "--id",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertRemovedFlagGuidance(t, test.args, test.flag)
		})
	}
}

func TestBuildsSelectorHelpDoesNotMentionRemovedAliases(t *testing.T) {
	for _, path := range [][]string{
		{"builds", "wait"},
		{"builds", "info"},
		{"builds", "dsyms"},
		{"builds", "list"},
		{"builds", "test-notes", "view"},
	} {
		t.Run(strings.Join(path, " "), func(t *testing.T) {
			usage := usageForCommand(t, path...)
			for _, removed := range []string{"\n  --build ", "\n  --newest", "\n  --app-id", "\n  --id "} {
				if strings.Contains(usage, removed) {
					t.Fatalf("expected removed alias %q to be absent from help for %q, got %q", strings.TrimSpace(removed), strings.Join(path, " "), usage)
				}
			}
		})
	}
}
