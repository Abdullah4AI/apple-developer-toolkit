package testflight

import (
	"context"
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestBetaTestersRemoveCommand_RequiresConfirm(t *testing.T) {
	isolateTestFlightAuthEnvForAddTests(t)

	cmd := BetaTestersRemoveCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "123456789",
		"--email", "tester@example.com",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("remove without --confirm should fail validation, got %v", err)
	}
	if !strings.Contains(err.Error(), "--confirm") {
		t.Fatalf("usage error should name --confirm, got %q", err.Error())
	}
}

func TestBetaTestersRemoveCommand_ConfirmPassesValidation(t *testing.T) {
	isolateTestFlightAuthEnvForAddTests(t)

	cmd := BetaTestersRemoveCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "123456789",
		"--email", "tester@example.com",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if errors.Is(err, flag.ErrHelp) {
		t.Fatalf("remove with --confirm should pass validation, got %v", err)
	}
}

func TestBetaTestersRemoveWaitFlagsAreRegistered(t *testing.T) {
	cmd := BetaTestersRemoveCommand()
	for _, name := range []string{"wait", "poll-interval", "timeout"} {
		t.Run(name, func(t *testing.T) {
			flagValue := cmd.FlagSet.Lookup(name)
			if flagValue == nil {
				t.Fatalf("--%s is not registered", name)
			}
		})
	}
}
