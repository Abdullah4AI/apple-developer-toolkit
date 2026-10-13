//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package secureopen

import (
	"os"

	"golang.org/x/sys/unix"
)

// OpenNewFileNoFollow creates a new file without following symlinks.
// Uses O_EXCL to prevent overwriting existing files and O_NOFOLLOW to prevent symlink attacks.
func OpenNewFileNoFollow(path string, perm os.FileMode) (*os.File, error) {
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL | unix.O_NOFOLLOW
	return os.OpenFile(path, flags, perm)
}

// OpenAppendNoFollow opens a file for appending without following symlinks,
// creating it when it does not exist.
func OpenAppendNoFollow(path string, perm os.FileMode) (*os.File, error) {
	flags := os.O_WRONLY | os.O_APPEND | os.O_CREATE | unix.O_NOFOLLOW
	return os.OpenFile(path, flags, perm)
}

// OpenExistingNoFollow opens an existing file without following symlinks.
func OpenExistingNoFollow(path string) (*os.File, error) {
	// O_NONBLOCK prevents hanging when opening FIFOs/devices in untrusted paths.
	flags := os.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK
	return os.OpenFile(path, flags, 0)
}

// OpenNewFileNoFollowInRoot creates a new file relative to root without
// following the final component or permitting parent traversal outside root.
func OpenNewFileNoFollowInRoot(root *os.Root, name string, perm os.FileMode) (*os.File, error) {
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL | unix.O_NOFOLLOW
	return openWritableFileNoFollowInRootBestEffort(root, name, func() (*os.File, error) {
		return root.OpenFile(name, flags, perm)
	})
}

// OpenLockFileNoFollowInRoot opens or creates a lock file relative to root
// without following the final component. It neither truncates nor appends,
// matching the Windows variant, whose LockFileEx needs a handle with write
// access rather than append-only access.
func OpenLockFileNoFollowInRoot(root *os.Root, name string, perm os.FileMode) (*os.File, error) {
	flags := os.O_WRONLY | os.O_CREATE | unix.O_NOFOLLOW | unix.O_NONBLOCK
	return openWritableFileNoFollowInRootBestEffort(root, name, func() (*os.File, error) {
		return root.OpenFile(name, flags, perm)
	})
}

// OpenExistingAppendNoFollowInRoot opens an existing file for atomic appends
// without following the final component.
func OpenExistingAppendNoFollowInRoot(root *os.Root, name string) (*os.File, error) {
	flags := os.O_WRONLY | os.O_APPEND | unix.O_NOFOLLOW | unix.O_NONBLOCK
	return openExistingNoFollowInRootBestEffort(root, name, func() (*os.File, error) {
		return root.OpenFile(name, flags, 0)
	})
}

// OpenExistingNoFollowInRoot opens an existing file relative to root without
// following the final component or permitting parent traversal outside root.
func OpenExistingNoFollowInRoot(root *os.Root, name string) (*os.File, error) {
	flags := os.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK
	return openExistingNoFollowInRootBestEffort(root, name, func() (*os.File, error) {
		return root.OpenFile(name, flags, 0)
	})
}

// OpenExistingWritableNoFollowInRoot opens an existing file for writing
// relative to root without following its final component. Opening the existing
// inode lets callers honor write ACLs and preserve filesystem metadata.
func OpenExistingWritableNoFollowInRoot(root *os.Root, name string) (*os.File, error) {
	flags := os.O_WRONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK
	return openExistingNoFollowInRootBestEffort(root, name, func() (*os.File, error) {
		return root.OpenFile(name, flags, 0)
	})
}
