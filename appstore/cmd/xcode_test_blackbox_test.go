package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestXcodeTestRejectsEmptyAuthenticationEqualsValuesWithBuiltBinary(t *testing.T) {
	binaryPath := buildASCBlackboxBinary(t)
	configPath := filepath.Join(t.TempDir(), "config.json")

	for _, authFlag := range []string{"-authenticationKeyPath", "-authenticationKeyID", "-authenticationKeyIssuerID"} {
		for _, value := range []string{"", "   "} {
			t.Run(authFlag+"/"+fmt.Sprintf("%q", value), func(t *testing.T) {
				raw := authFlag + "=" + value
				command := exec.Command(
					binaryPath,
					"xcode", "test",
					"--project", filepath.Join(t.TempDir(), "Missing.xcodeproj"),
					"--scheme", "Demo",
					"--destination", "generic/platform=iOS",
					"--xcodebuild-flag="+raw,
				)
				command.Env = isolatedCLITestEnv(configPath)
				var stdout, stderr bytes.Buffer
				command.Stdout = &stdout
				command.Stderr = &stderr

				err := command.Run()
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) {
					t.Fatalf("error = %v, want usage failure", err)
				}
				if exitErr.ExitCode() != ExitUsage {
					t.Fatalf("exit code = %d, want %d; stderr=%q", exitErr.ExitCode(), ExitUsage, stderr.String())
				}
				wantError := fmt.Sprintf("--xcodebuild-flag %q cannot have an empty value", strings.TrimSpace(raw))
				wantPrefix := "Error: " + wantError + "\n"
				if !strings.HasPrefix(stderr.String(), wantPrefix) {
					t.Fatalf("stderr = %q, want exact usage diagnostic prefix %q", stderr.String(), wantPrefix)
				}
				wantUsage := "\nDESCRIPTION\n  Run local Xcode tests and report structured results.\n\nUSAGE\n  asc xcode test [flags]\n"
				if !strings.Contains(stderr.String(), wantUsage) {
					t.Fatalf("stderr = %q, want command usage block %q", stderr.String(), wantUsage)
				}
				if stdout.Len() != 0 {
					t.Fatalf("stdout = %q, want empty", stdout.String())
				}
			})
		}
	}
}
