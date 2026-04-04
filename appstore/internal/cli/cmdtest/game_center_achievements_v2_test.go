package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestGameCenterAchievementsV2ListValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "achievements", "v2", "list"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --app is required (or set ASC_APP_ID)") {
		t.Fatalf("expected missing app error, got %q", stderr)
	}
}

func TestGameCenterAchievementVersionsV2ListValidationErrors(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "achievements", "v2", "versions", "list"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --achievement-id is required") {
		t.Fatalf("expected missing achievement-id error, got %q", stderr)
	}
}

func TestGameCenterAchievementLocalizationsV2CreateValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing version-id",
			args:    []string{"game-center", "achievements", "v2", "localizations", "create", "--locale", "en-US", "--name", "Test", "--before-earned-description", "Before", "--after-earned-description", "After"},
			wantErr: "Error: --version-id is required",
		},
		{
			name:    "missing locale",
			args:    []string{"game-center", "achievements", "v2", "localizations", "create", "--version-id", "VER_ID", "--name", "Test", "--before-earned-description", "Before", "--after-earned-description", "After"},
			wantErr: "Error: --locale is required",
		},
		{
			name:    "missing name",
			args:    []string{"game-center", "achievements", "v2", "localizations", "create", "--version-id", "VER_ID", "--locale", "en-US", "--before-earned-description", "Before", "--after-earned-description", "After"},
			wantErr: "Error: --name is required",
		},
		{
			name:    "missing before",
			args:    []string{"game-center", "achievements", "v2", "localizations", "create", "--version-id", "VER_ID", "--locale", "en-US", "--name", "Test", "--after-earned-description", "After"},
			wantErr: "Error: --before-earned-description is required",
		},
		{
			name:    "missing after",
			args:    []string{"game-center", "achievements", "v2", "localizations", "create", "--version-id", "VER_ID", "--locale", "en-US", "--name", "Test", "--before-earned-description", "Before"},
			wantErr: "Error: --after-earned-description is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestGameCenterAchievementImagesV2ViewValidationErrors(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "achievements", "v2", "images", "view"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --id or --localization-id is required") {
		t.Fatalf("expected missing id/localization error, got %q", stderr)
	}
}

func TestGameCenterAchievementImagesV2DeleteValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing id",
			args:    []string{"game-center", "achievements", "v2", "images", "delete", "--confirm"},
			wantErr: "Error: --id is required",
		},
		{
			name:    "missing confirm",
			args:    []string{"game-center", "achievements", "v2", "images", "delete", "--id", "IMAGE_ID"},
			wantErr: "Error: --confirm is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}
