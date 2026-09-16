//go:build darwin || linux

package rootfs

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func openChmodFileAt(parent *os.Root, base string, flags int) (*os.File, error) {
	directory, err := parent.Open(".")
	if err != nil {
		return nil, err
	}
	defer directory.Close()

	raw, err := directory.SyscallConn()
	if err != nil {
		return nil, err
	}
	openedFD := -1
	var openErr error
	if err := raw.Control(func(parentFD uintptr) {
		openedFD, openErr = unix.Openat(int(parentFD), base, flags, 0)
	}); err != nil {
		return nil, err
	}
	if openErr != nil {
		if errors.Is(openErr, unix.ELOOP) {
			return nil, fmt.Errorf("%w: %q", ErrSymlink, base)
		}
		return nil, openErr
	}

	file := os.NewFile(uintptr(openedFD), base)
	if file == nil {
		_ = unix.Close(openedFD)
		return nil, fmt.Errorf("open %q: invalid file descriptor", base)
	}
	return file, nil
}
