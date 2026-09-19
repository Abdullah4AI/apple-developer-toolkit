package rootfs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/secureopen"
)

type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) {
	return 0, r.err
}

func TestValidateRelativeRejectsEscapes(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"empty", ""},
		{"unix absolute", "/etc/passwd"},
		{"windows absolute backslash", `\Windows\System32`},
		{"windows drive absolute", `C:\Windows\System32`},
		{"windows drive relative", "C:evil.txt"},
		{"unc share", `\\attacker\share\evil.txt`},
		{"parent traversal", "../evil.txt"},
		{"nested parent traversal", "a/../../evil.txt"},
		{"bare parent", ".."},
		{"backslash parent traversal", `..\evil.txt`},
		{"mixed separator traversal", `a\..\..\evil.txt`},
		{"nul byte", "a\x00b"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := ValidateRelative(testCase.path); err == nil {
				t.Fatalf("ValidateRelative(%q) = nil, want error", testCase.path)
			}
		})
	}
}

func TestValidateRelativeAcceptsOrdinaryPaths(t *testing.T) {
	cases := []string{
		"file.txt",
		"metadata/en-US/description.txt",
		"./file.txt",
		"a..b/c.txt",
		"...hidden",
	}

	for _, path := range cases {
		if err := ValidateRelative(path); err != nil {
			t.Fatalf("ValidateRelative(%q) error = %v, want nil", path, err)
		}
	}
}

func TestResolveRejectsEscapingAbsolutePath(t *testing.T) {
	root := mustRoot(t, t.TempDir())
	outside := filepath.Join(t.TempDir(), "sentinel.txt")

	if _, err := root.Resolve(outside); !errors.Is(err, ErrEscapesRoot) {
		t.Fatalf("Resolve(%q) error = %v, want ErrEscapesRoot", outside, err)
	}
}

func TestResolveAcceptsAbsolutePathInsideRoot(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	inside := filepath.Join(dir, "nested", "file.txt")

	resolved, err := root.Resolve(inside)
	if err != nil {
		t.Fatalf("Resolve(%q) error = %v", inside, err)
	}
	if resolved != filepath.Clean(inside) {
		t.Fatalf("Resolve(%q) = %q, want %q", inside, resolved, filepath.Clean(inside))
	}
}

func TestRootPreservesWhitespaceInValidPathNames(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Win32 path APIs strip trailing spaces and dots from file names")
	}
	parent := t.TempDir()
	dir := filepath.Join(parent, "trusted ")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}

	root := mustRoot(t, dir)
	if root.Path() != dir {
		t.Fatalf("Path() = %q, want exact path %q", root.Path(), dir)
	}
	if err := root.WriteFile("report ", []byte("exact"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if got := mustRead(t, filepath.Join(dir, "report ")); got != "exact" {
		t.Fatalf("content = %q, want exact whitespace-bearing destination", got)
	}
	if _, err := os.Lstat(filepath.Join(dir, "report")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("trimmed sibling exists or returned unexpected error: %v", err)
	}
	if err := root.WriteFile("   ", []byte("all spaces"), 0o600); err != nil {
		t.Fatalf("WriteFile(all-spaces) error = %v", err)
	}
	if got := mustRead(t, filepath.Join(dir, "   ")); got != "all spaces" {
		t.Fatalf("all-space filename content = %q", got)
	}

	allSpaceDir := filepath.Join(parent, "   ")
	if err := os.Mkdir(allSpaceDir, 0o755); err != nil {
		t.Fatalf("Mkdir(all-space root) error = %v", err)
	}
	allSpaceRoot := mustRoot(t, allSpaceDir)
	if allSpaceRoot.Path() != allSpaceDir {
		t.Fatalf("all-space Path() = %q, want %q", allSpaceRoot.Path(), allSpaceDir)
	}
}

func TestReadFileRefusesSymlinkedFinalComponent(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	secretDir := t.TempDir()
	secretPath := filepath.Join(secretDir, "secret.txt")
	mustWrite(t, secretPath, "top-secret")

	linkPath := filepath.Join(dir, "description.txt")
	if err := os.Symlink(secretPath, linkPath); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	root := mustRoot(t, dir)
	if _, err := root.ReadFile("description.txt"); !errors.Is(err, ErrSymlink) {
		t.Fatalf("ReadFile() error = %v, want ErrSymlink", err)
	}
}

func TestResolveContainedFinalSymlinkAcceptsOnlyInRootTarget(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "actual.md"), "inside")
	if err := os.Symlink("actual.md", filepath.Join(dir, "ASC.md")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
	root := mustRoot(t, dir)
	resolved, err := root.ResolveContainedFinalSymlink("ASC.md")
	if err != nil {
		t.Fatalf("ResolveContainedFinalSymlink() error = %v", err)
	}
	if resolved != "actual.md" {
		t.Fatalf("ResolveContainedFinalSymlink() = %q, want actual.md", resolved)
	}

	outside := filepath.Join(t.TempDir(), "outside.md")
	mustWrite(t, outside, "outside")
	if err := os.Symlink(outside, filepath.Join(dir, "external.md")); err != nil {
		t.Fatalf("Symlink(external) error = %v", err)
	}
	if _, err := root.ResolveContainedFinalSymlink("external.md"); !errors.Is(err, ErrEscapesRoot) {
		t.Fatalf("external ResolveContainedFinalSymlink() error = %v, want ErrEscapesRoot", err)
	}
}

func TestReadFileRefusesSymlinkedParentComponent(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	secretDir := t.TempDir()
	mustWrite(t, filepath.Join(secretDir, "secret.txt"), "top-secret")

	if err := os.Symlink(secretDir, filepath.Join(dir, "en-US")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	root := mustRoot(t, dir)
	if _, err := root.ReadFile(filepath.Join("en-US", "secret.txt")); !errors.Is(err, ErrSymlink) {
		t.Fatalf("ReadFile() error = %v, want ErrSymlink", err)
	}
}

func TestOpenFileRefusesSymlinkedParentComponent(t *testing.T) {
	requireSymlinks(t)

	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks(temp dir) error = %v", err)
	}
	realDir := filepath.Join(dir, "real")
	mustWrite(t, filepath.Join(realDir, "devices.json"), `{"schemaVersion":1}`)
	if err := os.Symlink(realDir, filepath.Join(dir, "linked")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	if _, err := OpenFile(filepath.Join(dir, "linked", "devices.json")); !errors.Is(err, ErrSymlink) {
		t.Fatalf("OpenFile() error = %v, want ErrSymlink", err)
	}
}

func TestReadFileReadsOrdinaryFile(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "keywords.txt"), "one,two")

	root := mustRoot(t, dir)
	data, err := root.ReadFile("keywords.txt")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "one,two" {
		t.Fatalf("ReadFile() = %q, want %q", data, "one,two")
	}
}

func TestReadFileDoesNotEscapeWhenParentIsSwappedAfterValidation(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	mustWrite(t, filepath.Join(nested, "report.txt"), "trusted")
	external := t.TempDir()
	mustWrite(t, filepath.Join(external, "report.txt"), "external-secret")

	root := mustRoot(t, dir)
	root.afterValidationForTest = func() {
		if err := os.Rename(nested, filepath.Join(dir, "nested-original")); err != nil {
			t.Fatalf("swap validated parent: %v", err)
		}
		if err := os.Symlink(external, nested); err != nil {
			t.Fatalf("replace validated parent with symlink: %v", err)
		}
	}

	data, err := root.ReadFile(filepath.Join("nested", "report.txt"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "trusted" {
		t.Fatalf("ReadFile() = %q, want the file anchored before the parent swap", data)
	}
}

func TestReadFileRejectsFinalSymlinkSwappedAfterValidation(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	report := filepath.Join(dir, "report.txt")
	mustWrite(t, report, "trusted")
	mustWrite(t, filepath.Join(dir, "secret.txt"), "in-root-secret")

	root := mustRoot(t, dir)
	root.afterValidationForTest = func() {
		if err := os.Remove(report); err != nil {
			t.Fatalf("remove validated file: %v", err)
		}
		if err := os.Symlink("secret.txt", report); err != nil {
			t.Fatalf("replace validated file with symlink: %v", err)
		}
	}

	data, err := root.ReadFile("report.txt")
	if err == nil {
		t.Fatalf("ReadFile() returned %q through a swapped final symlink, want an error", data)
	}
	if string(data) == "in-root-secret" {
		t.Fatal("ReadFile() disclosed the swapped symlink target")
	}
}

func TestReadFileOptionalReportsMissingWithoutError(t *testing.T) {
	root := mustRoot(t, t.TempDir())

	data, found, err := root.ReadFileOptional("missing.txt")
	if err != nil {
		t.Fatalf("ReadFileOptional() error = %v", err)
	}
	if found {
		t.Fatal("ReadFileOptional() found = true, want false")
	}
	if len(data) != 0 {
		t.Fatalf("ReadFileOptional() data = %q, want empty", data)
	}
}

func TestWriteFileRefusesFinalSymlink(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	sentinelDir := t.TempDir()
	sentinelPath := filepath.Join(sentinelDir, "sentinel.txt")
	mustWrite(t, sentinelPath, "original")

	if err := os.Symlink(sentinelPath, filepath.Join(dir, "out.json")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	root := mustRoot(t, dir)
	if err := root.WriteFile("out.json", []byte("attacker"), 0o600); !errors.Is(err, ErrSymlink) {
		t.Fatalf("WriteFile() error = %v, want ErrSymlink", err)
	}
	if got := mustRead(t, sentinelPath); got != "original" {
		t.Fatalf("sentinel content = %q, want %q", got, "original")
	}
}

func TestCheckWriteFileMatchesWriteFileConstraints(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)

	if err := root.CheckWriteFile(filepath.Join("nested", "missing.pdf")); err != nil {
		t.Fatalf("CheckWriteFile(missing) error = %v, want nil", err)
	}

	readOnly := filepath.Join(dir, "readonly.pdf")
	mustWrite(t, readOnly, "original")
	if err := os.Chmod(readOnly, 0o400); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}
	if err := root.CheckWriteFile("readonly.pdf"); err != nil {
		t.Fatalf("CheckWriteFile(read-only regular file) error = %v, want nil because WriteFile replaces by rename", err)
	}
	if err := root.WriteFile("readonly.pdf", []byte("replaced"), 0o600); err != nil {
		t.Fatalf("WriteFile(read-only regular file) error = %v", err)
	}

	if err := os.Mkdir(filepath.Join(dir, "somedir"), 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	if err := root.CheckWriteFile("somedir"); err == nil {
		t.Fatal("CheckWriteFile(directory) error = nil, want not-a-regular-file error")
	}
	if err := root.CheckWriteFile("."); !errors.Is(err, ErrEscapesRoot) {
		t.Fatalf("CheckWriteFile(root) error = %v, want ErrEscapesRoot", err)
	}
}

func TestCheckWriteFileRefusesSymlinks(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	sentinelDir := t.TempDir()
	sentinelPath := filepath.Join(sentinelDir, "sentinel.txt")
	mustWrite(t, sentinelPath, "original")
	if err := os.Symlink(sentinelPath, filepath.Join(dir, "out.pdf")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
	if err := os.Symlink(sentinelDir, filepath.Join(dir, "nested")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	root := mustRoot(t, dir)
	if err := root.CheckWriteFile("out.pdf"); !errors.Is(err, ErrSymlink) {
		t.Fatalf("CheckWriteFile(final symlink) error = %v, want ErrSymlink", err)
	}
	if err := root.CheckWriteFile(filepath.Join("nested", "sentinel.txt")); !errors.Is(err, ErrSymlink) {
		t.Fatalf("CheckWriteFile(symlinked parent) error = %v, want ErrSymlink", err)
	}
}

func TestWriteFileRefusesSymlinkedParent(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	sentinelDir := t.TempDir()
	sentinelPath := filepath.Join(sentinelDir, "sentinel.txt")
	mustWrite(t, sentinelPath, "original")

	if err := os.Symlink(sentinelDir, filepath.Join(dir, "nested")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	root := mustRoot(t, dir)
	err := root.WriteFile(filepath.Join("nested", "sentinel.txt"), []byte("attacker"), 0o600)
	if !errors.Is(err, ErrSymlink) {
		t.Fatalf("WriteFile() error = %v, want ErrSymlink", err)
	}
	if got := mustRead(t, sentinelPath); got != "original" {
		t.Fatalf("sentinel content = %q, want %q", got, "original")
	}
}

func TestWriteFileCreatesAndReplacesInRoot(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)

	target := filepath.Join("nested", "deep", "out.json")
	if err := root.WriteFile(target, []byte("first"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if got := mustRead(t, filepath.Join(dir, target)); got != "first" {
		t.Fatalf("content = %q, want %q", got, "first")
	}
	if err := root.WriteFile(target, []byte("second"), 0o600); err != nil {
		t.Fatalf("WriteFile() replace error = %v", err)
	}
	if got := mustRead(t, filepath.Join(dir, target)); got != "second" {
		t.Fatalf("content = %q, want %q", got, "second")
	}

	info, err := os.Lstat(filepath.Join(dir, target))
	if err != nil {
		t.Fatalf("Lstat() error = %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
	if leftovers := temporaryLeftovers(t, filepath.Join(dir, "nested", "deep")); len(leftovers) > 0 {
		t.Fatalf("temporary files left behind: %v", leftovers)
	}
}

func TestWriteFileDoesNotEscapeWhenParentIsSwappedAfterValidation(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatalf("create nested directory: %v", err)
	}
	external := t.TempDir()

	root := mustRoot(t, dir)
	root.afterValidationForTest = func() {
		if err := os.Rename(nested, filepath.Join(dir, "nested-original")); err != nil {
			t.Fatalf("swap validated parent: %v", err)
		}
		if err := os.Symlink(external, nested); err != nil {
			t.Fatalf("replace validated parent with symlink: %v", err)
		}
	}

	err := root.WriteFile(filepath.Join("nested", "out.json"), []byte("attacker"), 0o600)
	if err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, statErr := os.Lstat(filepath.Join(external, "out.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("WriteFile() created a file outside the trusted root: %v", statErr)
	}
	if got := mustRead(t, filepath.Join(dir, "nested-original", "out.json")); got != "attacker" {
		t.Fatalf("WriteFile() content in anchored parent = %q", got)
	}
}

func TestWriteFileDoesNotUsePredictableTemporaryPath(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	sentinelDir := t.TempDir()
	sentinelPath := filepath.Join(sentinelDir, "sentinel.txt")
	mustWrite(t, sentinelPath, "original")

	// A predictable "<target>.tmp" staging path lets a lower-trust checkout
	// pre-create a symlink that redirects the staged write.
	if err := os.Symlink(sentinelPath, filepath.Join(dir, "checkpoint.json.tmp")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	root := mustRoot(t, dir)
	if err := root.WriteFile("checkpoint.json", []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if got := mustRead(t, sentinelPath); got != "original" {
		t.Fatalf("sentinel content = %q, want %q", got, "original")
	}
	if got := mustRead(t, filepath.Join(dir, "checkpoint.json")); got != `{"ok":true}` {
		t.Fatalf("checkpoint content = %q", got)
	}
}

func TestWriteFromWritesReaderContents(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)

	written, err := root.WriteFrom("payload.bin", bytes.NewReader([]byte("payload")), 0o600)
	if err != nil {
		t.Fatalf("WriteFrom() error = %v", err)
	}
	if written != int64(len("payload")) {
		t.Fatalf("WriteFrom() = %d, want %d", written, len("payload"))
	}
	if got := mustRead(t, filepath.Join(dir, "payload.bin")); got != "payload" {
		t.Fatalf("content = %q", got)
	}
}

func TestWriteFilePreservingModeKeepsExistingModeAndDefaultsForNew(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)

	existingPath := filepath.Join(dir, "existing.txt")
	mustWrite(t, existingPath, "old")
	if err := os.Chmod(existingPath, 0o600); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}
	if err := root.WriteFilePreservingMode("existing.txt", []byte("new"), 0o644); err != nil {
		t.Fatalf("WriteFilePreservingMode() error = %v", err)
	}
	info, err := os.Lstat(existingPath)
	if err != nil {
		t.Fatalf("Lstat() error = %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("existing mode = %v, want preserved 0600", info.Mode().Perm())
	}
	if got := mustRead(t, existingPath); got != "new" {
		t.Fatalf("content = %q, want %q", got, "new")
	}

	if err := root.WriteFilePreservingMode("fresh.txt", []byte("data"), 0o640); err != nil {
		t.Fatalf("WriteFilePreservingMode() new file error = %v", err)
	}
	freshInfo, err := os.Lstat(filepath.Join(dir, "fresh.txt"))
	if err != nil {
		t.Fatalf("Lstat() error = %v", err)
	}
	if runtime.GOOS != "windows" && freshInfo.Mode().Perm() != 0o640 {
		t.Fatalf("new mode = %v, want default 0640", freshInfo.Mode().Perm())
	}
}

func TestWriteFilePreservingModeRefusesReadOnlyExistingFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not authoritative on Windows")
	}

	dir := t.TempDir()
	root := mustRoot(t, dir)
	path := filepath.Join(dir, "read-only.txt")
	mustWrite(t, path, "original")
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}

	if err := root.WriteFilePreservingMode("read-only.txt", []byte("replacement"), 0o644); err == nil {
		t.Fatal("WriteFilePreservingMode() error = nil, want read-only destination refusal")
	}
	if got := mustRead(t, path); got != "original" {
		t.Fatalf("content = %q, want original", got)
	}
}

func TestWriteFilePreservingModeRefusesMultiplyLinkedFile(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	path := filepath.Join(dir, "existing.txt")
	linkPath := filepath.Join(dir, "linked.txt")
	mustWrite(t, path, "original")
	if err := os.Link(path, linkPath); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	if err := root.WriteFilePreservingMode("existing.txt", []byte("replacement"), 0o644); err == nil {
		t.Fatal("WriteFilePreservingMode() error = nil, want multiply-linked file refusal")
	}
	if got := mustRead(t, path); got != "original" {
		t.Fatalf("destination content = %q, want original", got)
	}
	if got := mustRead(t, linkPath); got != "original" {
		t.Fatalf("hard-link content = %q, want original", got)
	}
}

func TestWriteFilePreservingModeDoesNotMutateHardLinkAddedAfterValidation(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	path := filepath.Join(dir, "existing.txt")
	externalLink := filepath.Join(t.TempDir(), "external.txt")
	mustWrite(t, path, "original")
	root.afterValidationForTest = func() {
		if err := os.Link(path, externalLink); err != nil {
			t.Skipf("hard links unavailable: %v", err)
		}
	}

	if err := root.WriteFilePreservingMode("existing.txt", []byte("replacement"), 0o644); err != nil {
		t.Fatalf("WriteFilePreservingMode() error = %v", err)
	}
	if got := mustRead(t, path); got != "replacement" {
		t.Fatalf("destination content = %q, want replacement", got)
	}
	if got := mustRead(t, externalLink); got != "original" {
		t.Fatalf("late hard link content = %q, want original", got)
	}
}

func TestWriteFilePreservingModeCreatesMissingIntermediateDirectories(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	name := filepath.Join("nested", "deep", "fresh.txt")

	if err := root.WriteFilePreservingMode(name, []byte("data"), 0o640); err != nil {
		t.Fatalf("WriteFilePreservingMode() error = %v", err)
	}
	if got := mustRead(t, filepath.Join(dir, name)); got != "data" {
		t.Fatalf("content = %q, want %q", got, "data")
	}
}

func TestCreateNewFileRefusesExistingFile(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "AuthKey.p8"), "existing")

	root := mustRoot(t, dir)
	if err := root.CreateNewFile("AuthKey.p8", []byte("new"), 0o600); !errors.Is(err, os.ErrExist) {
		t.Fatalf("CreateNewFile() error = %v, want os.ErrExist", err)
	}
	if got := mustRead(t, filepath.Join(dir, "AuthKey.p8")); got != "existing" {
		t.Fatalf("content = %q, want %q", got, "existing")
	}
}

func TestCreateNewFileAtomicRefusesRacingDestination(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	root.afterValidationForTest = func() {
		mustWrite(t, filepath.Join(dir, "plan.json"), "racing")
	}

	err := root.CreateNewFileAtomic("plan.json", []byte("planned"), 0o600)
	if err == nil {
		t.Fatal("CreateNewFileAtomic() replaced a racing destination")
	}
	if got := mustRead(t, filepath.Join(dir, "plan.json")); got != "racing" {
		t.Fatalf("content = %q, want racing destination preserved", got)
	}
}

func TestCreateNewFileAtomicWritesExactMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not authoritative on Windows")
	}
	dir := t.TempDir()
	root := mustRoot(t, dir)
	if err := root.CreateNewFileAtomic("plan.json", []byte("planned"), 0o600); err != nil {
		t.Fatalf("CreateNewFileAtomic() error = %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 600", got)
	}
}

func TestCreateNewFromClosesStagingBeforePublication(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	path := filepath.Join(dir, "ordinary.txt")
	closeErr := errors.New("injected staging close failure")
	var closeCalls int
	root.closeStagingFileForTest = func(file *os.File) error {
		closeCalls++
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("destination during staging close: err = %v, want absent", err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("close staging file in test seam: %v", err)
		}
		return closeErr
	}

	if _, err := root.CreateNewFrom("ordinary.txt", bytes.NewReader([]byte("complete")), 0o600); !errors.Is(err, closeErr) {
		t.Fatalf("CreateNewFrom() error = %v, want staging close failure", err)
	}
	if closeCalls != 1 {
		t.Fatalf("staging close calls = %d, want 1", closeCalls)
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("destination after staging close failure: %v", err)
	}
}

func TestCreateNewFileAtomicClosesStagingBeforePublication(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	path := filepath.Join(dir, "ordinary-atomic.txt")
	closeErr := errors.New("injected staging close failure")
	var closeCalls int
	root.closeStagingFileForTest = func(file *os.File) error {
		closeCalls++
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("destination during staging close: err = %v, want absent", err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("close staging file in test seam: %v", err)
		}
		return closeErr
	}

	if err := root.CreateNewFileAtomic("ordinary-atomic.txt", []byte("complete"), 0o600); !errors.Is(err, closeErr) {
		t.Fatalf("CreateNewFileAtomic() error = %v, want staging close failure", err)
	}
	if closeCalls != 1 {
		t.Fatalf("staging close calls = %d, want 1", closeCalls)
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("destination after staging close failure: %v", err)
	}
}

func TestCreateNewFileAtomicWithInfoClosesStagingAfterIdentityVerification(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows closes the staging handle before publication")
	}
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "identity.txt")
	closeErr := errors.New("injected staging close failure")
	var closeCalls int
	root.closeStagingFileForTest = func(file *os.File) error {
		closeCalls++
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("published destination during identity close: %v", err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("close staging file in test seam: %v", err)
		}
		return closeErr
	}

	info, err := root.CreateNewFileAtomicWithInfo("identity.txt", []byte("complete"), 0o600)
	if !errors.Is(err, closeErr) {
		t.Fatalf("CreateNewFileAtomicWithInfo() error = %v, want staging close failure", err)
	}
	if closeCalls != 1 {
		t.Fatalf("staging close calls = %d, want 1", closeCalls)
	}
	if info == nil {
		t.Fatal("CreateNewFileAtomicWithInfo() returned nil identity after post-publication close failure")
	}
	diskInfo, statErr := os.Stat(path)
	if statErr != nil {
		t.Fatalf("Stat(identity) error = %v", statErr)
	}
	if !os.SameFile(info, diskInfo) {
		t.Fatal("returned identity does not identify the published file")
	}
}

