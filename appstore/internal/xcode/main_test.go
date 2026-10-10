package xcode

import (
	"flag"
	"os"
	"testing"
)

// Helper tests re-exec this binary. A coverage-instrumented child without
// GOCOVERDIR prints a warning to stderr, which corrupts asserted child output.
func TestMain(m *testing.M) {
	flag.Parse()
	if testing.CoverMode() == "" || os.Getenv("GOCOVERDIR") != "" {
		os.Exit(m.Run())
	}
	coverDir := flag.Lookup("test.gocoverdir").Value.String()
	temporary := coverDir == ""
	if temporary {
		dir, err := os.MkdirTemp("", "xcode-test-cover-")
		if err != nil {
			panic(err)
		}
		coverDir = dir
	}
	if err := os.Setenv("GOCOVERDIR", coverDir); err != nil {
		panic(err)
	}
	code := m.Run()
	if temporary {
		_ = os.RemoveAll(coverDir)
	}
	os.Exit(code)
}
