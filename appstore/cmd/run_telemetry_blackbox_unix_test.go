//go:build darwin || linux

package cmd

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// lockFileExclusive blocks until it holds an exclusive flock on file, the
// lock type the telemetry worker takes.
func lockFileExclusive(file *os.File) error {
	for {
		err := unix.Flock(int(file.Fd()), unix.LOCK_EX)
		if !errors.Is(err, unix.EINTR) {
			return err
		}
	}
}

func unlockFile(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_UN)
}
