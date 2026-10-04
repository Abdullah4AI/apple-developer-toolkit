//go:build windows

package cmd

import (
	"os"

	"golang.org/x/sys/windows"
)

// lockFileExclusive blocks until it holds an exclusive lock on the first byte
// of file, the range the telemetry worker locks.
func lockFileExclusive(file *os.File) error {
	var overlapped windows.Overlapped
	return windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &overlapped)
}

func unlockFile(file *os.File) error {
	var overlapped windows.Overlapped
	return windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, &overlapped)
}
