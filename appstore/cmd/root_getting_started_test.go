package cmd

import (
	"regexp"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/auth"
)

var ansiEscapePattern = regexp.MustCompile("\x1b\\[[0-9;]*m")

// wantGettingStartedInvocations pins the copy-paste invocations that teach the
// discovery loop on the first screen of `asc --help`.
var wantGettingStartedInvocations = []string{
	auth.LoginCommandExample,
	`asc search "upload a build" --output json`,
	"asc auth doctor",
	"asc apps list --paginate --output table",
	"asc status --app APP_ID",
}

func TestRootHelpDoesNotOverstateAuthDoctorNetworkValidation(t *testing.T) {
	block := gettingStartedBlock(t)
	if !strings.Contains(block, "Diagnose local auth configuration:\n    asc auth doctor") {
		t.Fatalf("getting-started auth step must describe local diagnosis accurately:\n%s", block)
	}
	lower := strings.ToLower(block)
	for _, unsupportedClaim := range []string{
		"validate network",
		"validates network",
		"network validation",
		"validate app store connect",
		"validates app store connect",
		"app store connect access",
	} {
		if strings.Contains(lower, unsupportedClaim) {
			t.Fatalf("getting-started auth step must not claim %q:\n%s", unsupportedClaim, block)
		}
	}
}

func TestRootHelpGettingStartedSamplesHaveNoInlineShellComments(t *testing.T) {
	for _, line := range strings.Split(gettingStartedBlock(t), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "asc ") && strings.Contains(trimmed, "#") {
			t.Fatalf("pasteable sample has a shell-specific inline comment: %q", trimmed)
		}
	}
}

func TestRootHelpRendersGettingStartedBlock(t *testing.T) {
	usage := stripANSIEscapes(rootUsageText(t))

	headerIndex := strings.Index(usage, "\nGETTING STARTED\n")
	if headerIndex < 0 {
		t.Fatalf("root help is missing a GETTING STARTED block:\n%s", usage)
	}

	usageIndex := strings.Index(usage, "\nUSAGE\n")
	if usageIndex < 0 || usageIndex > headerIndex {
		t.Fatalf("GETTING STARTED block must render after USAGE:\n%s", usage)
	}

	groupIndex := strings.Index(usage, "GETTING STARTED COMMANDS")
	if groupIndex < 0 || headerIndex > groupIndex {
		t.Fatalf("GETTING STARTED block must render before the command groups:\n%s", usage)
	}

	for _, invocation := range wantGettingStartedInvocations {
		if !strings.Contains(usage, "  "+invocation+" ") && !strings.Contains(usage, "  "+invocation+"\n") {
			t.Errorf("root help is missing getting-started invocation %q", invocation)
		}
	}
}

func TestRootHelpGettingStartedUsesBarePlaceholders(t *testing.T) {
	block := gettingStartedBlock(t)
	if strings.ContainsAny(block, "<>") {
		t.Errorf("getting-started block must use bare placeholders like APP_ID, not shell redirection characters:\n%s", block)
	}
}

// TestRootHelpGettingStartedInvocationsResolve mechanically walks every
// advertised invocation against the real command tree so the first screen can
// never advertise a command path or flag that does not exist.
func TestRootHelpGettingStartedInvocationsResolve(t *testing.T) {
	root := RootCommand("test")
	invocations := gettingStartedInvocations(gettingStartedBlock(t))
	if len(invocations) < 3 {
		t.Fatalf("expected at least 3 getting-started invocations, got %d", len(invocations))
	}

	for _, invocation := range invocations {
		t.Run(invocation, func(t *testing.T) {
			assertInvocationResolves(t, root, invocation)
		})
	}
}

func assertInvocationResolves(t *testing.T, root *ffcli.Command, invocation string) {
	t.Helper()

	tokens := splitInvocationTokens(invocation)
	if len(tokens) == 0 || tokens[0] != "asc" {
		t.Fatalf("invocation %q must start with asc", invocation)
	}

	bareWord := regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	current := root
	index := 1
	for index < len(tokens) && len(current.Subcommands) > 0 {
		token := tokens[index]
		if !bareWord.MatchString(token) {
			break
		}
		sub := findDirectSubcommand(current, token)
		if sub == nil {
			t.Fatalf("invocation %q references unknown command %q", invocation, strings.Join(tokens[:index+1], " "))
		}
		current = sub
		index++
	}

	if current == root {
		t.Fatalf("invocation %q does not resolve to a subcommand", invocation)
	}
	if current.Exec == nil {
		t.Fatalf("invocation %q resolves to %q, which is not runnable", invocation, current.Name)
	}

	for _, token := range tokens[index:] {
		if !strings.HasPrefix(token, "--") {
			continue
		}
		name := strings.TrimPrefix(token, "--")
		if equals := strings.Index(name, "="); equals >= 0 {
			name = name[:equals]
		}
		if flagLookup(current, name) == nil && flagLookup(root, name) == nil {
			t.Fatalf("invocation %q uses unknown flag --%s for %q", invocation, name, current.Name)
		}
	}
}

func flagLookup(cmd *ffcli.Command, name string) any {
	if cmd == nil || cmd.FlagSet == nil {
		return nil
	}
	if f := cmd.FlagSet.Lookup(name); f != nil {
		return f
	}
	return nil
}

func rootUsageText(t *testing.T) string {
	t.Helper()
	root := RootCommand("test")
	if root.UsageFunc == nil {
		t.Fatal("root command has no UsageFunc")
	}
	return root.UsageFunc(root)
}

// gettingStartedBlock returns the GETTING STARTED section of the rendered root
// help, without ANSI styling.
func gettingStartedBlock(t *testing.T) string {
	t.Helper()

	usage := stripANSIEscapes(rootUsageText(t))
	start := strings.Index(usage, "\nGETTING STARTED\n")
	if start < 0 {
		t.Fatalf("root help is missing a GETTING STARTED block:\n%s", usage)
	}
	rest := usage[start+1:]
	end := strings.Index(rest, "GETTING STARTED COMMANDS")
	if end < 0 {
		t.Fatalf("root help is missing the GETTING STARTED COMMANDS group:\n%s", usage)
	}
	return rest[:end]
}

// gettingStartedInvocations extracts the `asc ...` sample lines from the block.
func gettingStartedInvocations(block string) []string {
	invocations := make([]string, 0, 4)
	for _, line := range strings.Split(block, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "asc ") {
			continue
		}
		invocations = append(invocations, trimmed)
	}
	return invocations
}

// splitInvocationTokens splits a shell-style invocation, keeping double-quoted
// arguments as single tokens.
func splitInvocationTokens(invocation string) []string {
	tokens := make([]string, 0, 8)
	var current strings.Builder
	quoted := false
	flush := func() {
		if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}
	for _, r := range invocation {
		switch {
		case r == '"':
			quoted = !quoted
			current.WriteRune(r)
		case r == ' ' && !quoted:
			flush()
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return tokens
}

func stripANSIEscapes(s string) string {
	return ansiEscapePattern.ReplaceAllString(s, "")
}
