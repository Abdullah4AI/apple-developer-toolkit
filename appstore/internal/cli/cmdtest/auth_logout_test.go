package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
	authsvc "github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/auth"
	authcmd "github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/auth"
)

type authLogoutCalls struct {
	names   []string
	all     int
	options []authsvc.RemoveOptions
}

func stubAuthLogoutRemovers(t *testing.T) *authLogoutCalls {
	t.Helper()
	// The retained-global-config warning reads the global config, so keep it
	// away from the developer's home directory.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	calls := &authLogoutCalls{}
	restore := authcmd.SetLogoutCredentialRemovers(
		func(name string, opts authsvc.RemoveOptions) error {
			calls.names = append(calls.names, name)
			calls.options = append(calls.options, opts)
			return nil
		},
		func(opts authsvc.RemoveOptions) error {
			calls.all++
			calls.options = append(calls.options, opts)
			return nil
		},
	)
	t.Cleanup(restore)
	return calls
}

func TestAuthLogoutConfirmRemovesNamedCredentialWithoutWarning(t *testing.T) {
	calls := stubAuthLogoutRemovers(t)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"auth", "logout", "--name", "demo", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stdout != "Successfully removed stored credential 'demo'\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no warning with --confirm, got %q", stderr)
	}
	if len(calls.names) != 1 || calls.names[0] != "demo" || calls.all != 0 {
		t.Fatalf("unexpected removal calls: %+v", calls)
	}
}

func TestAuthLogoutConfirmRemovesAllCredentialsWithoutWarning(t *testing.T) {
	calls := stubAuthLogoutRemovers(t)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"auth", "logout", "--all", "--confirm"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stdout != "Successfully removed stored credentials\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no warning with --confirm, got %q", stderr)
	}
	if len(calls.names) != 0 || calls.all != 1 {
		t.Fatalf("unexpected removal calls: %+v", calls)
	}
}

func TestAuthLogoutWithoutConfirmIsUsageErrorAndRemovesNothing(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
	}{
		{name: "implicit all", args: []string{"auth", "logout"}},
		{name: "explicit all", args: []string{"auth", "logout", "--all"}},
		{name: "named", args: []string{"auth", "logout", "--name", "demo"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := stubAuthLogoutRemovers(t)

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(tt.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("run error = %v, want flag.ErrHelp", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, "--confirm is required to remove stored credentials") {
				t.Fatalf("expected --confirm usage error, got %q", stderr)
			}
			if strings.Contains(stderr, "Warning") || strings.Contains(stderr, "5.0.0") {
				t.Fatalf("stderr still carries deprecation wording: %q", stderr)
			}
			if len(calls.names) != 0 || calls.all != 0 {
				t.Fatalf("expected no removal without --confirm, got %+v", calls)
			}
		})
	}
}

func TestAuthLogoutWithoutConfirmExitsWithUsageCode(t *testing.T) {
	calls := stubAuthLogoutRemovers(t)

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"auth", "logout"}, "1.2.3")
	})
	if code != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, rootcmd.ExitUsage, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--confirm is required to remove stored credentials") {
		t.Fatalf("expected --confirm usage error, got %q", stderr)
	}
	if len(calls.names) != 0 || calls.all != 0 {
		t.Fatalf("expected no removal without --confirm, got %+v", calls)
	}
}

func TestAuthLogoutRejectsExplicitFalseConfirmBeforeRemoval(t *testing.T) {
	calls := stubAuthLogoutRemovers(t)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"auth", "logout", "--all", "--confirm=false"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("run error = %v, want flag.ErrHelp", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--confirm must be true when specified") {
		t.Fatalf("expected explicit-false error, got %q", stderr)
	}
	if len(calls.names) != 0 || calls.all != 0 {
		t.Fatalf("expected no removal, got %+v", calls)
	}
}

func TestAuthLogoutRejectsPositionalArgsBeforeRemoval(t *testing.T) {
	calls := stubAuthLogoutRemovers(t)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"auth", "logout", "--all", "--confirm", "unexpected"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("run error = %v, want flag.ErrHelp", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "unexpected argument(s): unexpected") {
		t.Fatalf("expected positional-argument error, got %q", stderr)
	}
	if len(calls.names) != 0 || calls.all != 0 {
		t.Fatalf("expected no removal, got %+v", calls)
	}
}
