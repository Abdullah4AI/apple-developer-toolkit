package iosbuild

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/rootfs"
	"howett.net/plist"
)

// BuildOptions selects an installed xtool package and a create-only artifact destination.
type BuildOptions struct {
	PackagePath, Product, AppPath, Platform, Configuration string
	LogWriter                                              io.Writer
}

// ChildEnvironment passes compiler children only process, locale, proxy and
// toolchain settings, so credentials in the caller's environment never reach them.
func ChildEnvironment() []string {
	var result []string
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if childEnvironmentAllowed(key) {
			result = append(result, value)
		}
	}
	return result
}

func childEnvironmentAllowed(key string) bool {
	switch key {
	case "PATH", "HOME", "USER", "LOGNAME", "SHELL", "TMPDIR", "TMP", "TEMP", "TERM", "TZ", "LANG", "SYSTEMROOT",
		"SDKROOT", "DEVELOPER_DIR", "TOOLCHAINS",
		"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "no_proxy",
		"SSL_CERT_FILE", "SSL_CERT_DIR", "SSH_AUTH_SOCK":
		return true
	}
	for _, prefix := range []string{"LC_", "XDG_", "XTL_", "XTOOL_", "SWIFT_", "SWIFTPM_"} {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

// RunTool sends both compiler output streams to diagnostics, never receipt stdout.
func RunTool(ctx context.Context, directory string, logs io.Writer, executable string, args ...string) error {
	return runTool(ctx, directory, logs, ChildEnvironment(), nil, executable, args...)
}

// RunSigner restricts signer children to basic process and locale settings.
// Each input is readable by the child at /dev/fd/3, /dev/fd/4, and so on,
// so secrets reach it through pipes instead of files.
func RunSigner(ctx context.Context, directory string, logs io.Writer, inputs [][]byte, executable string, args ...string) error {
	var env []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		switch key {
		case "PATH", "HOME", "TMPDIR", "TMP", "TEMP", "LANG", "LC_ALL", "SYSTEMROOT":
			env = append(env, entry)
		}
	}
	return runTool(ctx, directory, logs, env, inputs, executable, args...)
}

func runTool(ctx context.Context, directory string, logs io.Writer, env []string, inputs [][]byte, executable string, args ...string) error {
	command := exec.CommandContext(ctx, executable, args...)
	command.Dir = directory
	command.Env = env
	configureProcessCancellation(command)
	command.WaitDelay = 5 * time.Second
	command.Stdout = logs
	command.Stderr = logs
	writers := make([]*os.File, 0, len(inputs))
	defer func() {
		for _, file := range append(command.ExtraFiles, writers...) {
			_ = file.Close()
		}
	}()
	for range inputs {
		reader, writer, err := os.Pipe()
		if err != nil {
			return err
		}
		command.ExtraFiles = append(command.ExtraFiles, reader)
		writers = append(writers, writer)
	}
	err := command.Start()
	if err == nil {
		for _, reader := range command.ExtraFiles {
			_ = reader.Close()
		}
		var feeding sync.WaitGroup
		for i, writer := range writers {
			feeding.Go(func() {
				_, _ = writer.Write(inputs[i])
				_ = writer.Close()
			})
		}
		err = command.Wait()
		feeding.Wait()
	}
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%s failed: %w", filepath.Base(executable), err)
	}
	return nil
}

var afterAppReservedForTest func()

