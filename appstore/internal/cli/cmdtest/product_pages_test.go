package cmdtest

import (
	"context"
	"errors"
	"flag"
	"strings"
	"testing"

	rootcmd "github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
)

func TestProductPagesCustomPagesListRequiresApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "custom-pages", "list"}); err != nil {
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
	if !strings.Contains(stderr, "--app is required") {
		t.Fatalf("expected missing app error, got %q", stderr)
	}
}

func TestProductPagesCustomPagesDeleteRequiresConfirm(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "custom-pages", "delete", "--custom-page-id", "page-1"}); err != nil {
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
	if !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("expected missing confirm error, got %q", stderr)
	}
}

func TestProductPagesExperimentsCreateRequiresVersionID(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "experiments", "create", "--name", "Icon Test", "--traffic-proportion", "50"}); err != nil {
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
	if !strings.Contains(stderr, "--version-id is required") {
		t.Fatalf("expected missing version-id error, got %q", stderr)
	}
}

func TestProductPagesExperimentsCreateRequiresAppForV2(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "experiments", "create", "--v2", "--platform", "IOS", "--name", "Icon Test", "--traffic-proportion", "50"}); err != nil {
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
	if !strings.Contains(stderr, "--app is required") {
		t.Fatalf("expected missing app error, got %q", stderr)
	}
}

func TestProductPagesExperimentsDeleteRequiresConfirm(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "experiments", "delete", "--experiment-id", "exp-1"}); err != nil {
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
	if !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("expected missing confirm error, got %q", stderr)
	}
}

func TestProductPagesCustomPagesListRejectsInvalidLimit(t *testing.T) {
	root := RootCommand("1.2.3")

	tests := []struct {
		name  string
		limit string
	}{
		{
			name:  "limit below range",
			limit: "-1",
		},
		{
			name:  "limit above range",
			limit: "201",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{"product-pages", "custom-pages", "list", "--app", "APP_ID", "--limit", test.limit}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			assertUsageDiagnosticFirstLine(t, stderr, "custom-pages list: --limit must be between 1 and 200")
		})
	}
}

func TestProductPagesCustomPagesListRejectsInvalidNextURL(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "custom-pages", "list", "--app", "APP_ID", "--next", "not-a-url"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	assertUsageDiagnosticFirstLine(t, stderr, "custom-pages list: --next must be an App Store Connect URL")
}

func TestProductPagesExperimentTreatmentLocalizationMediaSetsValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "treatment localization preview sets list missing localization",
			args: []string{"product-pages", "experiments", "treatments", "localizations", "preview-sets", "list"},
		},
		{
			name: "treatment localization screenshot sets list missing localization",
			args: []string{"product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "list"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")

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
			if stderr == "" {
				t.Fatalf("expected stderr output")
			}
		})
	}
}

