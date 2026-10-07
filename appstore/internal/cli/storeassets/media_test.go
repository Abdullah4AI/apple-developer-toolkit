package storeassets

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestDuoPreviewPreflight(t *testing.T) {
	previousLookup, previousRun := lookupProbe, runProbeCommand
	t.Cleanup(func() { lookupProbe, runProbeCommand = previousLookup, previousRun })
	lookupProbe = func(string) (string, error) { return os.Executable() }
	for _, tc := range []struct {
		name          string
		width, height int
		rotation      int
		wantErr       bool
	}{
		{"portrait", 886, 1920, 0, false},
		{"landscape", 1920, 886, 0, false},
		{"rotated portrait", 1920, 886, 90, false},
		{"outer screenshot portrait", 1398, 2034, 0, true},
		{"outer screenshot landscape", 2034, 1398, 0, true},
		{"inner screenshot portrait", 2007, 2853, 0, true},
		{"inner screenshot landscape", 2853, 2007, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report := fmt.Sprintf(`{"streams":[{"width":%d,"height":%d,"side_data_list":[{"rotation":%d}]}],"format":{"duration":"20"}}`, tc.width, tc.height, tc.rotation)
			runProbeCommand = func(ctx context.Context, executable string, _ ...string) *exec.Cmd {
				return exec.CommandContext(ctx, executable, "-test.run=^TestDuoProbeProcess$", "--", report)
			}
			err := validateVideo(context.Background(), "duo.mp4", "IPHONE_DUO", "00:00:05.000")
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), "unsupported IPHONE_DUO preview dimensions") {
					t.Fatalf("expected screenshot resolution rejection, got %v", err)
				}
			} else if err != nil {
				t.Fatalf("valid Duo preview rejected: %v", err)
			}
		})
	}
}

func TestDuoProbeProcess(t *testing.T) {
	for i, arg := range os.Args {
		if arg == "--" && i+1 < len(os.Args) {
			fmt.Print(os.Args[i+1])
			os.Exit(0)
		}
	}
}
