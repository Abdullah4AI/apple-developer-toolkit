//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package iosbuild

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func lockBuildFile(file *os.File) error {
	err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
		return fmt.Errorf("another asc builds compile is already running for this package")
	}
	return err
}

func unlockBuildFile(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_UN)
}
