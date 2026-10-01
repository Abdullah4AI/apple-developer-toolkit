package cmdtest

import (
	"strings"
	"testing"

	rootcmd "github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
)

func TestWebICloudContainersCreateRunRejectsUnprefixedIdentifier(t *testing.T) {
	setCmdtestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "1")
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{
			"web", "icloud-containers", "create",
			"--identifier", "com.example.app",
			"--name", "Example", "--confirm",
		}, "1.2.3")
	})
	if code != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d; stderr = %q", code, rootcmd.ExitUsage, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if count := strings.Count(stderr, `Error: --identifier must start with "iCloud."`); count != 1 {
		t.Fatalf("prefix diagnostic count = %d, want 1; stderr = %q", count, stderr)
	}
}

func TestWebICloudContainersCreateRunRequiresConfirm(t *testing.T) {
	setCmdtestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "1")
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{
			"web", "icloud-containers", "create",
			"--identifier", "iCloud.com.example.app",
			"--name", "Example",
		}, "1.2.3")
	})
	if code != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d; stderr = %q", code, rootcmd.ExitUsage, stderr)
	}
	if stdout != "" || !strings.Contains(stderr, "--confirm is required") {
		t.Fatalf("stdout = %q, stderr = %q; want only the --confirm diagnostic", stdout, stderr)
	}
}

func TestWebICloudContainersCreateRunRejectsUnsupportedFlags(t *testing.T) {
	setCmdtestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "1")
	for _, flag := range []string{"--hidden", "--paginate", "--container-id=cloud-1"} {
		t.Run(flag, func(t *testing.T) {
			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run([]string{
					"web", "icloud-containers", "create",
					"--identifier", "iCloud.com.example.app",
					"--name", "Example", "--confirm", flag,
				}, "1.2.3")
			})
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d; stderr = %q", code, rootcmd.ExitUsage, stderr)
			}
			if stdout != "" || !strings.Contains(stderr, "unknown flag") {
				t.Fatalf("stdout = %q, stderr = %q; want only unknown-flag diagnostic", stdout, stderr)
			}
		})
	}
}
