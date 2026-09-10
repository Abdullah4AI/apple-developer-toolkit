package cmdtest

import (
	"flag"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// TestCommandHelpUsesPlainDescriptions checks the complete registered command
// tree, including flag help, so descriptions stay consistent across surfaces.
func TestCommandHelpUsesPlainDescriptions(t *testing.T) {
	root := RootCommand("1.2.3")
	var visit func(*ffcli.Command, string)
	visit = func(command *ffcli.Command, path string) {
		check := func(kind, help string) {
			if strings.Contains(strings.ToLower(help), "experimental") {
				t.Errorf("%s %s contains a lifecycle label: %q", path, kind, help)
			}
		}
		check("short help", command.ShortHelp)
		check("long help", command.LongHelp)
		if command.FlagSet != nil {
			command.FlagSet.VisitAll(func(f *flag.Flag) { check("--"+f.Name, f.Usage) })
		}
		for _, child := range command.Subcommands {
			visit(child, path+" "+child.Name)
		}
	}
	visit(root, root.Name)
}

func TestWebCommandsDoNotHaveEndpointWarningLabels(t *testing.T) {
	root := RootCommand("1.2.3")

	webCmd := findSubcommand(root, "web")
	if webCmd == nil {
		t.Fatal("command [web] not found")
	}
	assertCommandTreeDoesNotMentionEndpointWarnings(t, webCmd, []string{"web"})
}

func assertCommandTreeDoesNotMentionEndpointWarnings(t *testing.T, cmd *ffcli.Command, path []string) {
	t.Helper()

	assertCommandDoesNotMentionEndpointWarnings(t, cmd, path)

	for _, sub := range cmd.Subcommands {
		assertCommandTreeDoesNotMentionEndpointWarnings(t, sub, append(path, sub.Name))
	}
}

func assertCommandDoesNotMentionEndpointWarnings(t *testing.T, cmd *ffcli.Command, path []string) {
	t.Helper()

	if cmd == nil {
		t.Errorf("command %v not found", path)
		return
	}
	help := strings.ToLower(cmd.ShortHelp + "\n" + cmd.LongHelp)
	for _, token := range []string{
		"unofficial",
		"discouraged",
		"private endpoint",
		"private web",
		"not sanctioned",
		"at your own risk",
		"account restrictions",
		"production-critical",
		"break without notice",
	} {
		if strings.Contains(help, token) {
			t.Errorf("command %v: expected help not to mention %q, got %q", path, token, cmd.LongHelp)
		}
	}
}
