//go:build linux

package rootfs

import (
	"errors"
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

func TestChmodFileDescriptorFallbackDoesNotReportKeyMissing(t *testing.T) {
	var fallbackPath string
	err := chmodFileDescriptorFD(
		42,
		0o600,
		func(int, string, uint32, int) error { return unix.EOPNOTSUPP },
		func(path string, _ uint32) error {
			fallbackPath = path
			return unix.ENOENT
		},
	)

	if fallbackPath != "/proc/self/fd/42" {
		t.Fatalf("fallback path = %q, want retained descriptor path", fallbackPath)
	}
	if !errors.Is(err, ErrFileIdentityMutationUnsupported) {
		t.Fatalf("error = %v, want ErrFileIdentityMutationUnsupported", err)
	}
	if errors.Is(err, os.ErrNotExist) {
		t.Fatalf("fallback infrastructure error must not report the key as missing: %v", err)
	}
}
