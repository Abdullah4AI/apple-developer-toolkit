//go:build darwin || linux

package signing

import (
	"fmt"
	"os"
	"syscall"
)

func validateSigningRunInputPermissions(path string, info os.FileInfo, private bool) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Geteuid() {
		return fmt.Errorf("%q must be owned by the current user", path)
	}
	if stat.Nlink != 1 {
		return fmt.Errorf("%q must not have multiple hard links", path)
	}
	if private && info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("%q must not be accessible by group or other users", path)
	}
	if !private && info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("%q must not be writable by group or other users", path)
	}
	return nil
}
