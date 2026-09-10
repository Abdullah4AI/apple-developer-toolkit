package shared

import (
	"bytes"
	"flag"
	"os"
	"strings"
	"testing"
)

func TestBindIncludeSensitiveFlagDefaultsFalseWithoutEnvironmentOverride(t *testing.T) {
	// A secret must never become visible because of ambient configuration, so
	// plausible environment spellings are set and must all be ignored.
	for _, key := range []string{
		"ASC_INCLUDE_SENSITIVE",
		"ASC_SHOW_SENSITIVE",
		"INCLUDE_SENSITIVE",
	} {
		t.Setenv(key, "1")
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	includeSensitive := BindIncludeSensitiveFlag(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if *includeSensitive {
		t.Fatal("--include-sensitive defaulted to true")
	}

	if err := fs.Parse([]string{"--" + IncludeSensitiveFlagName}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if !*includeSensitive {
		t.Fatal("--include-sensitive did not enable when passed explicitly")
	}
}

func TestWarnIncludeSensitiveOnlyWarnsWhenEnabled(t *testing.T) {
	var quiet bytes.Buffer
	WarnIncludeSensitive(&quiet, false)
	if quiet.Len() != 0 {
		t.Fatalf("default invocation wrote %q, want nothing", quiet.String())
	}

	var loud bytes.Buffer
	WarnIncludeSensitive(&loud, true)
	warning := loud.String()
	if !strings.Contains(warning, "--include-sensitive prints secrets in plain text") {
		t.Fatalf("warning = %q, want plaintext-secret wording", warning)
	}
	if !strings.Contains(warning, "CI logs") {
		t.Fatalf("warning = %q, want log-exposure guidance", warning)
	}
}
