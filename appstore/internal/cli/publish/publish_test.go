package publish

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

func TestValidateIPAPathRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.ipa")
	if err := os.WriteFile(target, []byte("payload"), 0o600); err != nil {
		t.Fatalf("write target file: %v", err)
	}

	link := filepath.Join(dir, "app.ipa")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	_, err := shared.ValidateIPAPath(link)
	if err == nil {
		t.Fatal("expected symlink rejection error")
	}
	if !strings.Contains(err.Error(), "refusing to read symlink") {
		t.Fatalf("expected symlink rejection message, got %v", err)
	}
}

func TestValidateIPAPathAllowsRegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.ipa")
	content := []byte("payload")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write IPA file: %v", err)
	}

	info, err := shared.ValidateIPAPath(path)
	if err != nil {
		t.Fatalf("ValidateIPAPath returned error: %v", err)
	}
	if info.Size() != int64(len(content)) {
		t.Fatalf("expected size %d, got %d", len(content), info.Size())
	}
}
