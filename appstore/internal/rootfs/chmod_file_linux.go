//go:build linux

package rootfs

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func openChmodFile(parent *os.Root, base string) (*os.File, error) {
	return openChmodFileAt(parent, base, unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC)
}

func chmodFileDescriptor(file *os.File, mode os.FileMode) error {
	raw, err := file.SyscallConn()
	if err != nil {
		return err
	}

	var chmodErr error
	if err := raw.Control(func(fd uintptr) {
		chmodErr = chmodFileDescriptorFD(int(fd), unixFileMode(mode), unix.Fchmodat, unix.Chmod)
	}); err != nil {
		return err
	}
	return chmodErr
}

func chmodFileDescriptorFD(
	fd int,
	mode uint32,
	fchmodat func(int, string, uint32, int) error,
	chmod func(string, uint32) error,
) error {
	err := fchmodat(fd, "", mode, unix.AT_EMPTY_PATH)
	if !errors.Is(err, unix.EOPNOTSUPP) && !errors.Is(err, unix.ENOSYS) && !errors.Is(err, unix.EINVAL) {
		return err
	}

	fallbackErr := chmod(fmt.Sprintf("/proc/self/fd/%d", fd), mode)
	if errors.Is(fallbackErr, unix.ENOENT) || errors.Is(fallbackErr, unix.ENOTDIR) {
		return fmt.Errorf("%w: descriptor chmod fallback is unavailable: %s", ErrFileIdentityMutationUnsupported, fallbackErr.Error())
	}
	return fallbackErr
}

func unixFileMode(mode os.FileMode) uint32 {
	result := uint32(mode.Perm())
	if mode&os.ModeSetuid != 0 {
		result |= unix.S_ISUID
	}
	if mode&os.ModeSetgid != 0 {
		result |= unix.S_ISGID
	}
	if mode&os.ModeSticky != 0 {
		result |= unix.S_ISVTX
	}
	return result
}
