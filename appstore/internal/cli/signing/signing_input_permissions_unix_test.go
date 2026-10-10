//go:build darwin || linux

package signing

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadBoundedSigningRunFilePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.p12")
	if err := os.WriteFile(path, []byte("identity"), 0o600); err != nil {
		t.Fatalf("write input: %v", err)
	}
	if _, err := readBoundedSigningRunFile(path, 32, true); err != nil {
		t.Fatalf("private input rejected: %v", err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	if _, err := readBoundedSigningRunFile(path, 32, true); err == nil {
		t.Fatal("expected group/world-readable private input rejection")
	}
	if _, err := readBoundedSigningRunFile(path, 32, false); err != nil {
		t.Fatalf("read-only profile input rejected: %v", err)
	}
	if err := os.Chmod(path, 0o666); err != nil {
		t.Fatalf("chmod writable: %v", err)
	}
	if _, err := readBoundedSigningRunFile(path, 32, false); err == nil {
		t.Fatal("expected group/world-writable profile input rejection")
	}
}