func TestCreateNewFileAtomicWithInfoClosesStagingBeforePublicationWhenWindowsRequiresIt(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	root.simulateWindowsCloseForTest = true
	path := filepath.Join(dir, "identity-windows.txt")
	closeErr := errors.New("injected staging close failure")
	var closeCalls int
	root.closeStagingFileForTest = func(file *os.File) error {
		closeCalls++
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("destination during simulated Windows staging close: err = %v, want absent", err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("close staging file in test seam: %v", err)
		}
		return closeErr
	}

	info, err := root.CreateNewFileAtomicWithInfo("identity-windows.txt", []byte("complete"), 0o600)
	if !errors.Is(err, closeErr) {
		t.Fatalf("CreateNewFileAtomicWithInfo() error = %v, want staging close failure", err)
	}
	if closeCalls != 1 {
		t.Fatalf("staging close calls = %d, want 1", closeCalls)
	}
	if info != nil {
		t.Fatal("CreateNewFileAtomicWithInfo() returned an identity before publication")
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("destination after simulated Windows staging close failure: %v", err)
	}
}

func TestCreateNewFileAtomicWithInfoReturnsPublishedMetadata(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	info, err := root.CreateNewFileAtomicWithInfo("receipt.json", []byte("complete"), 0o600)
	if err != nil {
		t.Fatalf("CreateNewFileAtomicWithInfo() error = %v", err)
	}
	if info == nil {
		t.Fatal("CreateNewFileAtomicWithInfo() returned nil metadata")
	}
	diskInfo, err := os.Stat(filepath.Join(dir, "receipt.json"))
	if err != nil {
		t.Fatalf("Stat(receipt) error = %v", err)
	}
	if !os.SameFile(info, diskInfo) {
		t.Fatal("returned metadata does not identify the published file")
	}
}

func TestInfoCompatibilityAPIsRetainPublicationDescriptors(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	expected, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := root.WriteFileIfSameWithInfo("settings.xcconfig", []byte("updated"), 0o640, expected, []byte("old"), true); err != nil {
		t.Fatalf("WriteFileIfSameWithInfo() error = %v", err)
	}
	if _, err := root.CreateNewFileAtomicWithInfo("receipt.json", []byte("receipt"), 0o600); err != nil {
		t.Fatalf("CreateNewFileAtomicWithInfo() error = %v", err)
	}
	root.selectedIdentity.mu.RLock()
	retained := len(root.selectedIdentity.retainedFiles)
	root.selectedIdentity.mu.RUnlock()
	if retained != 2 {
		t.Fatalf("compatibility APIs retained %d descriptors, want two", retained)
	}
}

func TestCreateNewFileAtomicWithInfoRejectsPublishedIdentityReplacement(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	if err := os.WriteFile(filepath.Join(dir, "racer-source"), []byte("racer"), 0o600); err != nil {
		t.Fatalf("write racer source: %v", err)
	}
	root.renameNoReplaceForTest = func(parent *os.Root, oldName, newName string) error {
		if err := secureopen.RenameNoReplaceInRoot(parent, oldName, newName); err != nil {
			return err
		}
		if err := parent.Rename(newName, "published-original"); err != nil {
			return err
		}
		return parent.Rename("racer-source", newName)
	}

	info, err := root.CreateNewFileAtomicWithInfo("receipt.json", []byte("complete"), 0o600)
	if err == nil {
		t.Fatal("CreateNewFileAtomicWithInfo() succeeded after the published inode was replaced")
	}
	if errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("legacy CreateNewFileAtomicWithInfo() exposed strict identity sentinel: %v", err)
	}
	if runtime.GOOS != "windows" && info == nil {
		t.Fatal("Unix publication did not retain staged metadata after the published identity changed")
	}
	if got := mustRead(t, filepath.Join(dir, "receipt.json")); got != "racer" {
		t.Fatalf("replacement content = %q, want racer", got)
	}
	if got := mustRead(t, filepath.Join(dir, "published-original")); got != "complete" {
		t.Fatalf("original published content = %q, want complete", got)
	}
	if runtime.GOOS != "windows" {
		originalInfo, statErr := os.Stat(filepath.Join(dir, "published-original"))
		if statErr != nil {
			t.Fatalf("stat original publication: %v", statErr)
		}
		if !os.SameFile(info, originalInfo) {
			t.Fatal("retained metadata does not identify the staged publication")
		}
	}
	diskInfo, statErr := os.Stat(filepath.Join(dir, "receipt.json"))
	if statErr != nil {
		t.Fatalf("Stat(receipt) error = %v", statErr)
	}
	if diskInfo == nil {
		t.Fatal("Stat(receipt) returned a nil replacement identity")
	}
}

func TestCreateNewFileAtomicWithInfoRechecksActualPublishedIdentity(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	const content = "complete"
	if err := os.WriteFile(filepath.Join(dir, "racer-source"), []byte(content), 0o600); err != nil {
		t.Fatalf("write racer source: %v", err)
	}
	root.afterPublicationOpenForTest = func(parent *os.Root, name string) {
		if err := parent.Rename(name, "published-original"); err != nil {
			t.Fatalf("move original publication: %v", err)
		}
		if err := parent.Rename("racer-source", name); err != nil {
			t.Fatalf("install same-content replacement: %v", err)
		}
	}

	info, err := root.CreateNewFileAtomicWithInfo("receipt.json", []byte(content), 0o600)
	if err == nil {
		t.Fatal("CreateNewFileAtomicWithInfo() succeeded after same-content replacement")
	}
	if errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("legacy CreateNewFileAtomicWithInfo() exposed strict identity sentinel: %v", err)
	}
	if info == nil {
		t.Fatal("CreateNewFileAtomicWithInfo() returned no published metadata")
	}
	originalInfo, err := os.Stat(filepath.Join(dir, "published-original"))
	if err != nil {
		t.Fatalf("stat original publication: %v", err)
	}
	if !os.SameFile(info, originalInfo) {
		t.Fatal("returned metadata does not identify the originally installed inode")
	}
	replacementInfo, err := os.Stat(filepath.Join(dir, "receipt.json"))
	if err != nil {
		t.Fatalf("stat replacement: %v", err)
	}
	if os.SameFile(info, replacementInfo) {
		t.Fatal("returned metadata incorrectly identified same-content replacement")
	}
}

