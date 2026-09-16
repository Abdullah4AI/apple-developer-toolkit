package cmdtest

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	rootcmd "github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
)

// TestRemovedHiddenFlagAliasesAreUnknownFlags locks the 5.0.0 removal of the
// hidden compatibility spellings that 4.x accepted behind a deprecation
// warning. Each removed alias now fails with migration guidance before any
// HTTP request, and the canonical spelling stays registered.
func TestRemovedHiddenFlagAliasesAreUnknownFlags(t *testing.T) {
	tests := []struct {
		path      []string
		alias     string
		canonical string
		args      []string
	}{
		{path: []string{"versions", "view"}, alias: "id", canonical: "version-id", args: []string{"versions", "view", "--id", "version-1"}},
		{path: []string{"versions", "update"}, alias: "id", canonical: "version-id", args: []string{"versions", "update", "--id", "version-1", "--copyright", "2026 Example"}},
		{path: []string{"versions", "attach-build"}, alias: "build", canonical: "build-id", args: []string{"versions", "attach-build", "--version-id", "version-1", "--build", "build-1"}},
		{path: []string{"apps", "view"}, alias: "app", canonical: "id", args: []string{"apps", "view", "--app", "app-1"}},
		{path: []string{"apps", "app-encryption-declarations", "list"}, alias: "build", canonical: "build-id", args: []string{"apps", "app-encryption-declarations", "list", "--id", "app-1", "--build", "build-1"}},
		{path: []string{"encryption", "declarations", "list"}, alias: "build", canonical: "build-id", args: []string{"encryption", "declarations", "list", "--app", "app-1", "--build", "build-1"}},
		{path: []string{"encryption", "declarations", "assign-builds"}, alias: "build", canonical: "build-id", args: []string{"encryption", "declarations", "assign-builds", "--id", "decl-1", "--build", "build-1"}},
		{path: []string{"bundle-ids", "capabilities", "list"}, alias: "bundle-id", canonical: "bundle", args: []string{"bundle-ids", "capabilities", "list", "--bundle-id", "bundle-1"}},
		{path: []string{"localizations", "list"}, alias: "version-id", canonical: "version", args: []string{"localizations", "list", "--version-id", "version-1"}},
		{path: []string{"localizations", "update"}, alias: "version-id", canonical: "version", args: []string{"localizations", "update", "--version-id", "version-1", "--locale", "en-US", "--description", "Updated"}},
		{path: []string{"screenshots", "list"}, alias: "localization-id", canonical: "version-localization", args: []string{"screenshots", "list", "--localization-id", "loc-1"}},
	}

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("removed aliases must fail before HTTP: %s %s", req.Method, req.URL.String())
		return nil, errors.New("unexpected request")
	}))

	root := RootCommand("1.2.3")
	for _, test := range tests {
		commandPath := strings.Join(test.path, " ")
		t.Run(commandPath+" --"+test.alias, func(t *testing.T) {
			command := findSubcommand(root, test.path...)
			if command == nil {
				t.Fatalf("command %q not found", commandPath)
			}
			if command.FlagSet.Lookup(test.canonical) == nil {
				t.Fatalf("canonical flag --%s not found on %q", test.canonical, commandPath)
			}
			if command.FlagSet.Lookup(test.alias) != nil {
				t.Fatalf("removed alias --%s is still registered on %q", test.alias, commandPath)
			}

			stdout, stderr := captureOutput(t, func() {
				if code := rootcmd.Run(test.args, "1.2.3"); code != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
				}
			})
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			want := "Error: `--" + test.alias + "` was removed in 5.0.0; use `--" + test.canonical + "`"
			if !strings.Contains(stderr, want) {
				t.Fatalf("stderr = %q, want containing %q", stderr, want)
			}
			if strings.Contains(stderr, "deprecated") {
				t.Fatalf("stderr = %q, want no deprecation guidance for a removed alias", stderr)
			}
		})
	}
}

