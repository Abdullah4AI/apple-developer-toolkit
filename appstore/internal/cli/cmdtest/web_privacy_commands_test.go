package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestWebPrivacyPullRequiresApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"web", "privacy", "pull"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "--app is required") {
		t.Fatalf("expected missing --app message, got %q", stderr)
	}
}

func TestWebPrivacyCatalogRejectsPositionalArgs(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"web", "privacy", "catalog", "extra"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "does not accept positional arguments") {
		t.Fatalf("expected positional args usage message, got %q", stderr)
	}
}

func TestWebPrivacyPlanRequiresFile(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"web", "privacy", "plan", "--app", "123456789"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "--file is required") {
		t.Fatalf("expected missing --file message, got %q", stderr)
	}
}

func TestWebPrivacyApplyRequiresConfirmWhenAllowDeletesSet(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"web", "privacy", "apply",
			"--app", "123456789",
			"--file", "privacy.json",
			"--allow-deletes",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "--confirm is required when --allow-deletes is set") {
		t.Fatalf("expected missing --confirm message, got %q", stderr)
	}
}

func TestWebPrivacyPublishRequiresConfirm(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"web", "privacy", "publish",
			"--app", "123456789",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("expected missing --confirm message, got %q", stderr)
	}
}

func TestWebPrivacyApplyHelpDocumentsPartialFailureReceipt(t *testing.T) {
	apply := findSubcommand(RootCommand("1.2.3"), "web", "privacy", "apply")
	if apply == nil {
		t.Fatal("expected web privacy apply to be registered")
	}
	for _, want := range []string{
		"catalog tokens Apple",
		"applied, unknown, and not applied",
		"changed=false",
		"The exception is a delete a later create depends on",
	} {
		if !strings.Contains(apply.LongHelp, want) {
			t.Fatalf("web privacy apply help missing %q:\n%s", want, apply.LongHelp)
		}
	}
}

func TestWebPrivacyPlanHelpDocumentsStaleTokenDiagnostic(t *testing.T) {
	plan := findSubcommand(RootCommand("1.2.3"), "web", "privacy", "plan")
	if plan == nil {
		t.Fatal("expected web privacy plan to be registered")
	}
	if !strings.Contains(plan.LongHelp, "staleTokens") {
		t.Fatalf("web privacy plan help missing the stale-token diagnostic:\n%s", plan.LongHelp)
	}
}
