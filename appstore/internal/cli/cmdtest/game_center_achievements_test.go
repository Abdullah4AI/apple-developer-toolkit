package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestGameCenterAchievementsListValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "achievements", "list"}); err != nil {
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
}

func TestGameCenterAchievementsListNoDetailReturnsEmptyList(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	expectedURL := "https://api.appstoreconnect.apple.com/v1/apps/APP_ID/gameCenterDetail"

	callCount := &lockedCounter{}
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if callCount.Inc() > 1 {
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
		}
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.String() != expectedURL {
			t.Fatalf("expected URL %s, got %s", expectedURL, req.URL.String())
		}

		body := `{"data":{"type":"gameCenterDetails","id":"","attributes":{}},"links":{"self":"` + expectedURL + `"}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "achievements", "list", "--app", "APP_ID"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stderr, "Warning: no Game Center detail exists for this app") {
		t.Fatalf("expected warning in stderr, got %q", stderr)
	}

	var resp struct {
		Data []any `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nstdout: %q", err, stdout)
	}
	if len(resp.Data) != 0 {
		t.Fatalf("expected empty data array, got %d items", len(resp.Data))
	}
}

func TestGameCenterAchievementsGetValidationErrors(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "achievements", "get"}); err != nil {
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
}

func TestGameCenterAchievementsCreateValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing app",
			args: []string{"game-center", "achievements", "create", "--reference-name", "Test", "--vendor-id", "com.test", "--points", "10"},
		},
		{
			name: "app and group",
			args: []string{"game-center", "achievements", "create", "--app", "APP_ID", "--group-id", "GROUP_ID", "--reference-name", "Test", "--vendor-id", "com.test", "--points", "10"},
		},
		{
			name: "group vendor prefix",
			args: []string{"game-center", "achievements", "create", "--group-id", "GROUP_ID", "--reference-name", "Test", "--vendor-id", "com.test", "--points", "10"},
		},
		{
			name: "missing reference-name",
			args: []string{"game-center", "achievements", "create", "--app", "APP_ID", "--vendor-id", "com.test", "--points", "10"},
		},
		{
			name: "missing vendor-id",
			args: []string{"game-center", "achievements", "create", "--app", "APP_ID", "--reference-name", "Test", "--points", "10"},
		},
		{
			name: "missing points",
			args: []string{"game-center", "achievements", "create", "--app", "APP_ID", "--reference-name", "Test", "--vendor-id", "com.test"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
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
		})
	}
}

func TestGameCenterAchievementsSubmitValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing vendor-id",
			args: []string{"game-center", "achievements", "submit", "--percentage", "100", "--bundle-id", "BUNDLE_ID", "--scoped-player-id", "PLAYER_ID"},
		},
		{
			name: "missing percentage",
			args: []string{"game-center", "achievements", "submit", "--vendor-id", "com.example.achievement", "--bundle-id", "BUNDLE_ID", "--scoped-player-id", "PLAYER_ID"},
		},
		{
			name: "missing bundle-id",
			args: []string{"game-center", "achievements", "submit", "--vendor-id", "com.example.achievement", "--percentage", "100", "--scoped-player-id", "PLAYER_ID"},
		},
		{
			name: "missing scoped-player-id",
			args: []string{"game-center", "achievements", "submit", "--vendor-id", "com.example.achievement", "--percentage", "100", "--bundle-id", "BUNDLE_ID"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
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
		})
	}
}

func TestGameCenterAchievementsUpdateValidationErrors(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "achievements", "update"}); err != nil {
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
}

func TestGameCenterAchievementsDeleteValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing id",
			args: []string{"game-center", "achievements", "delete", "--confirm"},
		},
		{
			name: "missing confirm",
			args: []string{"game-center", "achievements", "delete", "--id", "ACH_ID"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
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
		})
	}
}

func TestGameCenterAchievementLocalizationsListValidationErrors(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "achievements", "localizations", "list"}); err != nil {
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
}

func TestGameCenterAchievementLocalizationsCreateValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing achievement-id",
			args: []string{"game-center", "achievements", "localizations", "create", "--locale", "en-US", "--name", "Test", "--before-earned-description", "Before", "--after-earned-description", "After"},
		},
		{
			name: "missing locale",
			args: []string{"game-center", "achievements", "localizations", "create", "--achievement-id", "ACH_ID", "--name", "Test", "--before-earned-description", "Before", "--after-earned-description", "After"},
		},
		{
			name: "missing name",
			args: []string{"game-center", "achievements", "localizations", "create", "--achievement-id", "ACH_ID", "--locale", "en-US", "--before-earned-description", "Before", "--after-earned-description", "After"},
		},
		{
			name: "missing before-earned-description",
			args: []string{"game-center", "achievements", "localizations", "create", "--achievement-id", "ACH_ID", "--locale", "en-US", "--name", "Test", "--after-earned-description", "After"},
		},
		{
			name: "missing after-earned-description",
			args: []string{"game-center", "achievements", "localizations", "create", "--achievement-id", "ACH_ID", "--locale", "en-US", "--name", "Test", "--before-earned-description", "Before"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
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
		})
	}
}

func TestGameCenterAchievementImagesUploadValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing localization-id",
			args: []string{"game-center", "achievements", "images", "upload", "--file", "test.png"},
		},
		{
			name: "missing file",
			args: []string{"game-center", "achievements", "images", "upload", "--localization-id", "LOC_ID"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
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
		})
	}
}

func TestGameCenterAchievementReleasesListValidationErrors(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "achievements", "releases", "list"}); err != nil {
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
}

func TestGameCenterAchievementsListLimitValidation(t *testing.T) {
	t.Setenv("ASC_APP_ID", "APP_ID")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "achievements", "list", "--app", "APP_ID", "--limit", "201"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestGameCenterAchievementGroupAchievementGetValidationErrors(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "achievements", "group-achievement", "get"}); err != nil {
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
}

func TestGameCenterAchievementLocalizationImageGetValidationErrors(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "achievements", "localizations", "image", "get"}); err != nil {
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
}

func TestGameCenterAchievementLocalizationAchievementGetValidationErrors(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "achievements", "localizations", "achievement", "get"}); err != nil {
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
}
