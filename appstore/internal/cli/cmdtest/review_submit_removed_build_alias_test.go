package cmdtest

import (
	"errors"
	"strings"
	"testing"

	rootcmd "github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

// TestReviewSubmitBuildAliasIsRemoved locks the 5.0.0 removal of the hidden
// `--build` alias on `asc review submit`. Only `--build-id` is registered, so
// the old spelling fails with migration guidance before authentication or HTTP.
func TestReviewSubmitBuildAliasIsRemoved(t *testing.T) {
	setupUsageExitCodeEnv(t)

	root := RootCommand("5.0.0")
	submit := findSubcommand(root, "review", "submit")
	if submit == nil {
		t.Fatal("asc review submit is not registered")
	}
	if submit.FlagSet.Lookup("build-id") == nil {
		t.Fatal("canonical --build-id is not registered on asc review submit")
	}
	if submit.FlagSet.Lookup("build") != nil {
		t.Fatal("removed alias --build is still registered on asc review submit")
	}

	factoryCalled := false
	restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		factoryCalled = true
		return nil, errors.New("poison client factory called")
	})
	t.Cleanup(restore)

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"review", "submit", "--app", "app-1", "--version-id", "version-1", "--build", "build-1", "--dry-run"}, "5.0.0")
	})

	if code != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	const want = "Error: `--build` was removed in 5.0.0; use `--build-id` (see migrate-to-5-0)\nFor help:\n  asc review submit --help\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want removed-flag guidance %q", stderr, want)
	}
	if strings.Contains(stderr, "deprecated") {
		t.Fatalf("stderr = %q, must not carry the retired alias warning", stderr)
	}
	if factoryCalled {
		t.Fatal("client factory called for removed --build alias")
	}
}