func TestProductPagesScreenshotSetIncludeScreenshotsIsRegistered(t *testing.T) {
	root := RootCommand("1.2.3")
	cases := [][]string{
		{"product-pages", "custom-pages", "localizations", "screenshot-sets", "list"},
		{"product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "list"},
	}

	for _, path := range cases {
		cmd := findSubcommand(root, path...)
		if cmd == nil {
			t.Fatalf("command %v not found", path)
		}
		includeScreenshots := cmd.FlagSet.Lookup("include-screenshots")
		if includeScreenshots == nil {
			t.Fatalf("command %v missing --include-screenshots", path)
		}
	}
}

func TestProductPagesScreenshotSetIncludeScreenshotsRequiresFullLocalizationList(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/appCustomProductPageLocalizations/loc-1/appScreenshotSets?cursor=next"

	cases := []struct {
		name           string
		path           []string
		localizationID string
		next           string
		wantStderr     string
	}{
		{
			name:       "custom pages requires localization id for expansion",
			path:       []string{"product-pages", "custom-pages", "localizations", "screenshot-sets", "list"},
			wantStderr: "Error: --localization-id is required",
		},
		{
			name:           "custom pages requires paginate",
			path:           []string{"product-pages", "custom-pages", "localizations", "screenshot-sets", "list"},
			localizationID: "loc-1",
			wantStderr:     "custom-pages localizations screenshot-sets list: --include-screenshots requires --paginate",
		},
		{
			name:           "custom pages rejects next",
			path:           []string{"product-pages", "custom-pages", "localizations", "screenshot-sets", "list"},
			localizationID: "loc-1",
			next:           nextURL,
			wantStderr:     "custom-pages localizations screenshot-sets list: --include-screenshots cannot be combined with --next",
		},
		{
			name:       "treatment requires localization id for expansion",
			path:       []string{"product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "list"},
			wantStderr: "Error: --localization-id is required",
		},
		{
			name:           "treatment requires paginate",
			path:           []string{"product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "list"},
			localizationID: "loc-1",
			wantStderr:     "experiments treatments localizations screenshot-sets list: --include-screenshots requires --paginate",
		},
		{
			name:           "treatment rejects next",
			path:           []string{"product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "list"},
			localizationID: "loc-1",
			next:           nextURL,
			wantStderr:     "experiments treatments localizations screenshot-sets list: --include-screenshots cannot be combined with --next",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			args := append(append([]string{}, test.path...), "--include-screenshots")
			if test.localizationID != "" {
				args = append(args, "--localization-id", test.localizationID)
			}
			if test.next != "" {
				args = append(args, "--paginate", "--next", test.next)
			}

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
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
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("expected %q in stderr, got %q", test.wantStderr, stderr)
			}
		})
	}
}

func TestProductPagesCustomPagesLocalizationsPreviewSetsListRejectsInvalidLimit(t *testing.T) {
	root := RootCommand("1.2.3")

	tests := []struct {
		name  string
		limit string
	}{
		{
			name:  "limit below range",
			limit: "-1",
		},
		{
			name:  "limit above range",
			limit: "201",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{"product-pages", "custom-pages", "localizations", "preview-sets", "list", "--localization-id", "loc-1", "--limit", test.limit}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			assertUsageDiagnosticFirstLine(t, stderr, "custom-pages localizations preview-sets list: --limit must be between 1 and 200")
		})
	}
}

func TestProductPagesCustomPagesLocalizationsPreviewSetsListRejectsInvalidNextURL(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "custom-pages", "localizations", "preview-sets", "list", "--localization-id", "loc-1", "--next", "not-a-url"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	assertUsageDiagnosticFirstLine(t, stderr, "custom-pages localizations preview-sets list: --next must be an App Store Connect URL")
}

func TestProductPagesCustomPagesLocalizationsScreenshotSetsListRejectsInvalidLimit(t *testing.T) {
	root := RootCommand("1.2.3")

	tests := []struct {
		name  string
		limit string
	}{
		{
			name:  "limit below range",
			limit: "-1",
		},
		{
			name:  "limit above range",
			limit: "201",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{"product-pages", "custom-pages", "localizations", "screenshot-sets", "list", "--localization-id", "loc-1", "--limit", test.limit}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			assertUsageDiagnosticFirstLine(t, stderr, "custom-pages localizations screenshot-sets list: --limit must be between 1 and 200")
		})
	}
}

func TestProductPagesCustomPagesLocalizationsScreenshotSetsListRejectsInvalidNextURL(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "custom-pages", "localizations", "screenshot-sets", "list", "--localization-id", "loc-1", "--next", "not-a-url"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	assertUsageDiagnosticFirstLine(t, stderr, "custom-pages localizations screenshot-sets list: --next must be an App Store Connect URL")
}

func TestProductPagesCustomPagesLocalizationsSearchKeywordsListRequiresLocalizationID(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "custom-pages", "localizations", "search-keywords", "list"}); err != nil {
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
	if !strings.Contains(stderr, "--localization-id is required") {
		t.Fatalf("expected missing localization-id error, got %q", stderr)
	}
}

