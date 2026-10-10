package iosbuild

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"howett.net/plist"
)

func TestBuildRejectsConcurrentCompileInSamePackage(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("POSIX compiler fixture")
	}
	directory := t.TempDir()
	prepared := filepath.Join(directory, "package")
	writeDeviceFixture(t, filepath.Join(prepared, "fixture.app"), 2)
	if err := os.WriteFile(filepath.Join(prepared, "Package.swift"), []byte("// prepared"), 0o644); err != nil {
		t.Fatal(err)
	}
	tools := filepath.Join(directory, "bin")
	if err := os.Mkdir(tools, 0o755); err != nil {
		t.Fatal(err)
	}
	script := `#!/bin/sh
/bin/mkdir -p xtool
/bin/rm -rf xtool/App.app
/bin/cp -R fixture.app xtool/App.app
printf '%s' "$4" > xtool/App.app/configuration
if test "$4" = debug; then
  : > ready
  while ! test -f release; do /bin/sleep 0.01; done
fi
`
	if err := os.WriteFile(filepath.Join(tools, "xtool"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tools)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	opts := BuildOptions{PackagePath: prepared, Product: "App", AppPath: filepath.Join(directory, "debug.app"), Platform: "device", Configuration: "debug", LogWriter: io.Discard}
	done := make(chan error, 1)
	go func() {
		_, err := Build(ctx, opts)
		done <- err
	}()
	for {
		if _, err := os.Stat(filepath.Join(prepared, "ready")); err == nil {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("first compiler did not start: %v", err)
		case <-ctx.Done():
			t.Fatal("first compiler did not become ready")
		case <-time.After(10 * time.Millisecond):
		}
	}
	second := opts
	second.Configuration = "release"
	second.AppPath = filepath.Join(directory, "release.app")
	_, concurrentErr := Build(ctx, second)
	if err := os.WriteFile(filepath.Join(prepared, "release"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("first compile failed: %v", err)
	}
	if concurrentErr == nil || !strings.Contains(concurrentErr.Error(), "already running") {
		t.Fatalf("overlapping compile was not rejected: %v", concurrentErr)
	}
	configuration, err := os.ReadFile(filepath.Join(opts.AppPath, "configuration"))
	if err != nil || string(configuration) != "debug" {
		t.Fatalf("wrong invocation's artifact published: %q, %v", configuration, err)
	}
	if _, err := Build(ctx, second); err != nil {
		t.Fatalf("package remained locked after completion: %v", err)
	}
}

func writeDeviceFixture(t *testing.T, directory string, platform uint32) {
	t.Helper()
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := plist.Marshal(map[string]any{"CFBundleIdentifier": "com.example.app", "CFBundleExecutable": "App", "CFBundleShortVersionString": "1.0", "CFBundleVersion": "1", "CFBundleSupportedPlatforms": []string{"iPhoneOS"}}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "Info.plist"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	// A minimal actual Mach-O header plus LC_BUILD_VERSION, not a mocked parser.
	header := []uint32{0xfeedfacf, 0x100000c, 0, 2, 1, 24, 0, 0, 0x32, 24, platform, 0x120000, 0x1b0000, 0}
	var executable bytes.Buffer
	for _, value := range header {
		if err := binary.Write(&executable, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(directory, "App"), executable.Bytes(), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestBuildActualChildFailureCannotPublishStaleApp(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("POSIX shell failure fixture")
	}
	directory := t.TempDir()
	prepared := filepath.Join(directory, "package")
	if err := os.MkdirAll(prepared, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(prepared, "Package.swift"), []byte("// prepared"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeDeviceFixture(t, filepath.Join(prepared, "xtool/App.app"), 2)
	tools := filepath.Join(directory, "bin")
	if err := os.Mkdir(tools, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nprintf 'intentional compiler failure\\n' >&2\nexit 17\n"
	if err := os.WriteFile(filepath.Join(tools, "xtool"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tools)
	var logs bytes.Buffer
	destination := filepath.Join(directory, "new-parent", "nested", "output.app")
	result, err := Build(context.Background(), BuildOptions{PackagePath: prepared, Product: "App", AppPath: destination, Platform: "device", Configuration: "release", LogWriter: &logs})
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 17 {
		t.Fatalf("wrong child failure: %v", err)
	}
	if result.Success || result.Operation != "compile" {
		t.Fatalf("wrong receipt: %+v", result)
	}
	if !strings.Contains(logs.String(), "intentional compiler failure") {
		t.Fatalf("missing compiler diagnostics: %q", logs.String())
	}
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale artifact published: %v", err)
	}
}

func TestBuildWithoutToolsLeavesNoOutputOrLock(t *testing.T) {
	directory := t.TempDir()
	prepared := filepath.Join(directory, "package")
	if err := os.MkdirAll(prepared, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(prepared, "Package.swift"), []byte("// prepared"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	parent := filepath.Join(directory, "new-parent")
	_, err := Build(context.Background(), BuildOptions{PackagePath: prepared, Product: "App", AppPath: filepath.Join(parent, "App.app"), Platform: "device", Configuration: "debug", LogWriter: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "xtool") {
		t.Fatalf("missing xtool not reported: %v", err)
	}
	for _, path := range []string{parent, filepath.Join(prepared, ".asc-ios-build.lock")} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s created before tool check: %v", path, err)
		}
	}
}

func TestDeviceValidationRejectsSimulatorAndNestedCode(t *testing.T) {
	for _, tc := range []struct {
		name     string
		platform uint32
		nested   bool
	}{{"simulator", 7, false}, {"nested", 2, true}} {
		t.Run(tc.name, func(t *testing.T) {
			app := filepath.Join(t.TempDir(), "App.app")
			writeDeviceFixture(t, app, tc.platform)
			if tc.nested {
				writeDeviceFixture(t, filepath.Join(app, "Resources/Other.bundle"), 2)
			}
			info, err := ReadAppInfo(app)
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateDeviceApp(app, info); err == nil {
				t.Fatal("unsupported code accepted")
			}
		})
	}
}

func TestCopyBundleRejectsSymlinkAndPreservesExecutableMode(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("requires POSIX iOS executable permissions")
	}
	directory := t.TempDir()
	source := filepath.Join(directory, "App.app")
	writeDeviceFixture(t, source, 2)
	destination := filepath.Join(directory, "copy.app")
	if err := CopyBundle(context.Background(), source, destination); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(destination, "App"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("executable mode changed: %v", info.Mode())
	}
	if err := os.Symlink(filepath.Join(source, "Info.plist"), filepath.Join(source, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := CopyBundle(context.Background(), source, filepath.Join(directory, "unsafe.app")); err == nil {
		t.Fatal("symlink accepted")
	}
}

func TestChildEnvironmentPassesOnlyToolchainSettings(t *testing.T) {
	for _, name := range []string{"ASC_KEY_ID", "ASC_PRIVATE_KEY", "RCODESIGN_SIGN_PEM_FILE", "AWS_SECRET_ACCESS_KEY", "GITHUB_TOKEN", "LD_PRELOAD"} {
		t.Setenv(name, "must-not-reach-child")
	}
	t.Setenv("PATH", "/toolchain/bin")
	t.Setenv("SWIFT_EXEC", "/compiler/swift")
	env := strings.Join(ChildEnvironment(), "\n")
	if strings.Contains(env, "must-not-reach-child") {
		t.Fatalf("child inherited sensitive settings: %s", env)
	}
	for _, want := range []string{"PATH=/toolchain/bin", "SWIFT_EXEC=/compiler/swift"} {
		if !strings.Contains(env, want) {
			t.Fatalf("toolchain setting %s lost: %s", want, env)
		}
	}
	if err := RunTool(context.Background(), "", io.Discard, "nonexistent-asc-tool-01a11503"); err == nil {
		t.Fatal("missing tool accepted")
	}
}

func TestAppCodeRequiresExecutableAndRejectsMalformedNestedMachO(t *testing.T) {
	for _, kind := range []string{"missing", "nonexecutable", "fat64", "malformed-thin"} {
		t.Run(kind, func(t *testing.T) {
			app := filepath.Join(t.TempDir(), "App.app")
			writeDeviceFixture(t, app, 2)
			switch kind {
			case "missing":
				if err := os.Remove(filepath.Join(app, "App")); err != nil {
					t.Fatal(err)
				}
			case "nonexecutable":
				if err := os.Chmod(filepath.Join(app, "App"), 0o644); err != nil {
					t.Fatal(err)
				}
			default:
				magic := []byte{0xca, 0xfe, 0xba, 0xbf}
				if kind == "malformed-thin" {
					magic = []byte{0xcf, 0xfa, 0xed, 0xfe}
				}
				if err := os.WriteFile(filepath.Join(app, "hidden-code"), magic, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			info, err := ReadAppInfo(app)
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateDeviceApp(app, info); err == nil {
				t.Fatal("unsupported executable accepted")
			}
		})
	}
	app := filepath.Join(t.TempDir(), "Simulator.app")
	writeDeviceFixture(t, app, 2)
	info, err := ReadAppInfo(app)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateAppCode(app, info, 7); err == nil {
		t.Fatal("device executable accepted as simulator")
	}
}

func TestBuildSuccessfulChildCannotPublishUnchangedApp(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("POSIX compiler fixture")
	}
	directory := t.TempDir()
	prepared := filepath.Join(directory, "package")
	if err := os.MkdirAll(prepared, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(prepared, "Package.swift"), []byte("// prepared"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeDeviceFixture(t, filepath.Join(prepared, "xtool/App.app"), 2)
	tools := filepath.Join(directory, "bin")
	if err := os.Mkdir(tools, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tools, "xtool"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tools)
	destination := filepath.Join(directory, "output.app")
	result, err := Build(context.Background(), BuildOptions{PackagePath: prepared, Product: "App", AppPath: destination, Platform: "device", Configuration: "release", LogWriter: io.Discard})
	if err == nil || result.Success {
		t.Fatalf("stale product published: %+v %v", result, err)
	}
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale output exists: %v", err)
	}
}

func TestBuildDeviceDoesNotClaimAnUnverifiedSigningType(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("POSIX compiler fixture")
	}
	directory := t.TempDir()
	prepared := filepath.Join(directory, "package")
	writeDeviceFixture(t, filepath.Join(prepared, "fixture.app"), 2)
	if err := os.WriteFile(filepath.Join(prepared, "Package.swift"), []byte("// prepared"), 0o644); err != nil {
		t.Fatal(err)
	}
	tools := filepath.Join(directory, "bin")
	if err := os.Mkdir(tools, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n/bin/mkdir -p xtool\n/bin/cp -R fixture.app xtool/App.app\n"
	if err := os.WriteFile(filepath.Join(tools, "xtool"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tools)
	result, err := Build(context.Background(), BuildOptions{PackagePath: prepared, Product: "App", AppPath: filepath.Join(directory, "output.app"), Platform: "device", Configuration: "release", LogWriter: io.Discard})
	if err != nil || !result.Success {
		t.Fatalf("compile failed: %+v %v", result, err)
	}
	if result.SigningType != "unknown" || result.SignatureVerified || result.ProfileValidated || result.AppleAcceptance != "notVerified" {
		t.Fatalf("compile must not infer a signing type from successful tool execution: %+v", result)
	}
}

func TestBundleWalksBoundResourceFileDescriptors(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("requires POSIX iOS executable permissions")
	}
	directory := t.TempDir()
	app := filepath.Join(directory, "App.app")
	writeDeviceFixture(t, app, 2)
	resources := filepath.Join(app, "Resources")
	if err := os.Mkdir(resources, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := range 512 {
		if err := os.WriteFile(filepath.Join(resources, fmt.Sprintf("%04d.txt", i)), []byte("resource"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	info, err := ReadAppInfo(app)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateDeviceApp(app, info); err != nil {
		t.Fatal(err)
	}
	copy := filepath.Join(directory, "Copy.app")
	if err := CopyBundle(context.Background(), app, copy); err != nil {
		t.Fatal(err)
	}
	if err := WriteIPA(context.Background(), copy, filepath.Join(directory, "App.ipa")); err != nil {
		t.Fatal(err)
	}
}

func TestBuildPublishesOnlyIntoReservedDirectory(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("POSIX compiler fixture")
	}
	directory := t.TempDir()
	prepared := filepath.Join(directory, "package")
	writeDeviceFixture(t, filepath.Join(prepared, "fixture.app"), 2)
	if err := os.WriteFile(filepath.Join(prepared, "Package.swift"), []byte("// prepared"), 0o644); err != nil {
		t.Fatal(err)
	}
	tools := filepath.Join(directory, "bin")
	if err := os.Mkdir(tools, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n/bin/mkdir -p xtool\n/bin/cp -R fixture.app xtool/App.app\n"
	if err := os.WriteFile(filepath.Join(tools, "xtool"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tools)
	output := filepath.Join(directory, "output")
	outside := filepath.Join(directory, "outside")
	for _, dir := range []string{output, outside} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	afterAppReservedForTest = func() {
		if err := os.Rename(output, filepath.Join(directory, "moved")); err != nil {
			t.Error(err)
		}
		if err := os.Symlink(outside, output); err != nil {
			t.Error(err)
		}
		stages, err := filepath.Glob(filepath.Join(directory, "moved", ".asc-ios-build-*"))
		if err != nil || len(stages) != 1 {
			t.Fatalf("build stage not found: %v %v", stages, err)
		}
		if err := os.Rename(stages[0], filepath.Join(outside, filepath.Base(stages[0]))); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(func() { afterAppReservedForTest = nil })
	result, err := Build(context.Background(), BuildOptions{PackagePath: prepared, Product: "App", AppPath: filepath.Join(output, "App.app"), Platform: "device", Configuration: "release", LogWriter: io.Discard})
	if err == nil || result.Success {
		t.Fatalf("publication followed a swapped parent: %+v", result)
	}
	if _, err := os.Lstat(filepath.Join(outside, "App.app")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("app written outside the selected directory: %v", err)
	}
}
