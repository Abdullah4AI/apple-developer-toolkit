package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestRelease_ShowsHelp(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"release"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "release") {
		t.Fatalf("expected help to mention release command, got %q", stderr)
	}
}

func TestReleaseRunRemovedShowsCanonicalGuidance(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"release", "run",
			"--app", "APP_123",
			"--version", "1.2.3",
			"--build", "BUILD_123",
			"--metadata-dir", "./metadata/version/1.2.3",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "Error: `asc release run` was removed. Use `asc release stage` instead.") {
		t.Fatalf("expected removed command guidance, got %q", stderr)
	}
	if !strings.Contains(stderr, "asc release stage") {
		t.Fatalf("expected replacement guidance to mention asc release stage, got %q", stderr)
	}
}
