package cmdtest

import "testing"

// TestWebXcodeCloudSCMCommandsAreRegistered is the regression for the
// previously observed root-binary failure where the lifecycle SCM command
// was reported as an unknown subcommand.
func TestWebXcodeCloudSCMCommandsAreRegistered(t *testing.T) {
	root := RootCommand("1.2.3")

	for _, path := range [][]string{
		{"web", "xcode-cloud", "scm", "providers", "list"},
		{"web", "xcode-cloud", "scm", "connection-status"},
	} {
		if command := findSubcommand(root, path...); command == nil {
			t.Fatalf("expected %v to be registered", path)
		}
	}
}
