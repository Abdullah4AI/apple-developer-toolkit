//go:build !darwin && !linux && !freebsd && !netbsd && !openbsd && !dragonfly

package secureopen

import "os"

// OpenNewFileNoFollow creates a new file without following symlinks.
// Uses a best-effort pre/post open validation sequence because a portable
// O_NOFOLLOW equivalent is not available on this platform.
func OpenNewFileNoFollow(path string, perm os.FileMode) (*os.File, error) {
	return openWritableFileNoFollowBestEffort(path, perm, func(path string, perm os.FileMode) (*os.File, error) {
		flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
		return os.OpenFile(path, flags, perm)
	})
}

// OpenAppendNoFollow opens a file for appending using best-effort symlink
// checks because a portable O_NOFOLLOW equivalent is not available on this
// platform.
func OpenAppendNoFollow(path string, perm os.FileMode) (*os.File, error) {
	return openWritableFileNoFollowBestEffort(path, perm, func(path string, perm os.FileMode) (*os.File, error) {
		flags := os.O_WRONLY | os.O_APPEND | os.O_CREATE
		return os.OpenFile(path, flags, perm)
	})
}

// OpenExistingNoFollow opens an existing file with best-effort symlink checks.
// The validation/open sequence reduces, but does not eliminate, TOCTOU risk.
func OpenExistingNoFollow(path string) (*os.File, error) {
	return openExistingNoFollowBestEffort(path, os.Open)
}

// OpenNewFileNoFollowInRoot creates a new file relative to root using
// best-effort final-component checks. Root itself prevents parent traversal.
func OpenNewFileNoFollowInRoot(root *os.Root, name string, perm os.FileMode) (*os.File, error) {
	return openWritableFileNoFollowInRootBestEffort(root, name, func() (*os.File, error) {
		return root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	})
}

// OpenLockFileNoFollowInRoot opens or creates a lock file relative to root
// using best-effort final-component checks. It neither truncates nor appends:
// on Windows an O_APPEND handle carries FILE_APPEND_DATA without
// FILE_WRITE_DATA, and LockFileEx rejects such a handle with
// ERROR_ACCESS_DENIED.
func OpenLockFileNoFollowInRoot(root *os.Root, name string, perm os.FileMode) (*os.File, error) {
	return openWritableFileNoFollowInRootBestEffort(root, name, func() (*os.File, error) {
		return root.OpenFile(name, os.O_WRONLY|os.O_CREATE, perm)
	})
}

// OpenExistingAppendNoFollowInRoot opens an existing file for appending
// relative to root using best-effort final-component checks.
func OpenExistingAppendNoFollowInRoot(root *os.Root, name string) (*os.File, error) {
	return openExistingNoFollowInRootBestEffort(root, name, func() (*os.File, error) {
		return root.OpenFile(name, os.O_WRONLY|os.O_APPEND, 0)
	})
}

// OpenExistingNoFollowInRoot opens an existing file relative to root using
// best-effort final-component checks. Root itself prevents parent traversal.
func OpenExistingNoFollowInRoot(root *os.Root, name string) (*os.File, error) {
	return openExistingNoFollowInRootBestEffort(root, name, func() (*os.File, error) {
		return root.Open(name)
	})
}

// OpenExistingWritableNoFollowInRoot opens an existing file for writing
// relative to root using best-effort final-component checks.
func OpenExistingWritableNoFollowInRoot(root *os.Root, name string) (*os.File, error) {
	return openExistingNoFollowInRootBestEffort(root, name, func() (*os.File, error) {
		return root.OpenFile(name, os.O_WRONLY, 0)
	})
}
