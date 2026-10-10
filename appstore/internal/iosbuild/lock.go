package iosbuild

import (
	"fmt"
	"os"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/secureopen"
)

func acquireBuildLock(root *os.Root) (*os.File, error) {
	const name = ".asc-ios-build.lock"
	file, err := secureopen.OpenAppendNoFollowInRoot(root, name, 0o600)
	if err != nil {
		return nil, err
	}
	if err := lockBuildFile(file); err != nil {
		file.Close()
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	current, err := root.Lstat(name)
	if err != nil {
		file.Close()
		return nil, err
	}
	if !opened.Mode().IsRegular() || !current.Mode().IsRegular() || !os.SameFile(opened, current) {
		file.Close()
		return nil, fmt.Errorf("build lock must remain a regular file")
	}
	// Keep the inode in place: unlinking would let a later build lock a different file.
	return file, nil
}