func TestCreateNewFileAtomicWithInfoReturnsNilBeforeDestinationIdentity(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	const content = "complete"
	root.simulateWindowsCloseForTest = true
	if err := os.WriteFile(filepath.Join(dir, "racer-source"), []byte(content), 0o600); err != nil {
		t.Fatalf("write racer source: %v", err)
	}
	root.beforePublicationOpenForTest = func(parent *os.Root, name string) {
		if err := parent.Rename(name, "published-original"); err != nil {
			t.Fatalf("move original publication: %v", err)
		}
		if err := parent.Rename("racer-source", name); err != nil {
			t.Fatalf("install same-content replacement: %v", err)
		}
	}

	info, err := root.CreateNewFileAtomicWithInfo("receipt.json", []byte(content), 0o600)
	if err == nil || !strings.Contains(err.Error(), "identity changed before reopen") {
		t.Fatalf("CreateNewFileAtomicWithInfo() error = %v, want reopen identity uncertainty", err)
	}
	if info != nil {
		t.Fatal("CreateNewFileAtomicWithInfo() returned metadata before the destination identity was proven")
	}
	if got := mustRead(t, filepath.Join(dir, "published-original")); got != content {
		t.Fatalf("original published content = %q, want staged bytes", got)
	}
	replacementInfo, statErr := os.Stat(filepath.Join(dir, "receipt.json"))
	if statErr != nil {
		t.Fatalf("stat replacement: %v", statErr)
	}
	if replacementInfo == nil {
		t.Fatal("Stat(replacement) returned a nil identity")
	}
}

func TestCreateNewFileAtomicWithInfoRetainsIdentityAfterInitialPublishedLstatFailure(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	transient := errors.New("injected published lstat failure")
	root.postPublicationLstatForTest = func(_ *os.Root, _ string) (os.FileInfo, error) {
		return nil, transient
	}

	info, err := root.CreateNewFileAtomicWithInfo("receipt.json", []byte("complete"), 0o600)
	if err == nil || !errors.Is(err, transient) {
		t.Fatalf("CreateNewFileAtomicWithInfo() error = %v, want injected Lstat failure", err)
	}
	if info == nil {
		t.Fatal("CreateNewFileAtomicWithInfo() returned nil identity after transient Lstat failure")
	}
	diskInfo, err := os.Stat(filepath.Join(dir, "receipt.json"))
	if err != nil {
		t.Fatalf("Stat(receipt) error = %v", err)
	}
	if !os.SameFile(info, diskInfo) {
		t.Fatal("returned identity does not identify the published file")
	}
}

func TestCreateNewFileAtomicWithInfoRetainsIdentityAfterPublishedReopenFailure(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	transient := errors.New("injected published reopen failure")
	attempts := 0
	root.openPublishedFileForTest = func(parent *os.Root, name string) (*os.File, error) {
		attempts++
		if attempts == 1 {
			return nil, transient
		}
		return secureopen.OpenExistingNoFollowInRoot(parent, name)
	}

	info, err := root.CreateNewFileAtomicWithInfo("receipt.json", []byte("complete"), 0o600)
	if err == nil || !errors.Is(err, transient) {
		t.Fatalf("CreateNewFileAtomicWithInfo() error = %v, want injected reopen failure", err)
	}
	if info == nil {
		t.Fatal("CreateNewFileAtomicWithInfo() returned nil identity after transient reopen failure")
	}
	diskInfo, err := os.Stat(filepath.Join(dir, "receipt.json"))
	if err != nil {
		t.Fatalf("Stat(receipt) error = %v", err)
	}
	if !os.SameFile(info, diskInfo) {
		t.Fatal("returned identity does not identify the published file")
	}
}

func TestCreateNewFileAtomicWithInfoRetainsIdentityAfterPublishedDescriptorStatFailure(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	root.simulateWindowsCloseForTest = true
	transient := errors.New("injected published descriptor stat failure")
	attempts := 0
	root.statPublishedFileForTest = func(file *os.File) (os.FileInfo, error) {
		attempts++
		if attempts == 1 {
			return nil, transient
		}
		return file.Stat()
	}

	info, err := root.CreateNewFileAtomicWithInfo("receipt.json", []byte("complete"), 0o600)
	if err == nil || !errors.Is(err, transient) {
		t.Fatalf("CreateNewFileAtomicWithInfo() error = %v, want injected descriptor Stat failure", err)
	}
	if info == nil {
		t.Fatal("CreateNewFileAtomicWithInfo() returned nil identity after transient descriptor Stat failure")
	}
	diskInfo, err := os.Stat(filepath.Join(dir, "receipt.json"))
	if err != nil {
		t.Fatalf("Stat(receipt) error = %v", err)
	}
	if !os.SameFile(info, diskInfo) {
		t.Fatal("returned identity does not identify the published file")
	}
}

func TestCreateNewFileAtomicWithInfoKeepsUnixStagingIdentityWhenPublishedStatStaysUnavailable(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	transient := errors.New("persistent published descriptor stat failure")
	root.statPublishedFileForTest = func(*os.File) (os.FileInfo, error) {
		return nil, transient
	}

	info, err := root.CreateNewFileAtomicWithInfo("receipt.json", []byte("complete"), 0o600)
	if err == nil || !errors.Is(err, transient) {
		t.Fatalf("CreateNewFileAtomicWithInfo() error = %v, want persistent descriptor Stat failure", err)
	}
	if runtime.GOOS == "windows" {
		if info != nil {
			t.Fatal("Windows returned an identity without a verified published descriptor")
		}
		return
	}
	if info == nil {
		t.Fatal("Unix lost the retained staging identity after persistent descriptor Stat failure")
	}
	diskInfo, err := os.Stat(filepath.Join(dir, "receipt.json"))
	if err != nil {
		t.Fatalf("Stat(receipt) error = %v", err)
	}
	if !os.SameFile(info, diskInfo) {
		t.Fatal("returned identity does not identify the published file")
	}
}

func TestCreateNewFileAtomicWithInfoRetainsStagingIdentityWhilePublishedReopenFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows must close the staging descriptor before publication")
	}
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	transient := errors.New("injected persistent published reopen failure")
	root.openPublishedFileForTest = func(*os.Root, string) (*os.File, error) {
		return nil, transient
	}

	info, err := root.CreateNewFileAtomicWithInfo("receipt.json", []byte("complete"), 0o600)
	if err == nil || !errors.Is(err, transient) {
		t.Fatalf("CreateNewFileAtomicWithInfo() error = %v, want injected reopen failure", err)
	}
	if info == nil {
		t.Fatal("CreateNewFileAtomicWithInfo() returned nil retained staging identity")
	}
	diskInfo, err := os.Stat(filepath.Join(dir, "receipt.json"))
	if err != nil {
		t.Fatalf("Stat(receipt) error = %v", err)
	}
	if !os.SameFile(info, diskInfo) {
		t.Fatal("retained staging identity does not identify the published file")
	}
}

func TestCreateNewFileAtomicWithInfoCapturesDestinationAfterInitialPublishedLstatFailure(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	root.simulateWindowsCloseForTest = true
	transient := errors.New("injected published lstat failure")
	root.postPublicationLstatForTest = func(_ *os.Root, _ string) (os.FileInfo, error) {
		return nil, transient
	}

	info, err := root.CreateNewFileAtomicWithInfo("receipt.json", []byte("complete"), 0o600)
	if err == nil || !errors.Is(err, transient) {
		t.Fatalf("CreateNewFileAtomicWithInfo() error = %v, want injected Lstat failure", err)
	}
	if info == nil {
		t.Fatal("CreateNewFileAtomicWithInfo() returned nil destination identity after transient Lstat failure")
	}
	diskInfo, err := os.Stat(filepath.Join(dir, "receipt.json"))
	if err != nil {
		t.Fatalf("Stat(receipt) error = %v", err)
	}
	if !os.SameFile(info, diskInfo) {
		t.Fatal("returned identity does not identify the published file")
	}
}

func TestCreateNewFileAtomicWithInfoPreservesReplacementAfterInitialPublishedLstatFailure(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	if err := os.WriteFile(filepath.Join(dir, "racer-source"), []byte("replacement"), 0o600); err != nil {
		t.Fatalf("WriteFile(racer-source) error = %v", err)
	}
	transient := errors.New("injected published lstat failure")
	root.postPublicationLstatForTest = func(parent *os.Root, name string) (os.FileInfo, error) {
		if err := parent.Rename(name, "published-original"); err != nil {
			t.Fatalf("move original publication: %v", err)
		}
		if err := parent.Rename("racer-source", name); err != nil {
			t.Fatalf("install replacement publication: %v", err)
		}
		return nil, transient
	}

	info, err := root.CreateNewFileAtomicWithInfo("receipt.json", []byte("complete"), 0o600)
	if err == nil || !errors.Is(err, transient) {
		t.Fatalf("CreateNewFileAtomicWithInfo() error = %v, want injected Lstat failure", err)
	}
	if runtime.GOOS != "windows" && info == nil {
		t.Fatal("Unix publication did not retain staged identity for replacement-safe rollback")
	}
	if got := mustRead(t, filepath.Join(dir, "receipt.json")); got != "replacement" {
		t.Fatalf("replacement content = %q, want replacement preserved", got)
	}
	if got := mustRead(t, filepath.Join(dir, "published-original")); got != "complete" {
		t.Fatalf("published original content = %q, want staged content preserved", got)
	}
	if runtime.GOOS != "windows" {
		if info == nil {
			t.Fatal("Unix publication did not retain staged metadata for replacement recovery")
		}
		originalInfo, statErr := os.Stat(filepath.Join(dir, "published-original"))
		if statErr != nil {
			t.Fatalf("stat published original: %v", statErr)
		}
		if !os.SameFile(info, originalInfo) {
			t.Fatal("retained metadata does not identify the staged publication")
		}
	}
}

func TestCreateNewFileAtomicWithInfoPreservesDisappearanceAfterInitialPublishedLstatFailure(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	transient := errors.New("injected published lstat failure")
	root.postPublicationLstatForTest = func(parent *os.Root, name string) (os.FileInfo, error) {
		if err := parent.Remove(name); err != nil {
			t.Fatalf("remove published entry: %v", err)
		}
		return nil, transient
	}

	info, err := root.CreateNewFileAtomicWithInfo("receipt.json", []byte("complete"), 0o600)
	if err == nil || !errors.Is(err, transient) {
		t.Fatalf("CreateNewFileAtomicWithInfo() error = %v, want injected Lstat failure", err)
	}
	if runtime.GOOS != "windows" && info == nil {
		t.Fatal("Unix publication did not retain staged identity after disappearance")
	}
	if _, err := os.Lstat(filepath.Join(dir, "receipt.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("receipt after disappearance = %v, want absent", err)
	}
}

func TestRemoveFileIfSamePreservesReplacementAfterQuarantine(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	const content = "receipt"
	if err := os.WriteFile(filepath.Join(dir, "receipt.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "racer-source"), []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	expected, err := os.Stat(filepath.Join(dir, "receipt.json"))
	if err != nil {
		t.Fatal(err)
	}
	root.afterConditionalQuarantineForTest = func(parent *os.Root, _, name string) {
		if err := parent.Rename("racer-source", name); err != nil {
			t.Fatalf("install replacement after quarantine: %v", err)
		}
	}
	syncErr := errors.New("injected replacement cleanup sync failure")
	var syncObserved bool
	root.syncDirectoryForTest = func(*os.Root) error {
		syncObserved = true
		return syncErr
	}

	err = root.RemoveFileIfSame("receipt.json", expected, []byte(content))
	if err == nil || !strings.Contains(err.Error(), "replaced") {
		t.Fatalf("RemoveFileIfSame() error = %v, want replacement uncertainty", err)
	}
	if !errors.Is(err, syncErr) || !syncObserved {
		t.Fatalf("RemoveFileIfSame() error = %v, want parent sync failure after quarantine cleanup", err)
	}
	if got := mustRead(t, filepath.Join(dir, "receipt.json")); got != "replacement" {
		t.Fatalf("replacement content = %q, want preserved replacement", got)
	}
	if matches, globErr := filepath.Glob(filepath.Join(dir, rollbackFilePattern[:len(rollbackFilePattern)-1]+"*")); globErr != nil {
		t.Fatal(globErr)
	} else if len(matches) != 0 {
		t.Fatalf("quarantine files remain after replacement preservation: %v", matches)
	}
}

func TestRemoveFileIfSameSyncsParentAfterSuccessfulRemoval(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	const content = "receipt"
	path := filepath.Join(dir, "receipt.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	expected, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	syncErr := errors.New("injected receipt removal sync failure")
	var syncObserved bool
	root.syncDirectoryForTest = func(_ *os.Root) error {
		syncObserved = true
		return syncErr
	}

	err = root.RemoveFileIfSame("receipt.json", expected, []byte(content))
	if !errors.Is(err, syncErr) {
		t.Fatalf("RemoveFileIfSame() error = %v, want parent sync failure", err)
	}
	if !syncObserved {
		t.Fatal("parent directory sync hook was not invoked")
	}
	if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("removed receipt stat error = %v, want absent", statErr)
	}
}

func TestRemoveFileIfSamePreservesReplacementBeforeQuarantine(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	const content = "receipt"
	if err := os.WriteFile(filepath.Join(dir, "receipt.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "racer-source"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	expected, err := os.Stat(filepath.Join(dir, "receipt.json"))
	if err != nil {
		t.Fatal(err)
	}
	root.beforeConditionalQuarantineForTest = func(parent *os.Root, name string) {
		if err := parent.Rename(name, "published-original"); err != nil {
			t.Fatalf("move original before quarantine: %v", err)
		}
		if err := parent.Rename("racer-source", name); err != nil {
			t.Fatalf("install same-content replacement before quarantine: %v", err)
		}
	}

	err = root.RemoveFileIfSame("receipt.json", expected, []byte(content))
	if err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatalf("RemoveFileIfSame() error = %v, want pre-quarantine identity uncertainty", err)
	}
	if got := mustRead(t, filepath.Join(dir, "receipt.json")); got != content {
		t.Fatalf("replacement content = %q, want preserved replacement", got)
	}
	if got := mustRead(t, filepath.Join(dir, "published-original")); got != content {
		t.Fatalf("original content = %q, want preserved original", got)
	}
}

func TestRemoveFileIfSameReportsQuarantineDisappearanceBeforeRemoval(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	const content = "receipt"
	path := filepath.Join(dir, "receipt.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	expected, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	root.beforeConditionalQuarantineRemovalForTest = func(parent *os.Root, quarantineName string) {
		if err := parent.Remove(quarantineName); err != nil {
			t.Fatalf("remove quarantined entry in race hook: %v", err)
		}
	}

	err = root.RemoveFileIfSame("receipt.json", expected, []byte(content))
	if err == nil || !strings.Contains(err.Error(), "recheck quarantined file") {
		t.Fatalf("RemoveFileIfSame() error = %v, want disappearance uncertainty", err)
	}
	if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("destination after quarantine disappearance = %v, want absent", statErr)
	}
	matches, globErr := filepath.Glob(filepath.Join(dir, rollbackFilePattern[:len(rollbackFilePattern)-1]+"*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) != 0 {
		t.Fatalf("quarantine entries remain after injected disappearance: %v", matches)
	}
}

func TestRemoveFileIfSameRestoresQuarantineWhenDestinationCheckFails(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	const content = "receipt"
	path := filepath.Join(dir, "receipt.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	expected, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	injected := errors.New("injected destination lstat failure")
	root.postConditionalQuarantineLstatForTest = func(_ *os.Root, _ string) (os.FileInfo, error) {
		return nil, injected
	}

	err = root.RemoveFileIfSame("receipt.json", expected, []byte(content))
	if !errors.Is(err, injected) {
		t.Fatalf("RemoveFileIfSame() error = %v, want injected destination-check failure", err)
	}
	if got := mustRead(t, path); got != content {
		t.Fatalf("destination after uncertain check = %q, want restored original", got)
	}
	if matches, globErr := filepath.Glob(filepath.Join(dir, rollbackFilePattern[:len(rollbackFilePattern)-1]+"*")); globErr != nil {
		t.Fatal(globErr)
	} else if len(matches) != 0 {
		t.Fatalf("quarantine files remain after recovery: %v", matches)
	}
}

func TestRemoveFileIfSameRejectsQuarantineContentChangeBeforeRemoval(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	const content = "receipt"
	const mutated = "mutated-by-concurrent-writer"
	path := filepath.Join(dir, "receipt.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	expected, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	var quarantinePath string
	root.beforeConditionalQuarantineRemovalForTest = func(parent *os.Root, quarantineName string) {
		quarantinePath = filepath.Join(dir, quarantineName)
		file, openErr := parent.OpenFile(quarantineName, os.O_WRONLY|os.O_TRUNC, 0)
		if openErr != nil {
			t.Fatalf("mutate quarantined entry in race hook: %v", openErr)
		}
		if _, writeErr := file.Write([]byte(mutated)); writeErr != nil {
			_ = file.Close()
			t.Fatalf("write mutated quarantine contents: %v", writeErr)
		}
		if closeErr := file.Close(); closeErr != nil {
			t.Fatalf("close mutated quarantine: %v", closeErr)
		}
	}

	err = root.RemoveFileIfSame("receipt.json", expected, []byte(content))
	if err == nil || !strings.Contains(err.Error(), "contents changed") {
		t.Fatalf("RemoveFileIfSame() error = %v, want contents-changed uncertainty", err)
	}
	if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("destination after content race = %v, want absent", statErr)
	}
	if quarantinePath == "" {
		t.Fatal("race hook did not capture quarantine path")
	}
	if got := mustRead(t, quarantinePath); got != mutated {
		t.Fatalf("quarantine leftover = %q, want mutated contents preserved", got)
	}
}

func TestRemoveFileIfSamePreservesReplacementBetweenQuarantineCheckAndRemoval(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	const content = "receipt"
	const replacement = "replacement"
	path := filepath.Join(dir, "receipt.json")
	racerPath := filepath.Join(dir, "racer-source")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(racerPath, []byte(replacement), 0o600); err != nil {
		t.Fatal(err)
	}
	expected, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	var replacementQuarantine string
	root.beforeConditionalQuarantineRemovalForTest = func(parent *os.Root, quarantineName string) {
		replacementQuarantine = quarantineName
		if err := parent.Rename(quarantineName, "quarantine-original"); err != nil {
			t.Fatalf("retain original quarantine entry: %v", err)
		}
		if err := parent.Rename("racer-source", quarantineName); err != nil {
			t.Fatalf("install replacement quarantine entry: %v", err)
		}
	}

	err = root.RemoveFileIfSame("receipt.json", expected, []byte(content))
	if err == nil || !strings.Contains(err.Error(), "identity changed before removal") {
		t.Fatalf("RemoveFileIfSame() error = %v, want identity uncertainty", err)
	}
	if errors.Is(err, ErrQuarantineCleanupUncertain) {
		t.Fatalf("legacy RemoveFileIfSame() exposed strict identity sentinel: %v", err)
	}
	if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("destination after quarantine race = %v, want absent", statErr)
	}
	if got := mustRead(t, filepath.Join(dir, "quarantine-original")); got != content {
		t.Fatalf("original quarantine content = %q, want original receipt", got)
	}
	if replacementQuarantine == "" {
		t.Fatal("race hook did not capture replacement quarantine name")
	}
	if got := mustRead(t, filepath.Join(dir, replacementQuarantine)); got != replacement {
		t.Fatalf("replacement quarantine content = %q, want replacement", got)
	}
}

func TestLegacyConditionalMutationsRejectQuarantineModeDrift(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits beyond owner-writable are unsupported on Windows; 0600 and 0640 both report as writable")
	}
	for _, testCase := range []struct {
		name       string
		invoke     func(Root, os.FileInfo) error
		wantResult string
	}{
		{
			name: "remove",
			invoke: func(root Root, expected os.FileInfo) error {
				return root.RemoveFileIfSame("settings.xcconfig", expected, []byte("old"))
			},
		},
		{
			name: "write",
			invoke: func(root Root, expected os.FileInfo) error {
				return root.WriteFileIfSame("settings.xcconfig", []byte("new"), 0o640, expected, []byte("old"), true)
			},
			wantResult: "new",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			dir := t.TempDir()
			root := mustRoot(t, dir)
			t.Cleanup(func() { _ = root.Close() })
			path := filepath.Join(dir, "settings.xcconfig")
			if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
				t.Fatal(err)
			}
			expected, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			var quarantineName string
			root.openExpectedFileForTest = func(parent *os.Root, name string, expected os.FileInfo, expectedData []byte) (*os.File, os.FileInfo, error) {
				file, info, err := openExpectedRootedFile(parent, name, expected, expectedData)
				if err != nil || name == "settings.xcconfig" {
					return file, info, err
				}
				quarantineName = name
				if err := file.Chmod(0o640); err != nil {
					_ = file.Close()
					t.Fatalf("chmod quarantined file: %v", err)
				}
				return file, info, nil
			}

			err = testCase.invoke(root, expected)
			if err == nil {
				t.Fatal("legacy conditional mutation succeeded after quarantine mode drift")
			}
			if quarantineName == "" {
				t.Fatal("quarantine mode race hook did not observe a quarantine entry")
			}
			quarantinePath := filepath.Join(dir, quarantineName)
			quarantineInfo, statErr := os.Stat(quarantinePath)
			if statErr != nil {
				t.Fatalf("stat preserved quarantine: %v", statErr)
			}
			if got := quarantineInfo.Mode().Perm(); got != 0o640 {
				t.Fatalf("preserved quarantine mode = %o, want drifted mode 640", got)
			}
			if testCase.wantResult == "" {
				if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("removed destination = %v, want absent", statErr)
				}
			} else if got := mustRead(t, path); got != testCase.wantResult {
				t.Fatalf("published destination = %q, want %q", got, testCase.wantResult)
			}
		})
	}
}

