package cmdtest

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildsTestNotesUpdateRejectsInvalidLocale(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "test-notes", "update",
			"--build-id", "build-1",
			"--locale", "!!!bad!!!",
			"--whats-new", "test",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected invalid locale error")
	}
	if !strings.Contains(runErr.Error(), "invalid locale") {
		t.Fatalf("expected invalid locale error, got %v", runErr)
	}
}

func TestBuildsListRejectsInvalidLimit(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "list",
			"--app", "123456789",
			"--limit", "999",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected limit validation error")
	}
	if !strings.Contains(runErr.Error(), "--limit must be between 1 and 200") {
		t.Fatalf("expected limit range error, got %v", runErr)
	}
}
