//go:build !windows

package xcode

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"

	"golang.org/x/sys/unix"
)

func TestMoveExportedIPANoOverwriteClosesRootDescriptor(t *testing.T) {
	directory := t.TempDir()
	previousGCPercent := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(previousGCPercent)
	warmup := filepath.Join(directory, "warmup.ipa")
	if err := os.WriteFile(warmup, []byte("ipa"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := moveExportedIPA(warmup, filepath.Join(directory, "warmup-destination.ipa"), false); err != nil {
		t.Fatal(err)
	}
	before := countOpenXcodeDescriptors(t)
	for index := range 16 {
		source := filepath.Join(directory, fmt.Sprintf("source-%d.ipa", index))
		destination := filepath.Join(directory, fmt.Sprintf("destination-%d.ipa", index))
		if err := os.WriteFile(source, []byte("ipa"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := moveExportedIPA(source, destination, false); err != nil {
			t.Fatal(err)
		}
	}
	after := countOpenXcodeDescriptors(t)
	if leaked := after - before; leaked > 0 {
		t.Fatalf("moveExportedIPA leaked %d file descriptors", leaked)
	}
}

func countOpenXcodeDescriptors(t *testing.T) int {
	t.Helper()
	var limit unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_NOFILE, &limit); err != nil {
		t.Fatal(err)
	}
	maximum := 4096
	if limit.Cur < uint64(maximum) {
		maximum = int(limit.Cur)
	}
	count := 0
	for descriptor := range maximum {
		if _, err := unix.FcntlInt(uintptr(descriptor), unix.F_GETFD, 0); err == nil {
			count++
		} else if !errors.Is(err, unix.EBADF) {
			t.Fatalf("inspect descriptor %d: %v", descriptor, err)
		}
	}
	return count
}