func TestWriteFileIfSameClosesStagingBeforePublication(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	expected, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	closeErr := errors.New("injected staging close failure")
	var closeCalls int
	root.closeStagingFileForTest = func(file *os.File) error {
		closeCalls++
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("destination during staging close: err = %v, want absent so quarantine can still restore", err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("close staging file in test seam: %v", err)
		}
		return closeErr
	}

	err = root.WriteFileIfSame("settings.xcconfig", []byte("new"), 0o640, expected, []byte("old"), true)
	if !errors.Is(err, closeErr) {
		t.Fatalf("WriteFileIfSame() error = %v, want staging close failure", err)
	}
	if closeCalls != 1 {
		t.Fatalf("staging close calls = %d, want 1", closeCalls)
	}
	if got := mustRead(t, path); got != "old" {
		t.Fatalf("destination after staging close failure = %q, want restored original", got)
	}
}

func TestWriteFileIfSamePreservesReplacementAfterQuarantine(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "settings.xcconfig"), []byte("new"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "racer-source"), []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	expected, err := os.Stat(filepath.Join(dir, "settings.xcconfig"))
	if err != nil {
		t.Fatal(err)
	}
	root.afterConditionalQuarantineForTest = func(parent *os.Root, _, name string) {
		if err := parent.Rename("racer-source", name); err != nil {
			t.Fatalf("install replacement after quarantine: %v", err)
		}
	}
	syncErr := errors.New("injected replacement cleanup sync failure")
	var syncObserved bool
	root.syncDirectoryForTest = func(*os.Root) error {
		syncObserved = true
		return syncErr
	}

	err = root.WriteFileIfSame("settings.xcconfig", []byte("old"), 0o640, expected, []byte("new"), true)
	if err == nil || !strings.Contains(err.Error(), "destination changed") {
		t.Fatalf("WriteFileIfSame() error = %v, want replacement uncertainty", err)
	}
	if !errors.Is(err, syncErr) || !syncObserved {
		t.Fatalf("WriteFileIfSame() error = %v, want parent sync failure after quarantine cleanup", err)
	}
	if errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("legacy WriteFileIfSame() exposed strict identity sentinel: %v", err)
	}
	if got := mustRead(t, filepath.Join(dir, "settings.xcconfig")); got != "replacement" {
		t.Fatalf("replacement content = %q, want preserved replacement", got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".asc-tmp-rollback-") {
			t.Fatalf("quarantine leftover %q remained after discarding an open rollback handle", entry.Name())
		}
	}
}

func TestLegacyConditionalMutationsValidatePathBeforeNilIdentity(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	if err := os.WriteFile(filepath.Join(dir, "value"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real", filepath.Join(dir, "linked")); err != nil {
		t.Skipf("symlink creation is not permitted: %v", err)
	}

	operations := map[string]func(string) error{
		"remove": func(name string) error {
			return root.RemoveFileIfSame(name, nil, nil)
		},
		"write": func(name string) error {
			return root.WriteFileIfSame(name, []byte("new"), 0o600, nil, nil, true)
		},
	}
	for name, operation := range operations {
		t.Run(name+"/root", func(t *testing.T) {
			if err := operation("."); !errors.Is(err, ErrEscapesRoot) {
				t.Fatalf("operation error = %v, want ErrEscapesRoot", err)
			}
		})
		t.Run(name+"/symlink-parent", func(t *testing.T) {
			if err := operation(filepath.Join("linked", "value")); !errors.Is(err, ErrSymlink) {
				t.Fatalf("operation error = %v, want ErrSymlink", err)
			}
		})
		t.Run(name+"/ordinary-path", func(t *testing.T) {
			err := operation("value")
			if err == nil || err.Error() != "expected file identity is unavailable" {
				t.Fatalf("operation error = %v, want legacy identity error", err)
			}
			if errors.Is(err, ErrFileIdentityChanged) {
				t.Fatalf("legacy operation exposed strict identity sentinel: %v", err)
			}
		})
	}
}

func TestWriteFileIfSameKeepsVerifiedQuarantineHandleThroughPreparation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("old"), 0o640); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}
	expected, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(source) error = %v", err)
	}
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	quarantineOpens := 0
	transient := errors.New("injected quarantine reopen failure")
	root.openExpectedFileForTest = func(parent *os.Root, name string, expected os.FileInfo, expectedData []byte) (*os.File, os.FileInfo, error) {
		if name != "settings.xcconfig" {
			quarantineOpens++
			if quarantineOpens == 2 {
				if _, statErr := parent.Lstat("settings.xcconfig"); errors.Is(statErr, os.ErrNotExist) {
					return nil, nil, transient
				}
			}
		}
		return openExpectedRootedFile(parent, name, expected, expectedData)
	}

	if err := root.WriteFileIfSame("settings.xcconfig", []byte("new"), 0o640, expected, []byte("old"), true); err != nil {
		t.Fatalf("WriteFileIfSame() error = %v", err)
	}
	if got := mustRead(t, path); got != "new" {
		t.Fatalf("updated content = %q, want new", got)
	}
}

func TestWriteFileIfSameRestoresQuarantineAfterReopenFailureAndReportsSync(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	path := filepath.Join(dir, "settings.xcconfig")
	const original = "old"
	if err := os.WriteFile(path, []byte(original), 0o640); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}
	expected, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(source) error = %v", err)
	}
	transient := errors.New("injected quarantine reopen failure")
	syncErr := errors.New("injected quarantine restore sync failure")
	root.openExpectedFileForTest = func(parent *os.Root, name string, _ os.FileInfo, _ []byte) (*os.File, os.FileInfo, error) {
		if name != "settings.xcconfig" {
			return nil, nil, transient
		}
		return openExpectedRootedFile(parent, name, expected, []byte(original))
	}
	root.syncDirectoryForTest = func(_ *os.Root) error {
		return syncErr
	}

	err = root.WriteFileIfSame("settings.xcconfig", []byte("new"), 0o640, expected, []byte(original), true)
	if !errors.Is(err, transient) {
		t.Fatalf("WriteFileIfSame() error = %v, want reopen failure", err)
	}
	if !errors.Is(err, syncErr) {
		t.Fatalf("WriteFileIfSame() error = %v, want restore sync failure", err)
	}
	if got := mustRead(t, path); got != original {
		t.Fatalf("restored content = %q, want %q", got, original)
	}
	matches, globErr := filepath.Glob(filepath.Join(dir, rollbackFilePattern[:len(rollbackFilePattern)-1]+"*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) != 0 {
		t.Fatalf("quarantine files remain after reopen failure: %v", matches)
	}
}

func TestWriteFileIfSameClosesQuarantineBeforeRestoreAfterMetadataCopyFailure(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	path := filepath.Join(dir, "settings.xcconfig")
	const original = "original"
	if err := os.WriteFile(path, []byte(original), 0o640); err != nil {
		t.Fatal(err)
	}
	expected, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	injected := errors.New("injected metadata copy failure")
	root.copyReplacementMetadataForTest = func(*os.File, *os.File, os.FileInfo) error {
		return injected
	}

	err = root.WriteFileIfSame("settings.xcconfig", []byte("updated"), 0o640, expected, []byte(original), true)
	if !errors.Is(err, injected) {
		t.Fatalf("WriteFileIfSame() error = %v, want injected metadata-copy failure", err)
	}
	if got := mustRead(t, path); got != original {
		t.Fatalf("destination after metadata-copy failure = %q, want restored original", got)
	}
	if matches, globErr := filepath.Glob(filepath.Join(dir, rollbackFilePattern[:len(rollbackFilePattern)-1]+"*")); globErr != nil {
		t.Fatal(globErr)
	} else if len(matches) != 0 {
		t.Fatalf("quarantine files remain after metadata-copy recovery: %v", matches)
	}
}

func TestWriteFileIfSameRestoresQuarantineWhenDestinationCheckFails(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	path := filepath.Join(dir, "settings.xcconfig")
	const original = "original"
	if err := os.WriteFile(path, []byte(original), 0o640); err != nil {
		t.Fatal(err)
	}
	expected, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	injected := errors.New("injected destination lstat failure")
	root.postConditionalQuarantineLstatForTest = func(_ *os.Root, _ string) (os.FileInfo, error) {
		return nil, injected
	}

	err = root.WriteFileIfSame("settings.xcconfig", []byte("updated"), 0o640, expected, []byte(original), true)
	if !errors.Is(err, injected) {
		t.Fatalf("WriteFileIfSame() error = %v, want injected destination-check failure", err)
	}
	if got := mustRead(t, path); got != original {
		t.Fatalf("destination after uncertain check = %q, want restored original", got)
	}
	if matches, globErr := filepath.Glob(filepath.Join(dir, rollbackFilePattern[:len(rollbackFilePattern)-1]+"*")); globErr != nil {
		t.Fatal(globErr)
	} else if len(matches) != 0 {
		t.Fatalf("quarantine files remain after recovery: %v", matches)
	}
}

func TestWriteFileIfSameReportsRestoreSyncFailure(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	path := filepath.Join(dir, "settings.xcconfig")
	const original = "original"
	if err := os.WriteFile(path, []byte(original), 0o640); err != nil {
		t.Fatal(err)
	}
	expected, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	publicationErr := errors.New("injected publication failure")
	syncErr := errors.New("injected quarantine restore sync failure")
	root.renameNoReplaceForTest = func(_ *os.Root, _, _ string) error {
		return publicationErr
	}
	var syncObserved bool
	root.syncDirectoryForTest = func(_ *os.Root) error {
		syncObserved = true
		return syncErr
	}

	err = root.WriteFileIfSame(path, []byte("updated"), 0o640, expected, []byte(original), true)
	if !errors.Is(err, publicationErr) {
		t.Fatalf("WriteFileIfSame() error = %v, want publication failure", err)
	}
	if !errors.Is(err, syncErr) {
		t.Fatalf("WriteFileIfSame() error = %v, want quarantine-restore sync failure", err)
	}
	if !syncObserved {
		t.Fatal("quarantine restore did not sync its parent directory")
	}
	if got := mustRead(t, path); got != original {
		t.Fatalf("restored content = %q, want original", got)
	}
}

