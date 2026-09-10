package shared

import (
	"flag"
	"fmt"
	"io"
)

const (
	// IncludeSensitiveFlagName is the per-invocation opt-in for printing secrets.
	IncludeSensitiveFlagName = "include-sensitive"

	// IncludeSensitiveFlagUsage documents the opt-in. It has no environment
	// variable default: a secret must never become visible because of ambient
	// configuration.
	IncludeSensitiveFlagUsage = "Print secret values such as demo account passwords instead of \"" +
		"(redacted)\"; applies only to this invocation"

	// IncludeSensitiveWarning is written to stderr whenever the opt-in is used.
	IncludeSensitiveWarning = "Warning: --include-sensitive prints secrets in plain text; " +
		"they can persist in shell history, CI logs, and terminal scrollback."
)

// BindIncludeSensitiveFlag registers --include-sensitive on the flag set.
func BindIncludeSensitiveFlag(fs *flag.FlagSet) *bool {
	return fs.Bool(IncludeSensitiveFlagName, false, IncludeSensitiveFlagUsage)
}

// WarnIncludeSensitive writes the plaintext-secret warning when the opt-in is
// enabled. It is a no-op otherwise so default output stays quiet.
func WarnIncludeSensitive(w io.Writer, includeSensitive bool) {
	if !includeSensitive || w == nil {
		return
	}
	fmt.Fprintln(w, IncludeSensitiveWarning)
}