// Build compiles using xtool, fixes simulator metadata and signs simulator output ad hoc.
func Build(ctx context.Context, opts BuildOptions) (result *asc.IOSArtifactResult, resultErr error) {
	ctx, stop := ContextWithSignals(ctx)
	defer stop()
	started := time.Now()
	// xtool can apply ad hoc entitlements while compiling a device app even
	// without --sign. Successful compilation does not establish a signing type.
	result = &asc.IOSArtifactResult{Operation: "compile", Backend: "xtool", Platform: opts.Platform, Configuration: opts.Configuration, SigningType: "unknown", AppleAcceptance: "notVerified"}
	defer func() { result.DurationMs = time.Since(started).Milliseconds() }()
	tools := []string{"xtool"}
	if opts.Platform == "simulator" {
		tools = append(tools, "rcodesign")
	}
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			return result, fmt.Errorf("%s is not installed: %w", tool, err)
		}
	}
	packagePath, err := filepath.Abs(opts.PackagePath)
	if err != nil {
		return result, err
	}
	destination, err := filepath.Abs(opts.AppPath)
	if err != nil {
		return result, err
	}
	parent, err := rootfs.New(filepath.Dir(destination))
	if err != nil {
		return result, err
	}
	defer parent.Close()
	if err := parent.MkdirAll(".", 0o755); err != nil {
		return result, fmt.Errorf("create app destination parent: %w", err)
	}
	parentOS, err := parent.OpenRoot()
	if err != nil {
		return result, err
	}
	defer parentOS.Close()
	if _, err := parentOS.Lstat(filepath.Base(destination)); !errors.Is(err, os.ErrNotExist) {
		return result, fmt.Errorf("app destination must not exist")
	}
	packageRoot, err := rootfs.New(packagePath)
	if err != nil {
		return result, err
	}
	defer packageRoot.Close()
	if _, err := packageRoot.ReadFileLimited("Package.swift", 4<<20); err != nil {
		return result, fmt.Errorf("read prepared Package.swift: %w", err)
	}
	packageOS, err := packageRoot.OpenRoot()
	if err != nil {
		return result, err
	}
	defer packageOS.Close()
	lock, err := acquireBuildLock(packageOS)
	if err != nil {
		return result, fmt.Errorf("lock prepared package: %w", err)
	}
	defer func() {
		if err := errors.Join(unlockBuildFile(lock), lock.Close()); err != nil {
			result.Success = false
			resultErr = errors.Join(resultErr, fmt.Errorf("release package build lock: %w", err))
		}
	}()
	productPath := "xtool/" + opts.Product + ".app"
	// Pin an existing output's identity so a different configured product or a
	// successful no-op cannot cause publication of the previous build.
	var previousInfo os.FileInfo
	previous, err := packageOS.Open(productPath)
	if err == nil {
		defer previous.Close()
		previousInfo, err = previous.Stat()
		if err != nil {
			return result, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return result, err
	}
	triple := "arm64-apple-ios"
	if opts.Platform == "simulator" {
		triple = "arm64-apple-ios-simulator"
	}
	if err := RunTool(ctx, packagePath, opts.LogWriter, "xtool", "dev", "build", "--configuration", opts.Configuration, "--triple", triple); err != nil {
		return result, err
	}
	produced, err := packageOS.Lstat(productPath)
	if err != nil {
		return result, fmt.Errorf("read compiled product: %w", err)
	}
	if !produced.IsDir() || produced.Mode()&os.ModeSymlink != 0 {
		return result, fmt.Errorf("compiled app must be a real directory")
	}
	if previousInfo != nil && os.SameFile(previousInfo, produced) {
		return result, fmt.Errorf("xtool did not replace requested product; refusing stale app")
	}
	source := filepath.Join(packagePath, "xtool", opts.Product+".app")
	// Validate and sign a private copy before reserving the output directory.
	stage, err := os.MkdirTemp(filepath.Dir(destination), ".asc-ios-build-")
	if err != nil {
		return result, err
	}
	defer func() {
		if err := os.RemoveAll(stage); err != nil {
			result.Success = false
			resultErr = errors.Join(resultErr, fmt.Errorf("clean build stage %s failed; inspect output before retry: %w", stage, err))
		}
	}()
	stagedApp := filepath.Join(stage, filepath.Base(destination))
	if err := CopyBundle(ctx, source, stagedApp); err != nil {
		return result, err
	}
	info, err := ReadAppInfo(stagedApp)
	if err != nil {
		return result, err
	}
	if opts.Platform == "simulator" {
		if err := ValidateAppCode(stagedApp, info, 7); err != nil {
			return result, err
		}
		info["CFBundleSupportedPlatforms"] = []string{"iPhoneSimulator"}
		data, err := plist.Marshal(info, plist.BinaryFormat)
		if err != nil {
			return result, err
		}
		root, err := rootfs.New(stagedApp)
		if err != nil {
			return result, err
		}
		err = root.WriteFile("Info.plist", data, 0o644)
		root.Close()
		if err != nil {
			return result, err
		}
		if err := RunSigner(ctx, stage, opts.LogWriter, nil, "rcodesign", "--config-file", os.DevNull, "sign", "--timestamp-url", "none", stagedApp); err != nil {
			return result, err
		}
		result.SigningType = "adHoc"
	} else if err := ValidateDeviceApp(stagedApp, info); err != nil {
		return result, err
	}
	// Reserve the exact destination before copying. On failure it remains visibly
	// incomplete with no success receipt; callers can inspect it before retrying.
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := parentOS.Mkdir(filepath.Base(destination), 0o755); err != nil {
		return result, err
	}
	if afterAppReservedForTest != nil {
		afterAppReservedForTest()
	}
	if err := copyBundleInto(ctx, stagedApp, parent, filepath.Base(destination)); err != nil {
		return result, fmt.Errorf("app publication incomplete at %s: %w", destination, err)
	}
	if err := ctx.Err(); err != nil {
		return result, fmt.Errorf("app created at %s; inspect before retry: %w", destination, err)
	}
	result.AppPath = destination
	result.BundleID = info["CFBundleIdentifier"].(string)
	result.Success = true
	return result, nil
}
