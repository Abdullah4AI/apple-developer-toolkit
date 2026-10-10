package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestBuildsLocalWorkflowRejectsUnsupportedInputsBeforeTools(t *testing.T) {
	for _, tc := range []struct {
		args       []string
		diagnostic string
	}{
		{[]string{"builds", "compile", "--package-path", "missing", "--product", "App", "--app-path", "App.app", "--platform", "macos"}, "--platform must be device or simulator"},
		{[]string{"builds", "package", "--app-path", "App.app", "--ipa-path", "App.ipa"}, "select --ad-hoc or both --identity and --provisioning-profile"},
		{[]string{"builds", "package", "--app-path", "App.app", "--ipa-path", "App.ipa", "--ad-hoc", "--identity", "key.p12"}, "--ad-hoc cannot be combined"},
	} {
		t.Run(tc.diagnostic, func(t *testing.T) {
			root := RootCommand("test")
			root.FlagSet.SetOutput(io.Discard)
			if err := root.Parse(tc.args); err != nil {
				t.Fatalf("parse: %v", err)
			}
			var runErr error
			stdout, stderr := captureOutput(t, func() { runErr = root.Run(context.Background()) })
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected usage error, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("unexpected receipt: %s", stdout)
			}
			// UsageError carries the diagnostic; the root runner prints it at the binary boundary.
			if !strings.Contains(stderr+runErr.Error(), tc.diagnostic) {
				t.Fatalf("missing diagnostic: %v %s", runErr, stderr)
			}
		})
	}
}