func TestWriteFileIfSameLeavesQuarantineWhenRestoreSeesReplacement(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("new"), 0o640); err != nil {
		t.Fatal(err)
	}
	expected, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	root.beforeConditionalPublishForTest = func(parent *os.Root, name string) {
		if err := parent.WriteFile(name, []byte("replacement"), 0o600); err != nil {
			t.Fatalf("install replacement before publish: %v", err)
		}
	}

	err = root.WriteFileIfSame("settings.xcconfig", []byte("old"), 0o640, expected, []byte("new"), true)
	if err == nil || !strings.Contains(err.Error(), "was left in place") {
		t.Fatalf("WriteFileIfSame() error = %v, want preserved quarantine diagnostic", err)
	}
	if got := mustRead(t, path); got != "replacement" {
		t.Fatalf("replacement content = %q, want preserved replacement", got)
	}
	matches, globErr := filepath.Glob(filepath.Join(dir, rollbackFilePattern[:len(rollbackFilePattern)-1]+"*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) != 1 {
		t.Fatalf("quarantine count = %d, want one recoverable quarantine: %v", len(matches), matches)
	}
	if got := mustRead(t, matches[0]); got != "new" {
		t.Fatalf("quarantine content = %q, want original replacement", got)
	}
}

func TestWriteFileIfSameLeavesQuarantineAfterPublishedOpenFailure(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("new"), 0o640); err != nil {
		t.Fatal(err)
	}
	expected, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	root.afterConditionalPublicationForTest = func(parent *os.Root, name string) {
		if err := parent.Remove(name); err != nil {
			t.Fatalf("remove published file in race hook: %v", err)
		}
	}

	err = root.WriteFileIfSame("settings.xcconfig", []byte("old"), 0o640, expected, []byte("new"), true)
	if err == nil || !strings.Contains(err.Error(), "after publication uncertainty") {
		t.Fatalf("WriteFileIfSame() error = %v, want published-open uncertainty", err)
	}
	if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("destination after published-open failure = %v, want absent", statErr)
	}
	matches, globErr := filepath.Glob(filepath.Join(dir, rollbackFilePattern[:len(rollbackFilePattern)-1]+"*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) != 1 {
		t.Fatalf("quarantine count = %d, want one recoverable quarantine: %v", len(matches), matches)
	}
	if got := mustRead(t, matches[0]); got != "new" {
		t.Fatalf("quarantine content = %q, want original bytes", got)
	}
}

func TestWriteFileIfSameLeavesQuarantineAfterPublishedStatFailure(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("new"), 0o640); err != nil {
		t.Fatal(err)
	}
	expected, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	root.afterConditionalPublicationOpenForTest = func(_ *os.Root, _ string, file *os.File) {
		if err := file.Close(); err != nil {
			t.Fatalf("close published file in stat-failure hook: %v", err)
		}
	}

	err = root.WriteFileIfSame("settings.xcconfig", []byte("old"), 0o640, expected, []byte("new"), true)
	if err == nil || !strings.Contains(err.Error(), "stat published file") || !strings.Contains(err.Error(), "after publication uncertainty") {
		t.Fatalf("WriteFileIfSame() error = %v, want published-stat uncertainty", err)
	}
	if _, statErr := os.Lstat(path); statErr != nil {
		t.Fatalf("destination after published-stat failure = %v, want published file", statErr)
	}
	matches, globErr := filepath.Glob(filepath.Join(dir, rollbackFilePattern[:len(rollbackFilePattern)-1]+"*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) != 1 {
		t.Fatalf("quarantine count = %d, want one recoverable quarantine: %v", len(matches), matches)
	}
	if got := mustRead(t, matches[0]); got != "new" {
		t.Fatalf("quarantine content = %q, want original bytes", got)
	}
}

func TestWriteFileIfSamePreservesQuarantineAfterPublishedIdentityFailure(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	root.simulateWindowsCloseForTest = true
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("new"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "racer-source"), []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	expected, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	root.afterConditionalPublicationForTest = func(parent *os.Root, name string) {
		if err := parent.Rename(name, "published-original"); err != nil {
			t.Fatalf("retain original publication: %v", err)
		}
		if err := parent.Rename("racer-source", name); err != nil {
			t.Fatalf("install replacement publication: %v", err)
		}
	}

	info, err := root.WriteFileIfSameWithInfo("settings.xcconfig", []byte("old"), 0o640, expected, []byte("new"), true)
	if err == nil || !strings.Contains(err.Error(), "after publication uncertainty") || !strings.Contains(err.Error(), "identity changed") {
		t.Fatalf("WriteFileIfSameWithInfo() error = %v, want identity uncertainty with quarantine preservation", err)
	}
	if info != nil {
		t.Fatal("WriteFileIfSameWithInfo() returned metadata after the published identity changed")
	}
	if got := mustRead(t, path); got != "replacement" {
		t.Fatalf("replacement content = %q, want preserved replacement", got)
	}
	if got := mustRead(t, filepath.Join(dir, "published-original")); got != "old" {
		t.Fatalf("original published content = %q, want staged bytes", got)
	}
	matches, globErr := filepath.Glob(filepath.Join(dir, rollbackFilePattern[:len(rollbackFilePattern)-1]+"*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) != 1 {
		t.Fatalf("quarantine count = %d, want one recoverable quarantine: %v", len(matches), matches)
	}
	if got := mustRead(t, matches[0]); got != "new" {
		t.Fatalf("quarantine content = %q, want original bytes", got)
	}
}

func TestWriteFileIfSameRestoresOriginalWhenUnchanged(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("new"), 0o640); err != nil {
		t.Fatal(err)
	}
	expected, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := root.WriteFileIfSame("settings.xcconfig", []byte("old"), 0o640, expected, []byte("new"), true); err != nil {
		t.Fatalf("WriteFileIfSame() error = %v", err)
	}
	if got := mustRead(t, path); got != "old" {
		t.Fatalf("restored content = %q, want old", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != expected.Mode().Perm() {
		t.Fatalf("restored mode = %o, want original %o", got, expected.Mode().Perm())
	}
	if runtime.GOOS != "windows" && expected.Mode().Perm() != 0o640 {
		t.Fatalf("original mode = %o, want 640", expected.Mode().Perm())
	}
}

func TestWriteFileIfSameWithInfoReturnsMetadataAfterStagedLinkCleanupFailure(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "unrelated"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	expected, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	cleanupErr := errors.New("injected staged-file cleanup failure")
	var stagedName string
	root.renameNoReplaceForTest = func(_ *os.Root, _, _ string) error {
		return secureopen.ErrRenameNoReplaceUnsupported
	}
	root.removeStagedFileForTest = func(_ *os.Root, name string) error {
		stagedName = name
		return cleanupErr
	}

	info, err := root.WriteFileIfSameWithInfo("settings.xcconfig", []byte("updated"), 0o640, expected, []byte("old"), true)
	if err == nil || !errors.Is(err, cleanupErr) || !strings.Contains(err.Error(), "remove staged file") {
		t.Fatalf("WriteFileIfSameWithInfo() error = %v, want staged cleanup failure", err)
	}
	if info == nil {
		t.Fatal("WriteFileIfSameWithInfo() returned nil metadata after successful hard-link publication")
	}
	diskInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat published path: %v", err)
	}
	if !os.SameFile(info, diskInfo) {
		t.Fatal("returned metadata does not identify installed destination")
	}
	if got := mustRead(t, path); got != "updated" {
		t.Fatalf("published content = %q, want updated", got)
	}
	if got := mustRead(t, filepath.Join(dir, "unrelated")); got != "keep" {
		t.Fatalf("unrelated content = %q, want keep", got)
	}
	if stagedName == "" {
		t.Fatal("cleanup seam was not invoked")
	}
	if got := mustRead(t, filepath.Join(dir, stagedName)); got != "updated" {
		t.Fatalf("preserved staged content = %q, want updated", got)
	}
}

func TestWriteFileIfSameWithInfoRetainsStagingMetadataAfterPublishedReopenFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows cannot retain the staging descriptor through publication")
	}
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	expected, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	transient := errors.New("injected published reopen failure")
	root.openPublishedFileForTest = func(*os.Root, string) (*os.File, error) {
		return nil, transient
	}

	info, err := root.WriteFileIfSameWithInfo("settings.xcconfig", []byte("updated"), 0o640, expected, []byte("old"), true)
	if err == nil || !errors.Is(err, transient) {
		t.Fatalf("WriteFileIfSameWithInfo() error = %v, want published reopen failure", err)
	}
	if errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("legacy WriteFileIfSameWithInfo() exposed strict identity sentinel: %v", err)
	}
	if info == nil {
		t.Fatal("WriteFileIfSameWithInfo() returned nil retained staging metadata")
	}
	diskInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat published path: %v", err)
	}
	if !os.SameFile(info, diskInfo) {
		t.Fatal("retained staging metadata does not identify the published destination")
	}
	if got := mustRead(t, path); got != "updated" {
		t.Fatalf("published content = %q, want updated", got)
	}
}

func TestWriteFileIfSameWithInfoReturnsMetadataAfterParentSyncFailure(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	root.simulateWindowsCloseForTest = true
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	expected, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	syncErr := errors.New("injected parent directory sync failure")
	var syncObserved bool
	root.syncDirectoryForTest = func(_ *os.Root) error {
		syncObserved = true
		return syncErr
	}

	info, err := root.WriteFileIfSameWithInfo("settings.xcconfig", []byte("updated"), 0o640, expected, []byte("old"), true)
	if err == nil || !errors.Is(err, syncErr) {
		t.Fatalf("WriteFileIfSameWithInfo() error = %v, want parent sync failure", err)
	}
	if info == nil {
		t.Fatal("WriteFileIfSameWithInfo() returned nil metadata after published sync failure")
	}
	if !syncObserved {
		t.Fatal("parent directory sync hook was not invoked")
	}
	diskInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat published path: %v", err)
	}
	if !os.SameFile(info, diskInfo) {
		t.Fatal("returned metadata does not identify installed destination")
	}
	if got := mustRead(t, path); got != "updated" {
		t.Fatalf("published content = %q, want updated", got)
	}
	matches, globErr := filepath.Glob(filepath.Join(dir, rollbackFilePattern[:len(rollbackFilePattern)-1]+"*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) != 0 {
		t.Fatalf("quarantine entries remain after sync failure: %v", matches)
	}
}

func TestCreateNewFromPreservesStagedReplacementAfterHardLinkCleanupFailure(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	const evidenceName = "staged-original"
	cleanupErr := errors.New("injected staged-file cleanup failure")
	var stagedName string
	root.renameNoReplaceForTest = func(_ *os.Root, _, _ string) error {
		return secureopen.ErrRenameNoReplaceUnsupported
	}
	root.removeStagedFileForTest = func(parent *os.Root, name string) error {
		stagedName = name
		if err := parent.Rename(name, evidenceName); err != nil {
			t.Fatalf("preserve staged inode as evidence: %v", err)
		}
		if err := parent.WriteFile(name, []byte("replacement"), 0o600); err != nil {
			t.Fatalf("install staged-name replacement: %v", err)
		}
		return cleanupErr
	}

	written, err := root.CreateNewFrom("receipt.json", strings.NewReader("complete"), 0o600)
	if written != int64(len("complete")) {
		t.Fatalf("CreateNewFrom() wrote %d bytes, want %d", written, len("complete"))
	}
	if err == nil || !errors.Is(err, cleanupErr) || !strings.Contains(err.Error(), "remove staged file") {
		t.Fatalf("CreateNewFrom() error = %v, want staged cleanup failure", err)
	}
	if got := mustRead(t, filepath.Join(dir, "receipt.json")); got != "complete" {
		t.Fatalf("published content = %q, want complete", got)
	}
	if stagedName == "" {
		t.Fatal("staged cleanup seam was not invoked")
	}
	if got := mustRead(t, filepath.Join(dir, evidenceName)); got != "complete" {
		t.Fatalf("staged evidence content = %q, want complete", got)
	}
	if got := mustRead(t, filepath.Join(dir, stagedName)); got != "replacement" {
		t.Fatalf("staged-name replacement content = %q, want replacement preserved", got)
	}
}

func TestWriteFileIfSameCleansQuarantineAndSyncsAfterPublishedCloseFailure(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	path := filepath.Join(dir, "settings.xcconfig")
	const original = "old"
	if err := os.WriteFile(path, []byte(original), 0o640); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}
	expected, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(source) error = %v", err)
	}
	closeErr := errors.New("injected published close failure")
	syncErr := errors.New("injected cleanup sync failure")
	root.closePublishedFileForTest = func(file *os.File) error {
		if err := file.Close(); err != nil {
			t.Fatalf("close published file in failure hook: %v", err)
		}
		return closeErr
	}
	var syncObserved bool
	root.syncDirectoryForTest = func(_ *os.Root) error {
		syncObserved = true
		return syncErr
	}

	err = root.WriteFileIfSame("settings.xcconfig", []byte("updated"), 0o640, expected, []byte(original), true)
	if err == nil || !errors.Is(err, closeErr) {
		t.Fatalf("WriteFileIfSame() error = %v, want published close failure", err)
	}
	if !errors.Is(err, syncErr) {
		t.Fatalf("WriteFileIfSame() error = %v, want cleanup sync failure", err)
	}
	if !syncObserved {
		t.Fatal("quarantine cleanup did not sync its parent directory")
	}
	if got := mustRead(t, path); got != "updated" {
		t.Fatalf("published content = %q, want updated", got)
	}
	matches, globErr := filepath.Glob(filepath.Join(dir, rollbackFilePattern[:len(rollbackFilePattern)-1]+"*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) != 0 {
		t.Fatalf("quarantine entries remain after close failure cleanup: %v", matches)
	}
}

func TestCreateNewFileFallsBackWhenAtomicRenameIsUnsupported(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	root.renameNoReplaceForTest = func(_ *os.Root, _, _ string) error {
		return secureopen.ErrRenameNoReplaceUnsupported
	}

	if err := root.CreateNewFile("AuthKey.p8", []byte("complete"), 0o600); err != nil {
		t.Fatalf("CreateNewFile() error = %v", err)
	}
	if got := mustRead(t, filepath.Join(dir, "AuthKey.p8")); got != "complete" {
		t.Fatalf("content = %q, want complete", got)
	}
	if err := root.CreateNewFile("AuthKey.p8", []byte("replacement"), 0o600); !errors.Is(err, os.ErrExist) {
		t.Fatalf("overwrite error = %v, want os.ErrExist", err)
	}
	if got := mustRead(t, filepath.Join(dir, "AuthKey.p8")); got != "complete" {
		t.Fatalf("existing content = %q, want complete", got)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".asc-tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain after fallback: %v", matches)
	}
}

