package auth

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestPrivateKeyTildeHintRequiresExpandedPath(t *testing.T) {
	for _, path := range []string{"~/AuthKey.p8", "~/key$NAME$(printf expanded)`printf backtick`'quote.p8", "~/key\x1b[2J\r\nforged.p8"} {
		t.Run(path, func(t *testing.T) {
			message := privateKeyOpenError(path, "read private key", os.ErrNotExist).Error()
			if !strings.Contains(message, fmt.Sprintf("private key file not found: %q", path)) {
				t.Fatalf("missing original path: %q", message)
			}
			_, hint, found := strings.Cut(message, " (the shell did not expand ~;")
			if !found || hint != " use an absolute path)" {
				t.Fatalf("expected portable recovery guidance, got %q", message)
			}
			if strings.ContainsAny(message, "\x1b\r\n") {
				t.Fatalf("terminal controls in diagnostic: %q", message)
			}
		})
	}
}