// TestRemovedVisibleAppInfoAliasesAreUnknownFlags locks the 5.0.0 removal of
// the visible "Deprecated alias for ..." spellings on the apps info surface.
func TestRemovedVisibleAppInfoAliasesAreUnknownFlags(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	tests := []struct {
		path  []string
		alias string
		args  []string
	}{
		{path: []string{"apps", "info", "view"}, alias: "app-info", args: []string{"apps", "info", "view", "--app-info", "info-1"}},
		{path: []string{"apps", "info", "relationships", "primary-category"}, alias: "id", args: []string{"apps", "info", "relationships", "primary-category", "--id", "info-1"}},
		{path: []string{"apps", "info", "relationships", "primary-subcategory-one"}, alias: "id", args: []string{"apps", "info", "relationships", "primary-subcategory-one", "--id", "info-1"}},
		{path: []string{"apps", "info", "relationships", "primary-subcategory-two"}, alias: "id", args: []string{"apps", "info", "relationships", "primary-subcategory-two", "--id", "info-1"}},
		{path: []string{"apps", "info", "relationships", "secondary-category"}, alias: "id", args: []string{"apps", "info", "relationships", "secondary-category", "--id", "info-1"}},
		{path: []string{"apps", "info", "relationships", "secondary-subcategory-one"}, alias: "id", args: []string{"apps", "info", "relationships", "secondary-subcategory-one", "--id", "info-1"}},
		{path: []string{"apps", "info", "relationships", "secondary-subcategory-two"}, alias: "id", args: []string{"apps", "info", "relationships", "secondary-subcategory-two", "--id", "info-1"}},
		{path: []string{"apps", "info", "territory-age-ratings", "list"}, alias: "id", args: []string{"apps", "info", "territory-age-ratings", "list", "--id", "info-1"}},
	}

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("removed aliases must fail before HTTP: %s %s", req.Method, req.URL.String())
		return nil, errors.New("unexpected request")
	}))

	root := RootCommand("1.2.3")
	for _, test := range tests {
		commandPath := strings.Join(test.path, " ")
		t.Run(commandPath+" --"+test.alias, func(t *testing.T) {
			command := findSubcommand(root, test.path...)
			if command == nil {
				t.Fatalf("command %q not found", commandPath)
			}
			if command.FlagSet.Lookup("info-id") == nil {
				t.Fatalf("canonical flag --info-id not found on %q", commandPath)
			}
			if command.FlagSet.Lookup(test.alias) != nil {
				t.Fatalf("removed alias --%s is still registered on %q", test.alias, commandPath)
			}

			usage := command.UsageFunc(command)
			if strings.Contains(usage, "Deprecated alias") {
				t.Fatalf("help for %q still advertises a deprecated alias: %q", commandPath, usage)
			}

			assertUsageExit(t, test.args, "Error: `--"+test.alias+"` was removed in 5.0.0; use `--info-id`")
		})
	}
}

// TestPreOrdersEnableIgnoredFlagIsUnknown locks the 5.0.0 removal of the
// warn-and-ignore --available-in-new-territories flag on pre-orders enable.
func TestPreOrdersEnableIgnoredFlagIsUnknown(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("removed flag must fail before HTTP: %s %s", req.Method, req.URL.String())
		return nil, errors.New("unexpected request")
	}))

	command := findSubcommand(RootCommand("1.2.3"), "pre-orders", "enable")
	if command == nil {
		t.Fatal("pre-orders enable not found")
	}
	if command.FlagSet.Lookup("available-in-new-territories") != nil {
		t.Fatal("removed --available-in-new-territories flag is still registered on pre-orders enable")
	}

	stdout, stderr := captureOutput(t, func() {
		if code := rootcmd.Run([]string{
			"pre-orders", "enable",
			"--app", "app-1",
			"--territory", "USA",
			"--release-date", "2026-06-01",
			"--available-in-new-territories", "true",
		}, "1.2.3"); code != rootcmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "Error: `--available-in-new-territories` was removed in 5.0.0") {
		t.Fatalf("stderr = %q, want removed-flag guidance", stderr)
	}
	if strings.Contains(stderr, "deprecated and ignored") {
		t.Fatalf("stderr = %q, want no warn-and-ignore diagnostic", stderr)
	}
}