func TestCreateNewFileAtomicRejectsUnsupportedRenameWithoutOutput(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	root.renameNoReplaceForTest = func(_ *os.Root, _, _ string) error {
		return secureopen.ErrRenameNoReplaceUnsupported
	}

	err := root.CreateNewFileAtomic("identity.p12.enc", []byte("complete"), 0o600)
	if !errors.Is(err, secureopen.ErrRenameNoReplaceUnsupported) {
		t.Fatalf("CreateNewFileAtomic() error = %v, want unsupported rename", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "identity.p12.enc")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial destination exists: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".asc-tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain after strict failure: %v", matches)
	}

	mustWrite(t, filepath.Join(dir, "identity.p12.enc"), "existing")
	if err := root.CreateNewFileAtomic("identity.p12.enc", []byte("replacement"), 0o600); !errors.Is(err, os.ErrExist) {
		t.Fatalf("overwrite error = %v, want os.ErrExist", err)
	}
	if got := mustRead(t, filepath.Join(dir, "identity.p12.enc")); got != "existing" {
		t.Fatalf("existing content = %q, want existing", got)
	}
}

func TestCheckCreateNewFileAtomicProbesAndRemovesPublication(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	if err := root.CheckCreateNewFileAtomic("probe.p8", 0o600); err != nil {
		t.Fatalf("CheckCreateNewFileAtomic() error = %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "probe.p8")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("probe destination remains: %v", err)
	}
}

func TestCheckCreateNewFileAtomicRejectsUnsupportedRenameWithoutOutput(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	root.renameNoReplaceForTest = func(_ *os.Root, _, _ string) error {
		return secureopen.ErrRenameNoReplaceUnsupported
	}

	err := root.CheckCreateNewFileAtomic("probe.p8", 0o600)
	if !errors.Is(err, secureopen.ErrRenameNoReplaceUnsupported) {
		t.Fatalf("CheckCreateNewFileAtomic() error = %v, want unsupported rename", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "probe.p8")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("probe destination remains after unsupported check: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".asc-tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain after unsupported check: %v", matches)
	}
}

func TestCreateNewFromWriteFailureLeavesNoDestination(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	wantErr := errors.New("simulated reader failure")
	reader := io.MultiReader(strings.NewReader("partial"), errorReader{err: wantErr})

	if _, err := root.CreateNewFrom("identity.p12", reader, 0o600); !errors.Is(err, wantErr) {
		t.Fatalf("CreateNewFrom() error = %v, want %v", err, wantErr)
	}
	if _, err := os.Lstat(filepath.Join(dir, "identity.p12")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial destination exists after failure: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".asc-tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain after failure: %v", matches)
	}
}

func TestReadFileLimitedRejectsOversize(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "artifact.enc"), "12345")
	root := mustRoot(t, dir)

	if _, err := root.ReadFileLimited("artifact.enc", 4); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("ReadFileLimited() error = %v, want size limit", err)
	}
}

func TestReadFileLimitedMaxIntDoesNotOverflow(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "artifact.enc"), "complete")
	root := mustRoot(t, dir)
	data, err := root.ReadFileLimited("artifact.enc", math.MaxInt64)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "complete" {
		t.Fatalf("data = %q", data)
	}
}

func TestCreateNewFileDoesNotEscapeWhenParentIsSwappedAfterValidation(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatalf("create nested directory: %v", err)
	}
	external := t.TempDir()

	root := mustRoot(t, dir)
	root.afterValidationForTest = func() {
		if err := os.Rename(nested, filepath.Join(dir, "nested-original")); err != nil {
			t.Fatalf("swap validated parent: %v", err)
		}
		if err := os.Symlink(external, nested); err != nil {
			t.Fatalf("replace validated parent with symlink: %v", err)
		}
	}

	err := root.CreateNewFile(filepath.Join("nested", "AuthKey.p8"), []byte("private"), 0o600)
	if err != nil {
		t.Fatalf("CreateNewFile() error = %v", err)
	}
	if _, statErr := os.Lstat(filepath.Join(external, "AuthKey.p8")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("CreateNewFile() created a file outside the trusted root: %v", statErr)
	}
	if got := mustRead(t, filepath.Join(dir, "nested-original", "AuthKey.p8")); got != "private" {
		t.Fatalf("CreateNewFile() content in anchored parent = %q", got)
	}
}

func TestMkdirAllRefusesSymlinkedComponent(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "metadata")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	root := mustRoot(t, dir)
	err := root.MkdirAll(filepath.Join("metadata", "en-US"), 0o755)
	if !errors.Is(err, ErrSymlink) {
		t.Fatalf("MkdirAll() error = %v, want ErrSymlink", err)
	}
	if _, statErr := os.Stat(filepath.Join(outside, "en-US")); statErr == nil {
		t.Fatal("MkdirAll() created a directory outside the root")
	}
}

func TestMkdirAllCreatesNestedDirectories(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)

	if err := root.MkdirAll(filepath.Join("metadata", "en-US"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "metadata", "en-US"))
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if !info.IsDir() {
		t.Fatal("MkdirAll() did not create a directory")
	}
}

func TestMkdirAllCreatesMissingRoot(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "created-root")
	root := mustRoot(t, dir)

	if err := root.MkdirAll(".", 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("Stat(root) = %v, %v", info, err)
	}
}

func TestMkdirAllCreatesNestedMissingRootBelowSymlinkedAncestor(t *testing.T) {
	requireSymlinks(t)
	parent := t.TempDir()
	physical := filepath.Join(parent, "physical")
	if err := os.Mkdir(physical, 0o755); err != nil {
		t.Fatal(err)
	}
	selected := filepath.Join(parent, "selected")
	if err := os.Symlink(physical, selected); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(selected, "missing", "root")
	root := mustRoot(t, directory)

	if err := root.WriteFile("sentinel", []byte("selected"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if got := mustRead(t, filepath.Join(physical, "missing", "root", "sentinel")); got != "selected" {
		t.Fatalf("nested missing root content = %q", got)
	}
}

func TestRootedWriteRejectsMissingRootAncestorReplacedBeforeCreation(t *testing.T) {
	requireSymlinks(t)
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	physical := filepath.Join(parent, "physical")
	if err := os.Mkdir(physical, 0o755); err != nil {
		t.Fatal(err)
	}
	selected := filepath.Join(parent, "selected")
	if err := os.Symlink(physical, selected); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(selected, "missing", "root")
	root := mustRoot(t, directory)
	root.beforeCreateRootForTest = func() {
		if err := os.Rename(physical, physical+"-original"); err != nil {
			t.Fatalf("rename selected ancestor: %v", err)
		}
		if err := os.Mkdir(physical, 0o755); err != nil {
			t.Fatalf("replace selected ancestor: %v", err)
		}
	}

	if err := root.WriteFile("escaped", []byte("unsafe"), 0o600); !errors.Is(err, ErrSymlink) {
		t.Fatalf("WriteFile() error = %v, want ErrSymlink", err)
	}
	if _, err := os.Stat(filepath.Join(physical, "missing", "root", "escaped")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("WriteFile() reached replacement ancestor: %v", err)
	}
}

func TestAppendFileRefusesSymlinkAndLeavesModeIntact(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	sentinelDir := t.TempDir()
	sentinelPath := filepath.Join(sentinelDir, "sentinel.txt")
	mustWrite(t, sentinelPath, "original")
	if err := os.Chmod(sentinelPath, 0o644); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}

	if err := os.Symlink(sentinelPath, filepath.Join(dir, "snitch.log")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	root := mustRoot(t, dir)
	if err := root.AppendFile("snitch.log", []byte("entry\n"), 0o600); !errors.Is(err, ErrSymlink) {
		t.Fatalf("AppendFile() error = %v, want ErrSymlink", err)
	}
	if got := mustRead(t, sentinelPath); got != "original" {
		t.Fatalf("sentinel content = %q, want %q", got, "original")
	}
	info, err := os.Lstat(sentinelPath)
	if err != nil {
		t.Fatalf("Lstat() error = %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o644 {
		t.Fatalf("sentinel mode = %v, want 0644", info.Mode().Perm())
	}
}

func TestAppendFileAppendsInRoot(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)

	if err := root.AppendFile("snitch.log", []byte("one\n"), 0o600); err != nil {
		t.Fatalf("AppendFile() error = %v", err)
	}
	if err := root.AppendFile("snitch.log", []byte("two\n"), 0o600); err != nil {
		t.Fatalf("AppendFile() error = %v", err)
	}
	if got := mustRead(t, filepath.Join(dir, "snitch.log")); got != "one\ntwo\n" {
		t.Fatalf("content = %q", got)
	}
}

func TestAppendFileRefusesMultiplyLinkedFile(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	externalPath := filepath.Join(t.TempDir(), "external.log")
	mustWrite(t, externalPath, "external\n")
	if err := os.Chmod(externalPath, 0o644); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}
	linkedPath := filepath.Join(dir, "snitch.log")
	if err := os.Link(externalPath, linkedPath); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}

	if err := root.AppendFile("snitch.log", []byte("entry\n"), 0o600); err == nil {
		t.Fatal("AppendFile() error = nil, want multiply-linked file refusal")
	}
	if got := mustRead(t, externalPath); got != "external\n" {
		t.Fatalf("external hard-link content = %q, want unchanged", got)
	}
	if got := mustRead(t, linkedPath); got != "external\n" {
		t.Fatalf("rooted hard-link content = %q, want unchanged", got)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Lstat(externalPath)
		if err != nil {
			t.Fatalf("Lstat() error = %v", err)
		}
		if info.Mode().Perm() != 0o644 {
			t.Fatalf("external hard-link mode = %v, want unchanged 0644", info.Mode().Perm())
		}
	}
}

func TestAppendFileDoesNotEscapeWhenParentIsSwappedAfterValidation(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	mustWrite(t, filepath.Join(nested, "snitch.log"), "trusted\n")
	external := t.TempDir()
	externalLog := filepath.Join(external, "snitch.log")
	mustWrite(t, externalLog, "external\n")

	root := mustRoot(t, dir)
	root.afterValidationForTest = func() {
		if err := os.Rename(nested, filepath.Join(dir, "nested-original")); err != nil {
			t.Fatalf("swap validated parent: %v", err)
		}
		if err := os.Symlink(external, nested); err != nil {
			t.Fatalf("replace validated parent with symlink: %v", err)
		}
	}

	err := root.AppendFile(filepath.Join("nested", "snitch.log"), []byte("attacker\n"), 0o600)
	if err != nil {
		t.Fatalf("AppendFile() error = %v", err)
	}
	if got := mustRead(t, externalLog); got != "external\n" {
		t.Fatalf("AppendFile() modified a file outside the trusted root: %q", got)
	}
	if got := mustRead(t, filepath.Join(dir, "nested-original", "snitch.log")); got != "trusted\nattacker\n" {
		t.Fatalf("AppendFile() content in anchored parent = %q", got)
	}
}

func TestAppendFileRejectsFinalSymlinkSwappedAfterValidation(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	logPath := filepath.Join(dir, "snitch.log")
	mustWrite(t, logPath, "trusted\n")
	siblingPath := filepath.Join(dir, "sibling.log")
	mustWrite(t, siblingPath, "sibling\n")

	root := mustRoot(t, dir)
	root.afterValidationForTest = func() {
		if err := os.Remove(logPath); err != nil {
			t.Fatalf("remove validated file: %v", err)
		}
		if err := os.Symlink("sibling.log", logPath); err != nil {
			t.Fatalf("replace validated file with symlink: %v", err)
		}
	}

	err := root.AppendFile("snitch.log", []byte("attacker\n"), 0o600)
	if err == nil {
		t.Fatal("AppendFile() succeeded through a swapped final symlink, want an error")
	}
	if got := mustRead(t, siblingPath); got != "sibling\n" {
		t.Fatalf("AppendFile() modified the swapped symlink target: %q", got)
	}
}

