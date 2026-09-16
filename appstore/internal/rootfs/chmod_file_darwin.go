//go:build darwin

package rootfs

import (
	"os"

	"golang.org/x/sys/unix"
)

func openChmodFile(parent *os.Root, base string) (*os.File, error) {
	return openChmodFileAt(parent, base, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC)
}

func chmodFileDescriptor(file *os.File, mode os.FileMode) error {
	return file.Chmod(mode)
}
