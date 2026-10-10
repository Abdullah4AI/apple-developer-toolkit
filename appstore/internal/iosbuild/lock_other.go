//go:build !darwin && !linux && !freebsd && !netbsd && !openbsd && !dragonfly && !windows

package iosbuild

import (
	"fmt"
	"os"
)

func lockBuildFile(*os.File) error {
	return fmt.Errorf("iOS builds require package locking, which is unsupported on this platform")
}

func unlockBuildFile(*os.File) error { return nil }