func TestAllowingInternalSymlinksAcceptsContainedSymlinkedParent(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	realDir := filepath.Join(dir, "SharedGenerated")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	if err := os.Symlink(realDir, filepath.Join(dir, "Generated")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	root := mustRoot(t, dir).AllowingInternalSymlinks()
	if err := root.WriteFile(filepath.Join("Generated", "Info.plist"), []byte("payload"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if got := mustRead(t, filepath.Join(realDir, "Info.plist")); got != "payload" {
		t.Fatalf("content = %q", got)
	}
}

func TestAllowingInternalSymlinksRejectsEscapingSymlinkedParent(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(dir, "Generated")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	root := mustRoot(t, dir).AllowingInternalSymlinks()
	err := root.WriteFile(filepath.Join("Generated", "Info.plist"), []byte("payload"), 0o644)
	if !errors.Is(err, ErrSymlink) {
		t.Fatalf("WriteFile() error = %v, want ErrSymlink", err)
	}
	if _, statErr := os.Stat(filepath.Join(external, "Info.plist")); statErr == nil {
		t.Fatal("WriteFile() escaped through a symlinked parent")
	}
}

func TestAllowingInternalSymlinksStillRejectsFinalSymlink(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	sentinelPath := filepath.Join(t.TempDir(), "sentinel.txt")
	mustWrite(t, sentinelPath, "original")
	if err := os.Symlink(sentinelPath, filepath.Join(dir, "Info.plist")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	root := mustRoot(t, dir).AllowingInternalSymlinks()
	if err := root.WriteFile("Info.plist", []byte("payload"), 0o644); !errors.Is(err, ErrSymlink) {
		t.Fatalf("WriteFile() error = %v, want ErrSymlink", err)
	}
	if got := mustRead(t, sentinelPath); got != "original" {
		t.Fatalf("sentinel content = %q, want %q", got, "original")
	}
}

func TestCheckContainedRejectsSymlinkedParentForExternalCandidate(t *testing.T) {
	requireSymlinks(t)

	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "Configs")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
	mustWrite(t, filepath.Join(outside, "Shared.xcconfig"), "MARKETING_VERSION = 1.0.0\n")

	root := mustRoot(t, dir)
	err := root.CheckContained(filepath.Join(dir, "Configs", "Shared.xcconfig"))
	if !errors.Is(err, ErrSymlink) {
		t.Fatalf("CheckContained() error = %v, want ErrSymlink", err)
	}
}

func TestErrorMessagesIdentifyRejectedPath(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)

	_, err := root.Resolve("../escape.txt")
	if err == nil {
		t.Fatal("Resolve() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "../escape.txt") {
		t.Fatalf("error %q does not identify the rejected path", err)
	}
}

func TestOpenAbsoluteRootNoFollowUsesCurrentDirectoryAnchor(t *testing.T) {
	workingDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	selected := filepath.Join(workingDir, "selected")
	if err := os.Mkdir(selected, 0o755); err != nil {
		t.Fatal(err)
	}

	openWorkingDir := func() (*os.Root, error) {
		return os.OpenRoot(workingDir)
	}
	denyVolumeRoot := func(string) (*os.Root, error) {
		return nil, os.ErrPermission
	}

	opened, err := openAbsoluteRootNoFollowFrom(
		selected,
		workingDir,
		openWorkingDir,
		denyVolumeRoot,
	)
	if err != nil {
		t.Fatalf("openAbsoluteRootNoFollowFrom() error = %v", err)
	}
	defer opened.Close()
	if _, err := opened.Stat("."); err != nil {
		t.Fatalf("opened selected root is unusable: %v", err)
	}
}

func TestOpenAbsoluteRootNoFollowFailsClosedOutsideCurrentDirectory(t *testing.T) {
	workingDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	outside, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	openWorkingDir := func() (*os.Root, error) {
		return os.OpenRoot(workingDir)
	}
	denyVolumeRoot := func(string) (*os.Root, error) {
		return nil, os.ErrPermission
	}

	opened, err := openAbsoluteRootNoFollowFrom(
		outside,
		workingDir,
		openWorkingDir,
		denyVolumeRoot,
	)
	if opened != nil {
		_ = opened.Close()
		t.Fatal("openAbsoluteRootNoFollowFrom() opened a root outside the current directory")
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("openAbsoluteRootNoFollowFrom() error = %v, want permission denied", err)
	}
}

func TestOpenAbsoluteRootNoFollowRejectsReplacedCurrentDirectoryPath(t *testing.T) {
	requireDirectoryRenameWhileOpen(t)
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	workingDir := filepath.Join(parent, "working")
	if err := os.Mkdir(workingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	openedWorkingDir, err := os.OpenRoot(workingDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(workingDir, workingDir+"-original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(workingDir, 0o755); err != nil {
		t.Fatal(err)
	}

	opened, err := openAbsoluteRootNoFollowFrom(
		workingDir,
		workingDir,
		func() (*os.Root, error) { return openedWorkingDir, nil },
		os.OpenRoot,
	)
	if opened != nil {
		_ = opened.Close()
		t.Fatal("openAbsoluteRootNoFollowFrom() accepted a replaced working directory path")
	}
	if !errors.Is(err, ErrSymlink) {
		t.Fatalf("openAbsoluteRootNoFollowFrom() error = %v, want ErrSymlink", err)
	}
}

func TestOpenRootPinsOriginalDirectoryAcrossPathReplacement(t *testing.T) {
	requireSymlinks(t)
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(parent, "root")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	root := mustRoot(t, directory)
	opened, err := root.OpenRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	if err := os.Rename(directory, directory+"-original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), directory); err != nil {
		t.Fatal(err)
	}
	if err := opened.WriteFile("sentinel", []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(directory+"-original", "sentinel")); err != nil {
		t.Fatalf("pinned root did not address original directory: %v", err)
	}
}

func TestContainsPathUsesPinnedRootIdentity(t *testing.T) {
	requireSymlinks(t)
	parent := t.TempDir()
	selected := filepath.Join(parent, "selected")
	inside := filepath.Join(selected, "state", "receipt.json")
	outside := filepath.Join(parent, "outside", "receipt.json")
	if err := os.MkdirAll(filepath.Dir(inside), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(outside), 0o755); err != nil {
		t.Fatal(err)
	}
	root := mustRoot(t, selected)

	for path, want := range map[string]bool{inside: true, outside: false} {
		got, err := root.ContainsPath(path)
		if err != nil {
			t.Fatalf("ContainsPath(%q) error = %v", path, err)
		}
		if got != want {
			t.Fatalf("ContainsPath(%q) = %t, want %t", path, got, want)
		}
	}

	if err := os.Rename(selected, selected+"-original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(selected, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := root.ContainsPath(filepath.Join(selected, "receipt.json")); !errors.Is(err, ErrSymlink) {
		t.Fatalf("ContainsPath() after root replacement error = %v, want ErrSymlink", err)
	}
}

func TestContainsAnchoredPathRejectsPathSubstitution(t *testing.T) {
	requireDirectoryRenameWhileOpen(t)
	parent := t.TempDir()
	bundle := filepath.Join(parent, "bundle")
	artifact := filepath.Join(bundle, "state")
	if err := os.MkdirAll(artifact, 0o755); err != nil {
		t.Fatal(err)
	}
	bundleRoot := mustRoot(t, bundle)
	anchored, err := os.OpenRoot(artifact)
	if err != nil {
		t.Fatal(err)
	}
	defer anchored.Close()
	contained, err := bundleRoot.ContainsAnchoredPath(artifact, anchored)
	if err != nil || !contained {
		t.Fatalf("ContainsAnchoredPath() = %t, %v, want true", contained, err)
	}
	if err := os.Rename(artifact, artifact+"-original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(artifact, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := bundleRoot.ContainsAnchoredPath(artifact, anchored); !errors.Is(err, ErrSymlink) {
		t.Fatalf("ContainsAnchoredPath() replacement error = %v, want ErrSymlink", err)
	}
}

func TestContainmentUsesIdentityOnCaseInsensitiveVolume(t *testing.T) {
	parent := t.TempDir()
	selected := filepath.Join(parent, "PreparedBundle")
	if err := os.Mkdir(selected, 0o755); err != nil {
		t.Fatal(err)
	}
	caseAlias := filepath.Join(parent, "preparedbundle")
	selectedInfo, err := os.Stat(selected)
	if err != nil {
		t.Fatal(err)
	}
	aliasInfo, err := os.Stat(caseAlias)
	if err != nil || !os.SameFile(selectedInfo, aliasInfo) {
		t.Skip("test volume is case-sensitive")
	}
	root := mustRoot(t, selected)
	contained, err := root.ContainsPath(filepath.Join(caseAlias, "state", "receipt.json"))
	if err != nil || !contained {
		t.Fatalf("ContainsPath() = %t, %v, want identity-based containment", contained, err)
	}
	anchored, err := os.OpenRoot(caseAlias)
	if err != nil {
		t.Fatal(err)
	}
	defer anchored.Close()
	contained, err = root.ContainsAnchoredPath(caseAlias, anchored)
	if err != nil || !contained {
		t.Fatalf("ContainsAnchoredPath() = %t, %v, want identity-based containment", contained, err)
	}
}

func TestRootCloseReleasesSharedPinnedIdentity(t *testing.T) {
	root := mustRoot(t, t.TempDir())
	copy := root
	if err := copy.Close(); err != nil {
		t.Fatal(err)
	}
	if err := root.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	opened, err := root.OpenRoot()
	if opened != nil {
		_ = opened.Close()
		t.Fatal("OpenRoot() succeeded after a copied Root was closed")
	}
	if !errors.Is(err, ErrSymlink) {
		t.Fatalf("OpenRoot() error = %v, want ErrSymlink", err)
	}
}

func TestOpenRootRejectsSelectedPathSwappedBeforeOpen(t *testing.T) {
	requireSymlinks(t)
	for _, test := range []struct {
		name         string
		rootRelative string
		swapRelative string
		outsideRoot  string
	}{
		{name: "final root", rootRelative: "root", swapRelative: "root"},
		{name: "intermediate directory", rootRelative: filepath.Join("selected", "root"), swapRelative: "selected", outsideRoot: "root"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.outsideRoot != "" {
				requireDirectoryRenameWhileOpen(t)
			}
			parent, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			directory := filepath.Join(parent, test.rootRelative)
			if err := os.MkdirAll(directory, 0o755); err != nil {
				t.Fatal(err)
			}
			swapPath := filepath.Join(parent, test.swapRelative)
			outside, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if test.outsideRoot != "" {
				if err := os.Mkdir(filepath.Join(outside, test.outsideRoot), 0o755); err != nil {
					t.Fatal(err)
				}
			}

			root := mustRoot(t, directory)
			root.beforeOpenRootForTest = func() {
				if err := os.Rename(swapPath, swapPath+"-original"); err != nil {
					t.Fatalf("rename selected path: %v", err)
				}
				if err := os.Symlink(outside, swapPath); err != nil {
					t.Fatalf("replace selected path with symlink: %v", err)
				}
			}

			opened, openErr := root.OpenRoot()
			if opened != nil {
				if openErr == nil {
					_ = opened.WriteFile("escaped", []byte("unsafe"), 0o600)
				}
				_ = opened.Close()
			}
			if !errors.Is(openErr, ErrSymlink) {
				t.Fatalf("OpenRoot() error = %v, want ErrSymlink", openErr)
			}
			if _, err := os.Lstat(filepath.Join(outside, test.outsideRoot, "escaped")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("OpenRoot() wrote through substituted path: %v", err)
			}
		})
	}
}

func TestOpenRootRejectsSelectedDirectoryReplacedBeforeOpen(t *testing.T) {
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(parent, "root")
	replacement := filepath.Join(parent, "replacement")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(replacement, 0o755); err != nil {
		t.Fatal(err)
	}

	root := mustRoot(t, directory)
	root.beforeOpenRootForTest = func() {
		if err := os.Rename(directory, directory+"-original"); err != nil {
			t.Fatalf("rename selected root: %v", err)
		}
		if err := os.Rename(replacement, directory); err != nil {
			t.Fatalf("replace selected root: %v", err)
		}
	}

	writeErr := root.WriteFile("escaped", []byte("unsafe"), 0o600)
	if !errors.Is(writeErr, ErrSymlink) {
		t.Fatalf("WriteFile() error = %v, want ErrSymlink", writeErr)
	}
	if _, err := os.Stat(filepath.Join(directory, "escaped")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("WriteFile() reached replacement directory: %v", err)
	}
}

func TestRootedWriteRejectsSelectedDirectoryRecreatedBeforeOpen(t *testing.T) {
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(parent, "root")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	root := mustRoot(t, directory)
	root.beforeOpenRootForTest = func() {
		if err := os.Remove(directory); err != nil {
			t.Fatalf("remove selected root: %v", err)
		}
		if err := os.Mkdir(directory, 0o755); err != nil {
			t.Fatalf("recreate selected root: %v", err)
		}
	}
	if err := root.WriteFile("escaped", []byte("unsafe"), 0o600); !errors.Is(err, ErrSymlink) {
		t.Fatalf("WriteFile() error = %v, want ErrSymlink", err)
	}
	if _, err := os.Stat(filepath.Join(directory, "escaped")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("WriteFile() reached recreated directory: %v", err)
	}
}

func TestOpenRootDoesNotAdoptDirectoryCreatedAfterNew(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "not-yet-selected")
	root, err := New(directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	opened, err := root.OpenRoot()
	if opened != nil {
		_ = opened.Close()
		t.Fatal("OpenRoot() adopted a directory created after New")
	}
	if !errors.Is(err, ErrSymlink) {
		t.Fatalf("OpenRoot() error = %v, want ErrSymlink", err)
	}
}

func TestRootedWriteRejectsIntermediateDirectoryReplacedBeforeOpen(t *testing.T) {
	requireDirectoryRenameWhileOpen(t)
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	selectedParent := filepath.Join(parent, "selected")
	directory := filepath.Join(selectedParent, "root")
	replacementParent := filepath.Join(parent, "replacement")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(replacementParent, 0o755); err != nil {
		t.Fatal(err)
	}
	root := mustRoot(t, directory)
	root.beforeOpenRootForTest = func() {
		if err := os.Rename(selectedParent, selectedParent+"-original"); err != nil {
			t.Fatalf("rename selected parent: %v", err)
		}
		if err := os.Rename(replacementParent, selectedParent); err != nil {
			t.Fatalf("replace selected parent: %v", err)
		}
	}
	if err := root.WriteFile("escaped", []byte("unsafe"), 0o600); err == nil {
		t.Fatal("WriteFile() succeeded through replacement ancestor")
	}
	if _, err := os.Stat(filepath.Join(directory, "escaped")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("WriteFile() reached replacement ancestor: %v", err)
	}
	if _, err := os.Stat(directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("WriteFile() created the selected root in replacement ancestor: %v", err)
	}
}

func TestOpenRootPreservesSelectedSymlinkedParentLayout(t *testing.T) {
	requireSymlinks(t)
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	physicalParent := filepath.Join(parent, "physical")
	directory := filepath.Join(physicalParent, "root")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	symlinkedParent := filepath.Join(parent, "selected")
	if err := os.Symlink(physicalParent, symlinkedParent); err != nil {
		t.Fatal(err)
	}

	root := mustRoot(t, filepath.Join(symlinkedParent, "root"))
	opened, err := root.OpenRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	if err := opened.WriteFile("sentinel", []byte("selected"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := mustRead(t, filepath.Join(directory, "sentinel")); got != "selected" {
		t.Fatalf("pinned symlinked parent content = %q", got)
	}
}

func TestMissingNestedRootResolvesNearestSymlinkedAncestor(t *testing.T) {
	requireSymlinks(t)
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	physical := filepath.Join(parent, "physical")
	if err := os.Mkdir(physical, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(parent, "link")
	if err := os.Symlink(physical, link); err != nil {
		t.Fatal(err)
	}
	root := mustRoot(t, filepath.Join(link, "one", "two", "root"))
	defer root.Close()
	if err := root.MkdirAll(".", 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := root.WriteFile("sentinel", []byte("selected"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(physical, "one", "two", "root", "sentinel"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "selected" {
		t.Fatalf("sentinel = %q", data)
	}
}

func TestMissingRootResolvesNestedSymlinkPrefix(t *testing.T) {
	requireSymlinks(t)
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(parent, "first")
	target := filepath.Join(parent, "target")
	if err := os.Mkdir(first, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(first, filepath.Join(parent, "outer")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(first, "inner")); err != nil {
		t.Fatal(err)
	}
	root := mustRoot(t, filepath.Join(parent, "outer", "inner", "new", "root"))
	defer root.Close()
	if err := root.MkdirAll(".", 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if info, err := os.Stat(filepath.Join(target, "new", "root")); err != nil || !info.IsDir() {
		t.Fatalf("physical nested root stat = %#v, %v", info, err)
	}
}

func TestNewRejectsInvalidExistingAncestorWithoutCreating(t *testing.T) {
	requireSymlinks(t)
	parent := t.TempDir()
	t.Run("dangling symlink", func(t *testing.T) {
		link := filepath.Join(parent, "dangling")
		if err := os.Symlink(filepath.Join(parent, "absent"), link); err != nil {
			t.Fatal(err)
		}
		if _, err := New(filepath.Join(link, "new", "root")); err == nil {
			t.Fatal("New() accepted a dangling symlink ancestor")
		}
		if _, err := os.Stat(filepath.Join(parent, "absent")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("New() created through dangling symlink: %v", err)
		}
	})
	t.Run("symlink loop", func(t *testing.T) {
		loop := filepath.Join(parent, "loop")
		if err := os.Symlink("loop", loop); err != nil {
			t.Fatal(err)
		}
		if _, err := New(filepath.Join(loop, "new", "root")); err == nil {
			t.Fatal("New() accepted a symlink loop ancestor")
		}
	})
	t.Run("regular file", func(t *testing.T) {
		file := filepath.Join(parent, "file")
		if err := os.WriteFile(file, []byte("file"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := New(filepath.Join(file, "new", "root")); err == nil {
			t.Fatal("New() accepted a non-directory ancestor")
		}
		if data, err := os.ReadFile(file); err != nil || string(data) != "file" {
			t.Fatalf("ancestor file changed: %q, %v", data, err)
		}
	})
}

func TestMissingRootRejectsAncestorReplacementBeforeCreation(t *testing.T) {
	requireSymlinks(t)
	for _, test := range []struct {
		name    string
		symlink bool
	}{
		{name: "ordinary ancestor"},
		{name: "symlinked ancestor", symlink: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			parent, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			original := filepath.Join(parent, "original")
			replacement := filepath.Join(parent, "replacement")
			if err := os.Mkdir(original, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(replacement, 0o755); err != nil {
				t.Fatal(err)
			}
			selectedBase := original
			if test.symlink {
				selectedBase = filepath.Join(parent, "selected")
				if err := os.Symlink(original, selectedBase); err != nil {
					t.Fatal(err)
				}
			}
			root := mustRoot(t, filepath.Join(selectedBase, "new", "root"))
			defer root.Close()
			root.beforeCreateRootForTest = func() {
				if test.symlink {
					if err := os.Rename(selectedBase, selectedBase+"-original"); err != nil {
						t.Fatalf("rename selected symlink: %v", err)
					}
					if err := os.Symlink(replacement, selectedBase); err != nil {
						t.Fatalf("replace selected symlink: %v", err)
					}
					return
				}
				if err := os.Rename(original, original+"-original"); err != nil {
					t.Fatalf("rename selected ancestor: %v", err)
				}
				if err := os.Rename(replacement, original); err != nil {
					t.Fatalf("replace selected ancestor: %v", err)
				}
			}
			if err := root.MkdirAll(".", 0o755); !errors.Is(err, ErrSymlink) {
				t.Fatalf("MkdirAll() error = %v, want ErrSymlink", err)
			}
			for _, directory := range []string{original, original + "-original", replacement} {
				if _, err := os.Stat(filepath.Join(directory, "new")); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("partial root created beneath %q: %v", directory, err)
				}
			}
		})
	}
}

func TestValidateMissingRootComponent(t *testing.T) {
	for _, component := range []string{"", ".", "..", string([]byte{'b', 'a', 'd', 0})} {
		if err := validateMissingRootComponent(component); !errors.Is(err, ErrEscapesRoot) {
			t.Fatalf("validateMissingRootComponent(%q) error = %v, want ErrEscapesRoot", component, err)
		}
	}
	for _, component := range []string{"root", "legacy-name", "a.b"} {
		if err := validateMissingRootComponent(component); err != nil {
			t.Fatalf("validateMissingRootComponent(%q) error = %v", component, err)
		}
	}
}

func TestCloseReleasesPendingRootAncestorDescriptors(t *testing.T) {
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	before := openDescriptorCount(t)
	const rootCount = 64
	roots := make([]Root, 0, rootCount)
	for index := 0; index < rootCount; index++ {
		roots = append(roots, mustRoot(t, filepath.Join(parent, fmt.Sprintf("missing-%d", index), "root")))
	}
	for _, root := range roots {
		if err := root.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}
	after := openDescriptorCount(t)
	if after > before+4 {
		t.Fatalf("open descriptors after pending-root Close = %d, baseline %d", after, before)
	}
}

func TestCloseReleasesPinnedDescriptorAcrossRootCopies(t *testing.T) {
	root := mustRoot(t, t.TempDir())
	copyOfRoot := root

	if err := root.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if opened, err := copyOfRoot.OpenRoot(); err == nil {
		_ = opened.Close()
		t.Fatal("copied Root remained usable after shared Close")
	}
	if err := copyOfRoot.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func TestCloseReleasesPinnedDescriptorsDeterministically(t *testing.T) {
	before := openDescriptorCount(t)
	const rootCount = 128
	roots := make([]Root, 0, rootCount)
	for range rootCount {
		roots = append(roots, mustRoot(t, t.TempDir()))
	}
	for _, root := range roots {
		if err := root.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}
	after := openDescriptorCount(t)
	if after > before+4 {
		t.Fatalf("open descriptors after Close = %d, baseline %d", after, before)
	}
}

func openDescriptorCount(t *testing.T) int {
	t.Helper()
	for _, directory := range []string{"/dev/fd", "/proc/self/fd"} {
		entries, err := os.ReadDir(directory)
		if err == nil {
			return len(entries)
		}
	}
	t.Skip("open descriptor enumeration is unavailable")
	return 0
}

func mustRoot(t *testing.T, path string) Root {
	t.Helper()
	root, err := New(path)
	if err != nil {
		t.Fatalf("New(%q) error = %v", path, err)
	}
	return root
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return string(data)
}

func temporaryLeftovers(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v", dir, err)
	}
	var leftovers []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".asc-tmp-") {
			leftovers = append(leftovers, entry.Name())
		}
	}
	return leftovers
}

func requireDirectoryRenameWhileOpen(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Windows cannot rename a directory while an os.Root handle to it or a descendant is open")
	}
}

func requireSymlinks(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "windows" {
		return
	}
	dir := t.TempDir()
	if err := os.Symlink(filepath.Join(dir, "target"), filepath.Join(dir, "link")); err != nil {
		t.Skip("symlink creation is not permitted on this host")
	}
}

func TestChmodFileTightensContainedFileWithoutReadAccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose POSIX permission bits")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses owner read permission")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "secret.p8")
	if err := os.WriteFile(path, []byte("key"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Chmod(path, 0o044); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}

	err := ChmodFile(path, 0o600)
	if runtime.GOOS != "linux" {
		if err == nil {
			t.Fatal("ChmodFile() repaired an unreadable file without a secure descriptor mechanism")
		}
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Fatalf("Stat() error = %v", statErr)
		}
		if info.Mode().Perm() != 0o044 {
			t.Fatalf("permissions = %#o, want 0044 unchanged after fail-closed result", info.Mode().Perm())
		}
		return
	}
	if err != nil {
		t.Fatalf("ChmodFile() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permissions = %#o, want 0600", info.Mode().Perm())
	}
}

func TestChmodFileRefusesSymlinkAndDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose POSIX permission bits")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "secret.p8")
	if err := os.WriteFile(target, []byte("key"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	link := filepath.Join(dir, "link.p8")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	if err := ChmodFile(link, 0o600); !errors.Is(err, ErrSymlink) {
		t.Fatalf("ChmodFile(symlink) error = %v, want ErrSymlink", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("target permissions = %#o, want 0644 untouched", info.Mode().Perm())
	}
	if err := ChmodFile(dir, 0o600); err == nil {
		t.Fatal("expected ChmodFile(directory) to fail")
	}
	if err := ChmodFile(filepath.Join(dir, "missing.p8"), 0o600); err == nil {
		t.Fatal("expected ChmodFile(missing) to fail")
	}
}

func TestChmodFileRejectsFinalSymlinkReplacement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose POSIX permission bits")
	}
	requireSymlinks(t)

	dir := t.TempDir()
	target := filepath.Join(dir, "key.p8")
	original := filepath.Join(dir, "key-original.p8")
	external := filepath.Join(t.TempDir(), "external.p8")
	mustWrite(t, target, "original")
	mustWrite(t, external, "external")
	if err := os.Chmod(target, 0o644); err != nil {
		t.Fatalf("Chmod(target) error = %v", err)
	}
	if err := os.Chmod(external, 0o644); err != nil {
		t.Fatalf("Chmod(external) error = %v", err)
	}

	root := mustRoot(t, dir)
	root.afterValidationForTest = func() {
		if err := os.Rename(target, original); err != nil {
			t.Fatalf("rename validated file: %v", err)
		}
		if err := os.Symlink(external, target); err != nil {
			t.Fatalf("replace validated file with symlink: %v", err)
		}
	}

	if err := root.ChmodFile("key.p8", 0o600); err == nil {
		t.Fatal("ChmodFile() accepted a final symlink replacement")
	}
	for _, path := range []string{original, external} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat(%q) error = %v", path, err)
		}
		if info.Mode().Perm() != 0o644 {
			t.Fatalf("%s mode = %#o, want 0644 untouched", path, info.Mode().Perm())
		}
	}
}

func TestChmodFileRejectsFinalDirectoryReplacement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose POSIX permission bits")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "key.p8")
	original := filepath.Join(dir, "key-original.p8")
	mustWrite(t, target, "original")
	if err := os.Chmod(target, 0o644); err != nil {
		t.Fatalf("Chmod(target) error = %v", err)
	}

	root := mustRoot(t, dir)
	root.afterValidationForTest = func() {
		if err := os.Rename(target, original); err != nil {
			t.Fatalf("rename validated file: %v", err)
		}
		if err := os.Mkdir(target, 0o755); err != nil {
			t.Fatalf("replace validated file with directory: %v", err)
		}
		if err := os.Chmod(target, 0o755); err != nil {
			t.Fatalf("normalize replacement directory mode: %v", err)
		}
	}

	if err := root.ChmodFile("key.p8", 0o600); err == nil {
		t.Fatal("ChmodFile() accepted a final directory replacement")
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat(replacement) error = %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("replacement directory mode = %#o, want 0755 untouched", info.Mode().Perm())
	}
}

func TestChmodFileRejectsFinalRegularFileReplacement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose POSIX permission bits")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "key.p8")
	original := filepath.Join(dir, "key-original.p8")
	mustWrite(t, target, "original")
	if err := os.Chmod(target, 0o644); err != nil {
		t.Fatalf("Chmod(target) error = %v", err)
	}

	root := mustRoot(t, dir)
	root.afterValidationForTest = func() {
		if err := os.Rename(target, original); err != nil {
			t.Fatalf("rename validated file: %v", err)
		}
		mustWrite(t, target, "replacement")
		if err := os.Chmod(target, 0o644); err != nil {
			t.Fatalf("normalize replacement mode: %v", err)
		}
	}

	err := root.ChmodFile("key.p8", 0o600)
	if !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("ChmodFile() error = %v, want ErrFileIdentityChanged", err)
	}
	for _, path := range []string{original, target} {
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Fatalf("Stat(%q) error = %v", path, statErr)
		}
		if info.Mode().Perm() != 0o644 {
			t.Fatalf("%s mode = %#o, want 0644 untouched", path, info.Mode().Perm())
		}
	}
}

func TestChmodFileIfSameRejectsReplacementBeforeRootedValidation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose POSIX permission bits")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "key.p8")
	original := filepath.Join(dir, "key-original.p8")
	mustWrite(t, target, "original")
	if err := os.Chmod(target, 0o644); err != nil {
		t.Fatalf("Chmod(target) error = %v", err)
	}
	expected, err := os.Lstat(target)
	if err != nil {
		t.Fatalf("Lstat(target) error = %v", err)
	}

	if err := os.Rename(target, original); err != nil {
		t.Fatalf("rename original: %v", err)
	}
	mustWrite(t, target, "replacement")
	if err := os.Chmod(target, 0o644); err != nil {
		t.Fatalf("normalize replacement mode: %v", err)
	}

	root := mustRoot(t, dir)
	err = root.ChmodFileIfSame("key.p8", expected, 0o600)
	if !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("ChmodFileIfSame() error = %v, want ErrFileIdentityChanged", err)
	}
	for _, path := range []string{original, target} {
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Fatalf("Stat(%q) error = %v", path, statErr)
		}
		if info.Mode().Perm() != 0o644 {
			t.Fatalf("%s mode = %#o, want 0644 untouched", path, info.Mode().Perm())
		}
	}
}

