// Package iosbuild provides local xtool compilation and bounded iOS bundle staging.
package iosbuild

import (
	"archive/zip"
	"context"
	"debug/macho"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/infoplist"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/rootfs"
	"howett.net/plist"
)

const (
	maxBundleBytes   int64 = 4 << 30
	maxBundleEntries       = 100000
)

// CopyBundle copies regular files and directories without following bundle symlinks.
func CopyBundle(ctx context.Context, source, destination string) error {
	dst, err := rootfs.New(destination)
	if err != nil {
		return err
	}
	defer dst.Close()
	return copyBundleInto(ctx, source, dst, ".")
}

// copyBundleInto writes below an already selected root so path swaps fail closed.
func copyBundleInto(ctx context.Context, source string, dst rootfs.Root, prefix string) error {
	src, err := rootfs.New(source)
	if err != nil {
		return err
	}
	defer src.Close()
	rooted, err := src.OpenRoot()
	if err != nil {
		return err
	}
	defer rooted.Close()
	var total int64
	entries := 0
	return fs.WalkDir(rooted.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		entries++
		if entries > maxBundleEntries {
			return fmt.Errorf("app exceeds entry limit")
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("app symlinks are unsupported: %s", name)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return dst.MkdirAll(filepath.Join(prefix, name), 0o755)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported app file: %s", name)
		}
		total += info.Size()
		if total > maxBundleBytes {
			return fmt.Errorf("app exceeds size limit")
		}
		file, err := src.OpenFile(name)
		if err != nil {
			return err
		}
		_, err = dst.CreateNewFrom(filepath.Join(prefix, name), io.LimitReader(file, info.Size()+1), info.Mode().Perm()&0o755)
		return errors.Join(err, file.Close())
	})
}

// ReadAppInfo validates bounded plist input before interpreting application metadata.
func ReadAppInfo(app string) (map[string]any, error) {
	root, err := rootfs.New(app)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	data, err := root.ReadFileLimited("Info.plist", infoplist.MaxBytes)
	if err != nil {
		return nil, err
	}
	if err := infoplist.ValidateStructure(data); err != nil {
		return nil, err
	}
	var info map[string]any
	if _, err := plist.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	for _, key := range []string{"CFBundleIdentifier", "CFBundleExecutable", "CFBundleShortVersionString", "CFBundleVersion"} {
		if value, ok := info[key].(string); !ok || strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("app is missing %s", key)
		}
	}
	executable := info["CFBundleExecutable"].(string)
	if filepath.Base(executable) != executable || executable == "." || strings.ContainsAny(executable, `/\`) {
		return nil, fmt.Errorf("app executable must be a filename")
	}
	return info, nil
}

// ValidateDeviceApp accepts one thin arm64 iOS executable and resource bundles.
// Nested signing targets require their own profiles and are intentionally unsupported.
func ValidateDeviceApp(app string, info map[string]any) error {
	platforms, ok := info["CFBundleSupportedPlatforms"].([]any)
	if !ok || len(platforms) != 1 || platforms[0] != "iPhoneOS" {
		return fmt.Errorf("package requires an iPhoneOS device app")
	}
	return ValidateAppCode(app, info, 2)
}

// ValidateAppCode checks the actual binary platform before metadata or signing changes.
func ValidateAppCode(app string, info map[string]any, platform uint32) error {
	root, err := rootfs.New(app)
	if err != nil {
		return err
	}
	defer root.Close()
	rooted, err := root.OpenRoot()
	if err != nil {
		return err
	}
	defer rooted.Close()
	executable := info["CFBundleExecutable"].(string)
	found := false
	err = fs.WalkDir(rooted.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("app symlinks are unsupported")
		}
		if name != "." && entry.IsDir() {
			switch strings.ToLower(filepath.Ext(name)) {
			case ".app", ".appex", ".framework", ".xpc":
				return fmt.Errorf("nested signing targets are unsupported: %s", name)
			}
		}
		if entry.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(name), ".dylib") {
			return fmt.Errorf("nested dylibs are unsupported")
		}
		f, err := root.OpenFile(name)
		if err != nil {
			return err
		}
		if name == executable {
			found = true
		}
		return validateAppFile(f, name, executable, platform)
	})
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("app executable is missing")
	}
	return nil
}

// validateAppFile owns and closes exactly one file before the walk continues.
func validateAppFile(f *os.File, name, executable string, platform uint32) (resultErr error) {
	defer func() { resultErr = errors.Join(resultErr, f.Close()) }()
	binary, parseErr := macho.NewFile(f)
	if name != executable {
		if parseErr == nil {
			return fmt.Errorf("nested Mach-O code is unsupported: %s", name)
		}
		var magic [4]byte
		if _, err := f.ReadAt(magic[:], 0); err == nil && isMachOMagic(magic) {
			return fmt.Errorf("nested Mach-O code is unsupported")
		}
		return nil
	}
	fileInfo, err := f.Stat()
	if err != nil {
		return err
	}
	if !fileInfo.Mode().IsRegular() || fileInfo.Mode().Perm()&0o100 == 0 {
		return fmt.Errorf("app executable must be a regular executable file")
	}
	if parseErr != nil {
		return fmt.Errorf("app executable must be a thin Mach-O: %w", parseErr)
	}
	if binary.Cpu != macho.CpuArm64 || binary.Type != macho.TypeExec {
		return fmt.Errorf("app executable must be arm64 MH_EXECUTE")
	}
	device := false
	for _, load := range binary.Loads {
		data := load.Raw()
		if len(data) >= 12 && binary.ByteOrder.Uint32(data[:4]) == 0x32 {
			if binary.ByteOrder.Uint32(data[8:12]) != platform {
				return fmt.Errorf("Mach-O platform does not match requested target")
			}
			device = true
		}
	}
	if !device {
		return fmt.Errorf("app executable lacks the requested LC_BUILD_VERSION")
	}
	return nil
}

func isMachOMagic(magic [4]byte) bool {
	switch magic {
	case [4]byte{0xca, 0xfe, 0xba, 0xbe}, [4]byte{0xbe, 0xba, 0xfe, 0xca},
		[4]byte{0xca, 0xfe, 0xba, 0xbf}, [4]byte{0xbf, 0xba, 0xfe, 0xca},
		[4]byte{0xfe, 0xed, 0xfa, 0xce}, [4]byte{0xce, 0xfa, 0xed, 0xfe},
		[4]byte{0xfe, 0xed, 0xfa, 0xcf}, [4]byte{0xcf, 0xfa, 0xed, 0xfe}:
		return true
	}
	return false
}

// WriteIPA packages the privately staged bundle without following symlinks.
func WriteIPA(ctx context.Context, app, output string) error {
	root, err := rootfs.New(app)
	if err != nil {
		return err
	}
	defer root.Close()
	rooted, err := root.OpenRoot()
	if err != nil {
		return err
	}
	defer rooted.Close()
	file, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(file)
	var total int64
	entries := 0
	walkErr := fs.WalkDir(rooted.FS(), ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		entries++
		if entries > maxBundleEntries {
			return fmt.Errorf("app exceeds entry limit")
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("app symlinks are unsupported")
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported app file")
		}
		total += info.Size()
		if total > maxBundleBytes {
			return fmt.Errorf("app exceeds size limit")
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = "Payload/" + filepath.Base(app) + "/" + filepath.ToSlash(name)
		header.Method = zip.Deflate
		out, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		in, err := root.OpenFile(name)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, io.LimitReader(in, info.Size()+1))
		return errors.Join(err, in.Close())
	})
	return errors.Join(walkErr, writer.Close(), file.Close())
}
