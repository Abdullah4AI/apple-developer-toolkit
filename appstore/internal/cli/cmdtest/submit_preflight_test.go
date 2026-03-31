package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestSubmitPreflightCommandExists(t *testing.T) {
	root := RootCommand("1.2.3")
	cmd := findSubcommand(root, "submit", "preflight")
	if cmd == nil {
		t.Fatal("expected submit preflight command")
	}
	if !strings.HasPrefix(cmd.ShortHelp, "DEPRECATED:") {
		t.Fatalf("expected deprecated short help, got %q", cmd.ShortHelp)
	}

	outputFlag := cmd.FlagSet.Lookup("output")
	if outputFlag == nil {
		t.Fatal("expected --output flag")
	}
	if got := outputFlag.DefValue; got != "json" {
		t.Fatalf("expected --output default json, got %q", got)
	}
}

func TestSubmitPreflightRejectsUnsupportedOutput(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"submit", "preflight", "--app", "123", "--version", "1.0", "--output", "table"}); err != nil {
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
	if !strings.Contains(stderr, "unsupported format: table") {
		t.Fatalf("expected unsupported format message, got %q", stderr)
	}
}

func TestSubmitPreflightWarnsAboutCanonicalValidateCommand(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"submit", "preflight", "--app", "123", "--version", "1.0", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected readiness lookup failure in test environment")
	}
	requireStderrContainsWarning(t, stderr, "Warning: `asc submit preflight` is deprecated. Use `asc validate`.")
}