func TestCheckContainedPathRejectsSymlinks(t *testing.T) {
	t.Run("final component", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "target")
		if err := os.WriteFile(target, []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(dir, "link")
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if err := CheckContainedPath(link); !errors.Is(err, ErrSymlink) {
			t.Fatalf("CheckContainedPath() error = %v, want ErrSymlink", err)
		}
	})

	t.Run("parent component", func(t *testing.T) {
		dir := t.TempDir()
		targetDir := filepath.Join(dir, "target")
		if err := os.Mkdir(targetDir, 0o700); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(targetDir, "file")
		if err := os.WriteFile(target, []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
		linkDir := filepath.Join(dir, "link")
		if err := os.Symlink(targetDir, linkDir); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if err := CheckContainedPath(filepath.Join(linkDir, "file")); !errors.Is(err, ErrSymlink) {
			t.Fatalf("CheckContainedPath() error = %v, want ErrSymlink", err)
		}
	})

	t.Run("regular file", func(t *testing.T) {
		target := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(target, []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := CheckContainedPath(target); err != nil {
			t.Fatalf("CheckContainedPath() error = %v", err)
		}
	})
}

func TestChmodFileIfSameAcceptsDarwinTmpAlias(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("/tmp is a system symlink on Darwin")
	}
	dir, err := os.MkdirTemp("/tmp", "asc-rootfs-chmod-")
	if err != nil {
		t.Fatalf("MkdirTemp(/tmp) error = %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	target := filepath.Join(dir, "key.p8")
	mustWrite(t, target, "private key")
	if err := os.Chmod(target, 0o644); err != nil {
		t.Fatalf("Chmod(target) error = %v", err)
	}
	expected, err := os.Lstat(target)
	if err != nil {
		t.Fatalf("Lstat(target) error = %v", err)
	}

	if err := ChmodFileIfSame(target, expected, 0o600); err != nil {
		t.Fatalf("ChmodFileIfSame(%q) error = %v", target, err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat(target) error = %v", err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o600); got != want {
		t.Fatalf("target mode = %#o, want %#o", got, want)
	}
}

func TestChmodFileIfSameDarwinTmpAliasRejectsNestedSymlink(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("/tmp is a system symlink on Darwin")
	}
	dir, err := os.MkdirTemp("/tmp", "asc-rootfs-symlink-")
	if err != nil {
		t.Fatalf("MkdirTemp(/tmp) error = %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	outside := t.TempDir()
	target := filepath.Join(outside, "key.p8")
	mustWrite(t, target, "private key")
	if err := os.Chmod(target, 0o644); err != nil {
		t.Fatalf("Chmod(target) error = %v", err)
	}
	linkedParent := filepath.Join(dir, "linked")
	if err := os.Symlink(outside, linkedParent); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
	expected, err := os.Lstat(filepath.Join(linkedParent, "key.p8"))
	if err != nil {
		t.Fatalf("Lstat(linked target) error = %v", err)
	}

	err = ChmodFileIfSame(filepath.Join(linkedParent, "key.p8"), expected, 0o600)
	if !errors.Is(err, ErrSymlink) {
		t.Fatalf("ChmodFileIfSame() error = %v, want ErrSymlink", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat(target) error = %v", err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o644); got != want {
		t.Fatalf("symlink target mode = %#o, want %#o untouched", got, want)
	}
}

func TestChmodFileMutatesRetainedDescriptorAfterPathReplacement(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose POSIX permission bits")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "key.p8")
	original := filepath.Join(dir, "key-original.p8")
	mustWrite(t, target, "original")
	if err := os.Chmod(target, 0o644); err != nil {
		t.Fatalf("Chmod(target) error = %v", err)
	}

	root := mustRoot(t, dir)
	root.afterChmodOpenForTest = func() {
		if err := os.Rename(target, original); err != nil {
			t.Fatalf("rename opened file: %v", err)
		}
		mustWrite(t, target, "replacement")
		if err := os.Chmod(target, 0o644); err != nil {
			t.Fatalf("normalize replacement mode: %v", err)
		}
	}

	if err := root.ChmodFile("key.p8", 0o600); err != nil {
		t.Fatalf("ChmodFile() error = %v", err)
	}
	originalInfo, err := os.Stat(original)
	if err != nil {
		t.Fatalf("Stat(original) error = %v", err)
	}
	replacementInfo, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat(replacement) error = %v", err)
	}
	if originalInfo.Mode().Perm() != 0o600 {
		t.Fatalf("retained original mode = %#o, want 0600", originalInfo.Mode().Perm())
	}
	if replacementInfo.Mode().Perm() != 0o644 {
		t.Fatalf("replacement mode = %#o, want 0644 untouched", replacementInfo.Mode().Perm())
	}
}
