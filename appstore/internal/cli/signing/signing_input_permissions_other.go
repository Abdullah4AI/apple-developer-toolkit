//go:build !darwin && !linux

package signing

import "os"

func validateSigningRunInputPermissions(string, os.FileInfo, bool) error { return nil }
