package cmdtest

import (
	"strings"
	"testing"
)

func TestXcodeDoctorCommandIsDiscoverable(t *testing.T) {
	root := RootCommand("1.2.3")
	doctor := findSubcommand(root, "xcode", "doctor")
	if doctor == nil {
		t.Fatal("expected xcode doctor command")
	}

	for _, name := range []string{"developer-dir", "sdk", "output", "pretty"} {
		if doctor.FlagSet.Lookup(name) == nil {
			t.Fatalf("expected xcode doctor to expose --%s", name)
		}
	}
	if !strings.Contains(doctor.LongHelp, "DEVELOPER_DIR") || !strings.Contains(doctor.LongHelp, "xcode-select") {
		t.Fatalf("doctor help does not explain selection precedence: %q", doctor.LongHelp)
	}
}