func TestProductPagesCustomPagesLocalizationsSearchKeywordsAddRequiresLocalizationID(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "custom-pages", "localizations", "search-keywords", "add", "--keywords", "kw-1"}); err != nil {
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
	if !strings.Contains(stderr, "--localization-id is required") {
		t.Fatalf("expected missing localization-id error, got %q", stderr)
	}
}

func TestProductPagesCustomPagesLocalizationsSearchKeywordsAddRequiresKeywords(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "custom-pages", "localizations", "search-keywords", "add", "--localization-id", "loc-1"}); err != nil {
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
	if !strings.Contains(stderr, "--keywords is required") {
		t.Fatalf("expected missing keywords error, got %q", stderr)
	}
}

func TestProductPagesCustomPagesLocalizationsSearchKeywordsDeleteRequiresConfirm(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "custom-pages", "localizations", "search-keywords", "delete", "--localization-id", "loc-1", "--keywords", "kw-1"}); err != nil {
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
	if !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("expected missing confirm error, got %q", stderr)
	}
}

func TestProductPagesCustomPagesLocalizationsSearchKeywordsDeleteRequiresKeywords(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "custom-pages", "localizations", "search-keywords", "delete", "--localization-id", "loc-1", "--confirm"}); err != nil {
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
	if !strings.Contains(stderr, "--keywords is required") {
		t.Fatalf("expected missing keywords error, got %q", stderr)
	}
}

func TestProductPagesCustomPagesLocalizationsPreviewSetsListRequiresLocalizationID(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "custom-pages", "localizations", "preview-sets", "list"}); err != nil {
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
	if !strings.Contains(stderr, "--localization-id is required") {
		t.Fatalf("expected missing localization-id error, got %q", stderr)
	}
}

func TestProductPagesCustomPagesLocalizationsScreenshotSetsListRequiresLocalizationID(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "custom-pages", "localizations", "screenshot-sets", "list"}); err != nil {
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
	if !strings.Contains(stderr, "--localization-id is required") {
		t.Fatalf("expected missing localization-id error, got %q", stderr)
	}
}

func TestProductPagesCustomPagesLocalizationMediaUploadAndSyncValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "preview sets upload missing localization id",
			args: []string{
				"product-pages", "custom-pages", "localizations", "preview-sets", "upload",
				"--path", "./previews",
				"--device-type", "IPHONE_65",
			},
			wantErr: "--localization-id is required",
		},
		{
			name: "preview sets sync missing confirm",
			args: []string{
				"product-pages", "custom-pages", "localizations", "preview-sets", "sync",
				"--localization-id", "loc-1",
				"--path", "./previews",
				"--device-type", "IPHONE_65",
			},
			wantErr: "--confirm is required to sync",
		},
		{
			name: "screenshot sets upload missing localization id",
			args: []string{
				"product-pages", "custom-pages", "localizations", "screenshot-sets", "upload",
				"--path", "./screenshots",
				"--device-type", "IPHONE_65",
			},
			wantErr: "--localization-id is required",
		},
		{
			name: "screenshot sets sync missing confirm",
			args: []string{
				"product-pages", "custom-pages", "localizations", "screenshot-sets", "sync",
				"--localization-id", "loc-1",
				"--path", "./screenshots",
				"--device-type", "IPHONE_65",
			},
			wantErr: "--confirm is required to sync",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")

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

func TestProductPagesCustomPagesLocalizationsScreenshotSetsUploadRejectsInvalidDeviceType(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"product-pages", "custom-pages", "localizations", "screenshot-sets", "upload",
			"--localization-id", "loc-1",
			"--path", "./screenshots",
			"--device-type", "not-a-device",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if errors.Is(err, flag.ErrHelp) {
			t.Fatalf("unexpected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestProductPagesCustomPagesLocalizationsPreviewSetsUploadRejectsInvalidDeviceType(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"product-pages", "custom-pages", "localizations", "preview-sets", "upload",
			"--localization-id", "loc-1",
			"--path", "./previews",
			"--device-type", "not-a-device",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if errors.Is(err, flag.ErrHelp) {
			t.Fatalf("unexpected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}
