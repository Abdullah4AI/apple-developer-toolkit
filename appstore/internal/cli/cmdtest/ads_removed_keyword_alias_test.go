package cmdtest

import (
	"strings"
	"testing"

	rootcmd "github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
)

// TestAdsV5NegativeKeywordViewKeywordAliasIsRemoved locks the 5.0.0 removal of
// the CLI-side hidden `--keyword` alias on the deprecated v5 negative keyword
// view commands. The v5 tree and its Apple-retirement warnings stay; only
// `--negative-keyword` is registered, so the old spelling fails with migration
// guidance before any Apple Ads authentication.
func TestAdsV5NegativeKeywordViewKeywordAliasIsRemoved(t *testing.T) {
	setupUsageExitCodeEnv(t)
	t.Setenv("ASC_ADS_CLIENT_ID", "")
	t.Setenv("ASC_ADS_TEAM_ID", "")
	t.Setenv("ASC_ADS_KEY_ID", "")
	t.Setenv("ASC_ADS_PRIVATE_KEY", "")
	t.Setenv("ASC_ADS_PRIVATE_KEY_PATH", "")

	for _, path := range [][]string{
		{"ads", "v5", "campaign-negative-keywords", "view"},
		{"ads", "v5", "ad-group-negative-keywords", "view"},
	} {
		t.Run(strings.Join(path[1:], " "), func(t *testing.T) {
			root := RootCommand("5.0.0")
			cmd := findSubcommand(root, path...)
			if cmd == nil {
				t.Fatalf("asc %s is not registered", strings.Join(path, " "))
			}
			if cmd.FlagSet.Lookup("negative-keyword") == nil {
				t.Fatalf("canonical --negative-keyword is not registered on asc %s", strings.Join(path, " "))
			}
			if cmd.FlagSet.Lookup("keyword") != nil {
				t.Fatalf("removed alias --keyword is still registered on asc %s", strings.Join(path, " "))
			}

			args := append(append([]string{}, path...), "--org", "1", "--campaign", "2")
			if path[2] == "ad-group-negative-keywords" {
				args = append(args, "--ad-group", "3")
			}
			args = append(args, "--keyword", "4")

			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(args, "5.0.0")
			})

			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			commandPath := "asc " + strings.Join(path, " ")
			want := "Error: `--keyword` was removed in 5.0.0; use `--negative-keyword` (see migrate-to-5-0)\nFor help:\n  " + commandPath + " --help\n"
			if stderr != want {
				t.Fatalf("stderr = %q, want removed-flag guidance %q", stderr, want)
			}
		})
	}
}
