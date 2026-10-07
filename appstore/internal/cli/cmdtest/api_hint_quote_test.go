package cmdtest

import (
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/cmd"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

func TestAPIMissingMethodHintPreservesLiteralPath(t *testing.T) {
	for _, path := range []string{"/v1/apps?limit=5&sort=name", "/v1/apps?filter[name]=Jane's app", "/v1/apps\nforged"} {
		t.Run(path, func(t *testing.T) {
			code := -1
			stdout, stderr := captureOutput(t, func() { code = cmd.Run([]string{"api", path}, "test") })
			if code != cmd.ExitUsage || stdout != "" {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			quoted, ok := shared.ShellQuote(path)
			if ok {
				if !strings.Contains(stderr, "asc api GET "+quoted+"\n") {
					t.Fatalf("hint does not preserve literal path: %q", stderr)
				}
			} else if strings.Contains(stderr, "Hint: pass the method first:") {
				t.Fatalf("unsafe path should omit copyable hint: %q", stderr)
			}
		})
	}
}
