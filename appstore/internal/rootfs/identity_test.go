package rootfs

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/secureopen"
)

func TestCaptureFileIdentityRejectsSameContentReplacement(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	const original = "same bytes"
	if err := os.WriteFile(filepath.Join(dir, "settings.xcconfig"), []byte(original), 0o640); err != nil {
		t.Fatal(err)
	}

	identity, err := root.CaptureFile("settings.xcconfig")
	if err != nil {
		t.Fatalf("CaptureFile() error = %v", err)
	}
	if string(identity.Data()) != original {
		t.Fatalf("captured data = %q, want %q", identity.Data(), original)
	}
	if err := os.WriteFile(filepath.Join(dir, "replacement.xcconfig"), []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(dir, "replacement.xcconfig"), filepath.Join(dir, "settings.xcconfig")); err != nil {
		t.Fatal(err)
	}

	if _, err := root.ReplaceFileIfSame("settings.xcconfig", identity, []byte("updated"), 0o640, true); !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("ReplaceFileIfSame() error = %v, want ErrFileIdentityChanged", err)
	}
	if got := mustRead(t, filepath.Join(dir, "settings.xcconfig")); got != original {
		t.Fatalf("replacement content = %q, want %q", got, original)
	}
}

func TestCaptureFileLimitedRejectsTornSnapshot(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("before"), 0o640); err != nil {
		t.Fatal(err)
	}
	var callbackCalls int
	root.afterIdentityCaptureReadForTest = func() {
		callbackCalls++
		if err := os.WriteFile(path, []byte("after"), 0o640); err != nil {
			t.Fatalf("replace captured contents: %v", err)
		}
	}

	identity, err := root.CaptureFileLimited("settings.xcconfig", int64(len("before")))
	if identity != nil {
		t.Fatal("CaptureFileLimited() returned an identity for a torn snapshot")
	}
	if !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("CaptureFileLimited() error = %v, want ErrFileIdentityChanged", err)
	}
	if callbackCalls != 1 {
		t.Fatalf("identity capture hook calls = %d, want one", callbackCalls)
	}
	if got := mustRead(t, path); got != "after" {
		t.Fatalf("post-race contents = %q, want after", got)
	}
}

func TestCaptureFileLimitedRejectsSameContentPathReplacementDuringCapture(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	const content = "same bytes"
	path := filepath.Join(dir, "settings.xcconfig")
	replacementPath := filepath.Join(dir, "replacement.xcconfig")
	if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(replacementPath, []byte(content), 0o640); err != nil {
		t.Fatal(err)
	}
	root.afterIdentityCaptureReadForTest = func() {
		if err := os.Rename(replacementPath, path); err != nil {
			t.Fatalf("replace capture path: %v", err)
		}
	}

	identity, err := root.CaptureFileLimited("settings.xcconfig", int64(len(content)))
	if identity != nil {
		t.Fatal("CaptureFileLimited() returned an identity after path replacement")
	}
	if !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("CaptureFileLimited() error = %v, want ErrFileIdentityChanged", err)
	}
	if got := mustRead(t, path); got != content {
		t.Fatalf("replacement contents = %q, want %q", got, content)
	}
}

func TestCaptureFileLimitedRejectsHardLinkCreatedDuringCapture(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	const content = "settings"
	path := filepath.Join(dir, "settings.xcconfig")
	linkPath := filepath.Join(dir, "settings-link.xcconfig")
	if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
		t.Fatal(err)
	}
	root.afterIdentityCaptureReadForTest = func() {
		if err := os.Link(path, linkPath); err != nil {
			t.Skipf("hard links are unavailable: %v", err)
		}
	}

	identity, err := root.CaptureFileLimited("settings.xcconfig", int64(len(content)))
	if identity != nil {
		t.Fatal("CaptureFileLimited() returned an identity after hard-link state changed")
	}
	if !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("CaptureFileLimited() error = %v, want ErrFileIdentityChanged", err)
	}
	if got := mustRead(t, path); got != content {
		t.Fatalf("destination contents = %q, want %q", got, content)
	}
	if got := mustRead(t, linkPath); got != content {
		t.Fatalf("hard-link contents = %q, want %q", got, content)
	}
}

func TestCaptureFileIdentityRejectsSymlinkAndFIFO(t *testing.T) {
	t.Run("symlink", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "special")
		if err := os.Symlink("target", path); err != nil {
			t.Skipf("symlink creation is not permitted: %v", err)
		}
		root := mustRoot(t, dir)
		t.Cleanup(func() { _ = root.Close() })
		if identity, err := root.CaptureFile("special"); identity != nil || err == nil {
			t.Fatalf("CaptureFile() identity = %v, error = %v; want symlink rejection", identity, err)
		}
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatalf("Lstat(symlink) error = %v", err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("special mode = %v, want symlink preserved", info.Mode())
		}
	})

	t.Run("fifo", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("FIFO creation is unavailable on Windows")
		}
		mkfifo, err := exec.LookPath("mkfifo")
		if err != nil {
			t.Skipf("mkfifo is unavailable: %v", err)
		}
		dir := t.TempDir()
		path := filepath.Join(dir, "special")
		if err := exec.Command(mkfifo, path).Run(); err != nil {
			t.Skipf("FIFO creation is unavailable: %v", err)
		}
		root := mustRoot(t, dir)
		t.Cleanup(func() { _ = root.Close() })
		if identity, err := root.CaptureFile("special"); identity != nil || err == nil {
			t.Fatalf("CaptureFile() identity = %v, error = %v; want FIFO rejection", identity, err)
		}
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatalf("Lstat(FIFO) error = %v", err)
		}
		if info.Mode()&os.ModeNamedPipe == 0 {
			t.Fatalf("special mode = %v, want FIFO preserved", info.Mode())
		}
	})
}

func TestFileIdentityIsInvalidAfterRootClose(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "settings.xcconfig"), []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("settings.xcconfig")
	if err != nil {
		t.Fatalf("CaptureFile() error = %v", err)
	}
	if err := root.Close(); err != nil {
		t.Fatalf("Root.Close() error = %v", err)
	}
	if _, err := root.ReplaceFileIfSame("settings.xcconfig", identity, []byte("new"), 0o640, true); !errors.Is(err, ErrFileIdentityClosed) {
		t.Fatalf("ReplaceFileIfSame() error = %v, want ErrFileIdentityClosed", err)
	}
}

func TestCheckFileIdentityRejectsSameContentReplacement(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	const content = "same bytes"
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("settings.xcconfig")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "replacement"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(dir, "replacement"), path); err != nil {
		t.Fatal(err)
	}
	if err := root.CheckFileIdentity("settings.xcconfig", identity); !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("CheckFileIdentity() error = %v, want ErrFileIdentityChanged", err)
	}
}

func TestCheckFileIdentityRejectsHardLinkAddedAfterCapture(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "settings.xcconfig")
	linkPath := filepath.Join(dir, "settings-link.xcconfig")
	if err := os.WriteFile(path, []byte("settings"), 0o640); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("settings.xcconfig")
	if err != nil {
		t.Fatalf("CaptureFile() error = %v", err)
	}
	if err := os.Link(path, linkPath); err != nil {
		t.Skipf("hard links are unavailable: %v", err)
	}

	if err := root.CheckFileIdentity("settings.xcconfig", identity); !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("CheckFileIdentity() error = %v, want ErrFileIdentityChanged", err)
	}
	if got := mustRead(t, path); got != "settings" {
		t.Fatalf("destination contents = %q, want settings", got)
	}
	if got := mustRead(t, linkPath); got != "settings" {
		t.Fatalf("hard-link contents = %q, want settings", got)
	}
}

func TestCheckFileIdentityRejectsInPlaceChangeDuringVerification(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("before"), 0o640); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("settings.xcconfig")
	if err != nil {
		t.Fatalf("CaptureFile() error = %v", err)
	}
	root.afterIdentityCheckReadForTest = func() {
		root.afterIdentityCheckReadForTest = nil
		if err := os.WriteFile(path, []byte("after!"), 0o640); err != nil {
			t.Fatalf("change file during identity verification: %v", err)
		}
	}
	if err := root.CheckFileIdentity("settings.xcconfig", identity); !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("CheckFileIdentity() error = %v, want ErrFileIdentityChanged", err)
	}
	if got := mustRead(t, path); got != "after!" {
		t.Fatalf("post-race contents = %q, want after!", got)
	}
}

func TestCreateNewFileAtomicWithIdentityReturnsPublicationToken(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })

	identity, err := root.CreateNewFileAtomicWithIdentity("receipt.json", []byte("receipt"), 0o600)
	if err != nil {
		t.Fatalf("CreateNewFileAtomicWithIdentity() error = %v", err)
	}
	requireBoundedRecoveryToken(t, identity)
	diskInfo, err := os.Stat(filepath.Join(dir, "receipt.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(identity.Info(), diskInfo) {
		t.Fatal("publication token does not identify the installed inode")
	}
}

func TestCreateNewFileAtomicWithIdentityUnsupportedOnWindowsLeavesDestinationAbsent(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("strict identity creation is unsupported only on Windows")
	}
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })

	identity, err := root.CreateNewFileAtomicWithIdentity("receipt.json", []byte("receipt"), 0o600)
	if identity != nil {
		t.Fatal("CreateNewFileAtomicWithIdentity() returned an identity on Windows")
	}
	if !errors.Is(err, ErrFileIdentityMutationUnsupported) {
		t.Fatalf("CreateNewFileAtomicWithIdentity() error = %v, want ErrFileIdentityMutationUnsupported", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "receipt.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("destination after unsupported creation = %v, want absent", err)
	}
	if leftovers := temporaryLeftovers(t, dir); len(leftovers) != 0 {
		t.Fatalf("temporary files after unsupported creation = %v, want none", leftovers)
	}
}

func TestStrictIdentityReplacementAndRemovalUnsupportedOnWindowsLeaveDestinationUntouched(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("strict identity replacement and removal are unsupported only on Windows")
	}
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	const original = "original"
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("settings.xcconfig")
	if err != nil {
		t.Fatalf("CaptureFile() error = %v", err)
	}

	installed, err := root.ReplaceFileIfSame("settings.xcconfig", identity, []byte("updated"), 0o600, true)
	if installed != nil {
		t.Fatal("ReplaceFileIfSame() returned an identity on Windows")
	}
	if !errors.Is(err, ErrFileIdentityMutationUnsupported) {
		t.Fatalf("ReplaceFileIfSame() error = %v, want ErrFileIdentityMutationUnsupported", err)
	}
	if got := mustRead(t, path); got != original {
		t.Fatalf("destination after unsupported replacement = %q, want %q", got, original)
	}
	if err := root.RemoveFileIfSameIdentity("settings.xcconfig", identity); !errors.Is(err, ErrFileIdentityMutationUnsupported) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want ErrFileIdentityMutationUnsupported", err)
	}
	if got := mustRead(t, path); got != original {
		t.Fatalf("destination after unsupported removal = %q, want %q", got, original)
	}
	if leftovers := temporaryLeftovers(t, dir); len(leftovers) != 0 {
		t.Fatalf("temporary files after unsupported mutations = %v, want none", leftovers)
	}
}

func TestRemoveFileIfSameIdentityPreservesSameContentReplacement(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	const content = "receipt"
	path := filepath.Join(dir, "receipt.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("receipt.json")
	if err != nil {
		t.Fatalf("CaptureFile() error = %v", err)
	}
	replacementPath := filepath.Join(dir, "replacement.json")
	if err := os.WriteFile(replacementPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	replacementInfo, err := os.Stat(replacementPath)
	if err != nil {
		t.Fatalf("Stat(replacement) error = %v", err)
	}
	if err := os.Rename(filepath.Join(dir, "replacement.json"), path); err != nil {
		t.Fatal(err)
	}

	if err := root.RemoveFileIfSameIdentity("receipt.json", identity); !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want ErrFileIdentityChanged", err)
	}
	if got := mustRead(t, path); got != content {
		t.Fatalf("replacement content = %q, want %q", got, content)
	}
	racerInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(replacement destination) error = %v", err)
	}
	if os.SameFile(identity.Info(), racerInfo) {
		t.Fatal("replacement destination still refers to the original receipt inode")
	}
	if !os.SameFile(replacementInfo, racerInfo) {
		t.Fatal("replacement destination does not retain the renamed replacement inode")
	}
}

func TestRemoveFileIfSameIdentityPreservesReplacementAfterQuarantine(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	const content = "receipt"
	path := filepath.Join(dir, "receipt.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("receipt.json")
	if err != nil {
		t.Fatalf("CaptureFile() error = %v", err)
	}
	replacementPath := filepath.Join(dir, "replacement.json")
	if err := os.WriteFile(replacementPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	replacementInfo, err := os.Stat(replacementPath)
	if err != nil {
		t.Fatalf("Stat(replacement) error = %v", err)
	}
	var syncObserved bool
	root.afterConditionalQuarantineForTest = func(parent *os.Root, _, name string) {
		if err := parent.Rename("replacement.json", name); err != nil {
			t.Fatalf("install replacement after quarantine: %v", err)
		}
	}
	root.syncDirectoryForTest = func(*os.Root) error {
		syncObserved = true
		return nil
	}

	err = root.RemoveFileIfSameIdentity("receipt.json", identity)
	if !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want ErrFileIdentityChanged", err)
	}
	if errors.Is(err, ErrFileIdentityRemoved) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, replacement still occupies destination", err)
	}
	if !syncObserved {
		t.Fatal("quarantine removal was not followed by a parent directory sync")
	}
	if got := mustRead(t, path); got != content {
		t.Fatalf("replacement content = %q, want %q", got, content)
	}
	racerInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(replacement destination) error = %v", err)
	}
	if os.SameFile(identity.Info(), racerInfo) {
		t.Fatal("replacement destination still refers to the original receipt inode")
	}
	if !os.SameFile(replacementInfo, racerInfo) {
		t.Fatal("replacement destination does not retain the renamed replacement inode")
	}
	if matches, globErr := filepath.Glob(filepath.Join(dir, rollbackFilePattern[:len(rollbackFilePattern)-1]+"*")); globErr != nil {
		t.Fatal(globErr)
	} else if len(matches) != 0 {
		t.Fatalf("quarantine files remain after replacement preservation: %v", matches)
	}
}

func TestRemoveFileIfSameIdentityPreservesReplacementDuringQuarantineRemoval(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	const content = "receipt"
	path := filepath.Join(dir, "receipt.json")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("receipt.json")
	if err != nil {
		t.Fatalf("CaptureFile() error = %v", err)
	}
	replacementPath := filepath.Join(dir, "replacement.json")
	if err := os.WriteFile(replacementPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	replacementInfo, err := os.Stat(replacementPath)
	if err != nil {
		t.Fatalf("Stat(replacement) error = %v", err)
	}
	var hookCalls int
	root.beforeConditionalQuarantineRemovalForTest = func(parent *os.Root, _ string) {
		hookCalls++
		if err := parent.Rename("replacement.json", "receipt.json"); err != nil {
			t.Fatalf("install replacement during quarantine removal: %v", err)
		}
	}

	err = root.RemoveFileIfSameIdentity("receipt.json", identity)
	if !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want ErrFileIdentityChanged", err)
	}
	if errors.Is(err, ErrFileIdentityRemoved) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, replacement still occupies destination", err)
	}
	if hookCalls != 1 {
		t.Fatalf("quarantine-removal hook calls = %d, want one", hookCalls)
	}
	racerInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(replacement destination) error = %v", err)
	}
	if os.SameFile(identity.Info(), racerInfo) || os.SameFile(replacementInfo, identity.Info()) {
		t.Fatal("replacement reused the original receipt inode")
	}
	if !os.SameFile(replacementInfo, racerInfo) {
		t.Fatal("replacement destination does not retain the renamed replacement inode")
	}
	if got := mustRead(t, path); got != content {
		t.Fatalf("replacement content = %q, want %q", got, content)
	}
}

func TestFileIdentityCannotCrossRoots(t *testing.T) {
	requireStrictIdentityPlatform(t)
	firstDir := t.TempDir()
	secondDir := t.TempDir()
	first := mustRoot(t, firstDir)
	second := mustRoot(t, secondDir)
	t.Cleanup(func() { _ = first.Close(); _ = second.Close() })
	if err := os.WriteFile(filepath.Join(firstDir, "value"), []byte("value"), 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err := first.CaptureFile("value")
	if err != nil {
		t.Fatalf("CaptureFile() error = %v", err)
	}
	if _, err := second.ReplaceFileIfSame("value", identity, []byte("new"), 0o600, false); !errors.Is(err, ErrFileIdentityMismatch) {
		t.Fatalf("ReplaceFileIfSame() error = %v, want ErrFileIdentityMismatch", err)
	}
}

func TestCaptureFileIdentityRejectsOversizeSnapshot(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	if err := os.WriteFile(filepath.Join(dir, "large"), []byte("12345"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := root.CaptureFileLimited("large", 4); !errors.Is(err, ErrFileIdentityDataTooLarge) {
		t.Fatalf("CaptureFileLimited() error = %v, want ErrFileIdentityDataTooLarge", err)
	}
	identity, err := root.CaptureFile("large")
	if err != nil {
		t.Fatal(err)
	}
	oversize := make([]byte, int(fileIdentityDataLimit)+1)
	if _, err := root.ReplaceFileIfSame("large", identity, oversize, 0o600, false); !errors.Is(err, ErrFileIdentityDataTooLarge) {
		t.Fatalf("ReplaceFileIfSame() error = %v, want ErrFileIdentityDataTooLarge", err)
	}
	if got := mustRead(t, filepath.Join(dir, "large")); got != "12345" {
		t.Fatalf("destination after oversize replacement = %q, want unchanged snapshot", got)
	}
}

func TestCaptureFileLimitedAllowsExplicitBoundAboveDefault(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	data := bytes.Repeat([]byte("x"), int(fileIdentityDataLimit)+1)
	if err := os.WriteFile(filepath.Join(dir, "large"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := root.CaptureFile("large"); !errors.Is(err, ErrFileIdentityDataTooLarge) {
		t.Fatalf("CaptureFile() error = %v, want default size-limit refusal", err)
	}
	identity, err := root.CaptureFileLimited("large", int64(len(data)))
	if err != nil {
		t.Fatalf("CaptureFileLimited() error: %v", err)
	}
	if got := len(identity.Data()); got != len(data) {
		t.Fatalf("captured %d bytes, want %d", got, len(data))
	}
}

func TestCaptureFileLimitedRejectsBoundAboveExplicitCaptureLimit(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })

	if _, err := root.CaptureFileLimited("large", fileIdentityCaptureLimit+1); !errors.Is(err, ErrFileIdentityDataTooLarge) {
		t.Fatalf("CaptureFileLimited() error = %v, want ErrFileIdentityDataTooLarge", err)
	}
}

func TestReplaceFileIfSameDoesNotFallBackWhenNativeNoReplaceIsUnavailable(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	const original = "original"
	if err := os.WriteFile(filepath.Join(dir, "value"), []byte(original), 0o640); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("value")
	if err != nil {
		t.Fatal(err)
	}
	var renameCalls int
	root.renameNoReplaceForTest = func(parent *os.Root, oldName, newName string) error {
		renameCalls++
		if renameCalls == 1 {
			return secureopen.RenameNoReplaceInRoot(parent, oldName, newName)
		}
		if renameCalls == 2 {
			return secureopen.ErrRenameNoReplaceUnsupported
		}
		return secureopen.RenameNoReplaceInRoot(parent, oldName, newName)
	}

	if _, err := root.ReplaceFileIfSame("value", identity, []byte("updated"), 0o640, true); !errors.Is(err, secureopen.ErrRenameNoReplaceUnsupported) {
		t.Fatalf("ReplaceFileIfSame() error = %v, want ErrRenameNoReplaceUnsupported", err)
	}
	if renameCalls != 2 {
		t.Fatalf("rename hook calls = %d, want quarantine and publication attempts", renameCalls)
	}
	if got := mustRead(t, filepath.Join(dir, "value")); got != original {
		t.Fatalf("destination after unsupported publication = %q, want %q", got, original)
	}
}

func TestReplaceFileIfSameSyncsParentAfterReplacementAppearsAfterQuarantine(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("original"), 0o640); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("settings.xcconfig")
	if err != nil {
		t.Fatalf("CaptureFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "racer-source"), []byte("replacement"), 0o600); err != nil {
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

	installed, err := root.ReplaceFileIfSame("settings.xcconfig", identity, []byte("updated"), 0o640, true)
	if installed != nil {
		t.Fatal("ReplaceFileIfSame() returned an identity before publication")
	}
	if !errors.Is(err, ErrFileIdentityChanged) || !errors.Is(err, syncErr) || !syncObserved {
		t.Fatalf("ReplaceFileIfSame() error = %v, want replacement and parent sync failures", err)
	}
	if got := mustRead(t, path); got != "replacement" {
		t.Fatalf("replacement content = %q, want preserved replacement", got)
	}
}

func TestReplaceFileIfSameLeavesChangedStagingEntryOnCleanupUncertainty(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("original"), 0o640); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("settings.xcconfig")
	if err != nil {
		t.Fatalf("CaptureFile() error = %v", err)
	}
	publicationErr := errors.New("injected publication failure")
	var renameCalls int
	var stagingName string
	root.renameNoReplaceForTest = func(parent *os.Root, oldName, newName string) error {
		renameCalls++
		if renameCalls == 1 {
			return secureopen.RenameNoReplaceInRoot(parent, oldName, newName)
		}
		if renameCalls != 2 {
			return secureopen.RenameNoReplaceInRoot(parent, oldName, newName)
		}
		stagingName = oldName
		if err := parent.Rename(oldName, "staging-original"); err != nil {
			t.Fatalf("move original staging entry: %v", err)
		}
		replacement, err := secureopen.OpenNewFileNoFollowInRoot(parent, oldName, 0o600)
		if err != nil {
			t.Fatalf("create staging replacement: %v", err)
		}
		if _, err := replacement.Write([]byte("racer")); err != nil {
			_ = replacement.Close()
			t.Fatalf("write staging replacement: %v", err)
		}
		if err := replacement.Close(); err != nil {
			t.Fatalf("close staging replacement: %v", err)
		}
		return publicationErr
	}

	installed, err := root.ReplaceFileIfSame("settings.xcconfig", identity, []byte("updated"), 0o640, true)
	if installed != nil {
		t.Fatal("ReplaceFileIfSame() returned an identity before publication")
	}
	if !errors.Is(err, publicationErr) {
		t.Fatalf("ReplaceFileIfSame() error = %v, want publication failure", err)
	}
	if !errors.Is(err, ErrStagingCleanupUncertain) {
		t.Fatalf("ReplaceFileIfSame() error = %v, want staging cleanup uncertainty", err)
	}
	if renameCalls != 2 {
		t.Fatalf("rename hook calls = %d, want quarantine and publication attempts", renameCalls)
	}
	if stagingName == "" {
		t.Fatal("staging replacement hook did not capture the staging name")
	}
	if got := mustRead(t, path); got != "original" {
		t.Fatalf("destination after failed publication = %q, want original", got)
	}
	if got := mustRead(t, filepath.Join(dir, "staging-original")); got != "updated" {
		t.Fatalf("moved staging evidence = %q, want updated", got)
	}
	if got := mustRead(t, filepath.Join(dir, stagingName)); got != "racer" {
		t.Fatalf("replacement staging entry = %q, want racer", got)
	}
}

func TestReplaceFileIfSameRejectsInPlaceChangeDuringPublication(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("original"), 0o640); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("settings.xcconfig")
	if err != nil {
		t.Fatalf("CaptureFile() error = %v", err)
	}
	root.afterConditionalPublicationOpenForTest = func(_ *os.Root, _ string, _ *os.File) {
		root.afterConditionalPublicationOpenForTest = nil
		if err := os.WriteFile(path, []byte("racer!!"), 0o640); err != nil {
			t.Fatalf("change published file in place: %v", err)
		}
	}

	installed, err := root.ReplaceFileIfSame("settings.xcconfig", identity, []byte("updated"), 0o640, true)
	requireBoundedRecoveryToken(t, installed)
	if !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("ReplaceFileIfSame() error = %v, want ErrFileIdentityChanged", err)
	}
	if got := mustRead(t, path); got != "racer!!" {
		t.Fatalf("racing contents = %q, want racer!!", got)
	}
	if removeErr := root.RemoveFileIfSameIdentity("settings.xcconfig", installed); !errors.Is(removeErr, ErrFileIdentityChanged) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want changed destination preserved", removeErr)
	}
	if got := mustRead(t, path); got != "racer!!" {
		t.Fatalf("destination after rejected recovery = %q, want racer!!", got)
	}
}

func TestReplaceFileIfSameRejectsInPlaceDataChangeAfterContentVerification(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("original"), 0o640); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("settings.xcconfig")
	if err != nil {
		t.Fatalf("CaptureFile() error = %v", err)
	}
	root.afterConditionalContentVerificationForTest = func(_ *os.Root, _ string) {
		root.afterConditionalContentVerificationForTest = nil
		if err := os.WriteFile(path, []byte("racing!!"), 0o640); err != nil {
			t.Fatalf("change published file in place: %v", err)
		}
	}

	installed, err := root.ReplaceFileIfSame("settings.xcconfig", identity, []byte("updated!"), 0o640, true)
	requireBoundedRecoveryToken(t, installed)
	if !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("ReplaceFileIfSame() error = %v, want ErrFileIdentityChanged", err)
	}
	if got := mustRead(t, path); got != "racing!!" {
		t.Fatalf("racing contents = %q, want racing!!", got)
	}
	if removeErr := root.RemoveFileIfSameIdentity("settings.xcconfig", installed); !errors.Is(removeErr, ErrFileIdentityChanged) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want changed destination preserved", removeErr)
	}
	if got := mustRead(t, path); got != "racing!!" {
		t.Fatalf("destination after rejected recovery = %q, want racing!!", got)
	}
}

func TestReplaceFileIfSameRejectsInPlaceMetadataChangeAfterContentVerification(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("original"), 0o640); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("settings.xcconfig")
	if err != nil {
		t.Fatalf("CaptureFile() error = %v", err)
	}
	root.afterConditionalContentVerificationForTest = func(_ *os.Root, _ string) {
		root.afterConditionalContentVerificationForTest = nil
		if err := os.Chmod(path, 0o600); err != nil {
			t.Fatalf("change published file metadata in place: %v", err)
		}
	}

	installed, err := root.ReplaceFileIfSame("settings.xcconfig", identity, []byte("updated!"), 0o640, true)
	requireBoundedRecoveryToken(t, installed)
	if !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("ReplaceFileIfSame() error = %v, want ErrFileIdentityChanged", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("racing mode = %o, want 600", got)
	}
	if removeErr := root.RemoveFileIfSameIdentity("settings.xcconfig", installed); !errors.Is(removeErr, ErrFileIdentityChanged) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want changed destination preserved", removeErr)
	}
	info, err = os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("destination after rejected recovery mode = %o, want 600", got)
	}
}

func TestReplaceFileIfSameRejectsReplacementAtFinalRootedCheck(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("original"), 0o640); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("settings.xcconfig")
	if err != nil {
		t.Fatalf("CaptureFile() error = %v", err)
	}
	replacementPath := filepath.Join(dir, "replacement.xcconfig")
	if err := os.WriteFile(replacementPath, []byte("racer"), 0o600); err != nil {
		t.Fatal(err)
	}
	replacementInfo, err := os.Stat(replacementPath)
	if err != nil {
		t.Fatalf("Stat(replacement) error = %v", err)
	}
	var hookCalls int
	root.beforeConditionalFinalRootedCheckForTest = func(parent *os.Root, name string) {
		hookCalls++
		if err := parent.Rename("replacement.xcconfig", name); err != nil {
			t.Fatalf("install replacement before final rooted check: %v", err)
		}
	}

	installed, err := root.ReplaceFileIfSame("settings.xcconfig", identity, []byte("updated"), 0o640, true)
	requireBoundedRecoveryToken(t, installed)
	if !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("ReplaceFileIfSame() error = %v, want ErrFileIdentityChanged", err)
	}
	if hookCalls != 1 {
		t.Fatalf("final-rooted-check hook calls = %d, want one", hookCalls)
	}
	racerInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(replacement destination) error = %v", err)
	}
	if os.SameFile(installed.Info(), racerInfo) || os.SameFile(replacementInfo, installed.Info()) {
		t.Fatal("replacement reused the published receipt inode")
	}
	if !os.SameFile(replacementInfo, racerInfo) {
		t.Fatal("replacement destination does not retain the renamed replacement inode")
	}
	if got := mustRead(t, path); got != "racer" {
		t.Fatalf("replacement content = %q, want racer", got)
	}
	removeErr := root.RemoveFileIfSameIdentity("settings.xcconfig", installed)
	if !errors.Is(removeErr, ErrFileIdentityChanged) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want ErrFileIdentityChanged", removeErr)
	}
	if errors.Is(removeErr, ErrFileIdentityRemoved) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, replacement still occupies destination", removeErr)
	}
	if got := mustRead(t, path); got != "racer" {
		t.Fatalf("destination after rejected recovery = %q, want racer", got)
	}
}

func TestReplaceFileIfSameRejectsMetadataChangeBeforePublishedIdentityBaseline(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("original"), 0o640); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("settings.xcconfig")
	if err != nil {
		t.Fatalf("CaptureFile() error = %v", err)
	}
	root.afterConditionalPublicationForTest = func(_ *os.Root, _ string) {
		root.afterConditionalPublicationForTest = nil
		if err := os.Chmod(path, 0o600); err != nil {
			t.Fatalf("change published file metadata before identity baseline: %v", err)
		}
	}

	installed, err := root.ReplaceFileIfSame("settings.xcconfig", identity, []byte("updated"), 0o640, true)
	requireBoundedRecoveryToken(t, installed)
	if !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("ReplaceFileIfSame() error = %v, want ErrFileIdentityChanged", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("racing mode = %o, want 600", got)
	}
	if removeErr := root.RemoveFileIfSameIdentity("settings.xcconfig", installed); !errors.Is(removeErr, ErrFileIdentityChanged) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want changed destination preserved", removeErr)
	}
}

func TestReplaceFileIfSameRejectsModTimeChangeBeforeQuarantine(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("original"), 0o640); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("settings.xcconfig")
	if err != nil {
		t.Fatalf("CaptureFile() error = %v", err)
	}
	changed := identity.Info().ModTime().Add(2 * time.Second)
	if err := os.Chtimes(path, changed, changed); err != nil {
		t.Fatalf("Chtimes() error = %v", err)
	}

	installed, err := root.ReplaceFileIfSame("settings.xcconfig", identity, []byte("updated"), 0o640, true)
	if installed != nil {
		t.Fatal("ReplaceFileIfSame() returned an identity after mtime drift")
	}
	if !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("ReplaceFileIfSame() error = %v, want ErrFileIdentityChanged", err)
	}
	if got := mustRead(t, path); got != "original" {
		t.Fatalf("destination content = %q, want original", got)
	}
}

func TestReplaceFileIfSameRetainsTokenWhenDestinationDisappearsAfterPublication(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("original"), 0o640); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("settings.xcconfig")
	if err != nil {
		t.Fatalf("CaptureFile() error = %v", err)
	}
	root.afterConditionalPublicationForTest = func(parent *os.Root, name string) {
		root.afterConditionalPublicationForTest = nil
		if err := parent.Remove(name); err != nil {
			t.Fatalf("remove published destination: %v", err)
		}
	}

	installed, err := root.ReplaceFileIfSame("settings.xcconfig", identity, []byte("updated"), 0o640, true)
	requireBoundedRecoveryToken(t, installed)
	if !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("ReplaceFileIfSame() error = %v, want ErrFileIdentityChanged", err)
	}
	if removeErr := root.RemoveFileIfSameIdentity("settings.xcconfig", installed); !errors.Is(removeErr, ErrFileIdentityChanged) || !errors.Is(removeErr, ErrFileIdentityRemoved) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want known-absent identity result", removeErr)
	}
	if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("destination after rejected recovery = %v, want absent", statErr)
	}
}

func TestReplaceFileIfSamePreservesHardLinkAndMetadata(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "settings.xcconfig")
	alias := filepath.Join(dir, "settings-alias.xcconfig")
	if err := os.WriteFile(path, []byte("original"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(path, alias); err != nil {
		t.Skipf("hard links are unavailable: %v", err)
	}
	identity, err := root.CaptureFile("settings.xcconfig")
	if err != nil {
		t.Fatalf("CaptureFile() error = %v", err)
	}

	installed, err := root.ReplaceFileIfSame("settings.xcconfig", identity, []byte("updated"), 0o600, true)
	if err != nil {
		t.Fatalf("ReplaceFileIfSame() error = %v", err)
	}
	if installed == nil {
		t.Fatal("ReplaceFileIfSame() returned nil installed identity")
	}
	if got := mustRead(t, path); got != "updated" {
		t.Fatalf("destination content = %q, want updated", got)
	}
	if got := mustRead(t, alias); got != "original" {
		t.Fatalf("hard-link content = %q, want original", got)
	}
	destinationInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(destination) error = %v", err)
	}
	if destinationInfo.Mode().Perm() != 0o640 {
		t.Fatalf("destination mode = %o, want preserved 640", destinationInfo.Mode().Perm())
	}
	aliasInfo, err := os.Stat(alias)
	if err != nil {
		t.Fatalf("Stat(alias) error = %v", err)
	}
	if os.SameFile(destinationInfo, aliasInfo) {
		t.Fatal("replacement reused the hard-linked inode")
	}
	if !os.SameFile(identity.Info(), aliasInfo) {
		t.Fatal("hard-link alias no longer identifies the original inode")
	}
	if !os.SameFile(installed.Info(), destinationInfo) {
		t.Fatal("installed identity does not identify the replacement inode")
	}
}

func TestReplaceFileIfSamePreservesSpecialBits(t *testing.T) {
	requireStrictIdentityPlatform(t)
	if runtime.GOOS == "darwin" {
		t.Skip("macOS clears setuid/setgid during rooted replacement; Linux CI covers special-bit preservation")
	}
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "special.txt")
	want := os.FileMode(0o755) | os.ModeSetuid | os.ModeSetgid | os.ModeSticky
	if err := os.WriteFile(path, []byte("original"), want); err != nil {
		t.Fatal(err)
	}
	initial, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	mask := os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky
	if initial.Mode()&mask != want {
		t.Skipf("filesystem does not preserve special mode bits: got %v", initial.Mode()&mask)
	}
	identity, err := root.CaptureFile("special.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := root.ReplaceFileIfSame("special.txt", identity, []byte("replacement"), 0o600, true); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode() & mask; got != want {
		t.Fatalf("mode = %v, want %v", got, want)
	}
}

func TestReplaceFileIfSameRejectsHardLinkAddedAfterCapture(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "settings.xcconfig")
	alias := filepath.Join(dir, "settings-alias.xcconfig")
	if err := os.WriteFile(path, []byte("original"), 0o640); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("settings.xcconfig")
	if err != nil {
		t.Fatalf("CaptureFile() error = %v", err)
	}
	root.beforeConditionalQuarantineForTest = func(_ *os.Root, _ string) {
		root.beforeConditionalQuarantineForTest = nil
		if err := os.Link(path, alias); err != nil {
			t.Fatalf("create late hard link: %v", err)
		}
	}

	installed, err := root.ReplaceFileIfSame("settings.xcconfig", identity, []byte("updated"), 0o640, true)
	if installed != nil {
		t.Fatal("ReplaceFileIfSame() returned an identity after the hard-link state changed")
	}
	if !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("ReplaceFileIfSame() error = %v, want ErrFileIdentityChanged", err)
	}
	if got := mustRead(t, path); got != "original" {
		t.Fatalf("destination content = %q, want original", got)
	}
	if got := mustRead(t, alias); got != "original" {
		t.Fatalf("hard-link content = %q, want original", got)
	}
}

func TestReplaceFileIfSameReturnsInstalledIdentityAfterDirectorySyncFailure(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("original"), 0o640); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("settings.xcconfig")
	if err != nil {
		t.Fatalf("CaptureFile() error = %v", err)
	}
	syncErr := errors.New("injected parent directory sync failure")
	var syncObserved bool
	root.syncDirectoryForTest = func(_ *os.Root) error {
		syncObserved = true
		return syncErr
	}

	installed, err := root.ReplaceFileIfSame("settings.xcconfig", identity, []byte("updated"), 0o600, true)
	if !errors.Is(err, syncErr) {
		t.Fatalf("ReplaceFileIfSame() error = %v, want parent sync failure", err)
	}
	if !syncObserved {
		t.Fatal("parent directory sync hook was not invoked")
	}
	requireBoundedRecoveryToken(t, installed)
	if got := mustRead(t, path); got != "updated" {
		t.Fatalf("destination content = %q, want updated", got)
	}
	diskInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(destination) error = %v", err)
	}
	if !os.SameFile(installed.Info(), diskInfo) {
		t.Fatal("installed identity does not identify the published destination")
	}
	if err := root.CheckFileIdentity("settings.xcconfig", installed); err != nil {
		t.Fatalf("CheckFileIdentity(installed) error = %v", err)
	}
}

func TestRootCloseWaitsForIdentityOperation(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "receipt.json"), []byte("receipt"), 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("receipt.json")
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	root.beforeConditionalQuarantineForTest = func(*os.Root, string) {
		close(entered)
		<-release
	}
	operationDone := make(chan error, 1)
	go func() {
		operationDone <- root.RemoveFileIfSameIdentity("receipt.json", identity)
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("identity operation did not reach the in-flight barrier")
	}
	closeDone := make(chan error, 1)
	go func() { closeDone <- root.Close() }()
	select {
	case err := <-closeDone:
		t.Fatalf("Root.Close() returned before identity operation completed: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	close(release)
	if err := <-operationDone; err != nil {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v", err)
	}
	if err := <-closeDone; err != nil {
		t.Fatalf("Root.Close() error = %v", err)
	}
}

func TestRemoveFileIfSameIdentityReturnsRemovedSentinelAfterDirectorySyncFailure(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "receipt.json")
	if err := os.WriteFile(path, []byte("receipt"), 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("receipt.json")
	if err != nil {
		t.Fatalf("CaptureFile() error = %v", err)
	}
	syncErr := errors.New("injected receipt removal sync failure")
	var syncObserved bool
	root.syncDirectoryForTest = func(_ *os.Root) error {
		syncObserved = true
		return syncErr
	}

	err = root.RemoveFileIfSameIdentity("receipt.json", identity)
	if !errors.Is(err, ErrFileIdentityRemoved) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want ErrFileIdentityRemoved", err)
	}
	if !errors.Is(err, syncErr) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want parent sync failure", err)
	}
	if !syncObserved {
		t.Fatal("parent directory sync hook was not invoked")
	}
	if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("removed receipt stat error = %v, want absent", statErr)
	}
}

func TestWriteFileIfSameWithInfoRootCloseWaitsForIdentityRetention(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "settings.xcconfig"), []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	expected, err := os.Stat(filepath.Join(dir, "settings.xcconfig"))
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	root.afterConditionalPublicationForTest = func(*os.Root, string) {
		close(entered)
		<-release
	}
	operationDone := make(chan error, 1)
	go func() {
		_, operationErr := root.WriteFileIfSameWithInfo("settings.xcconfig", []byte("updated"), 0o640, expected, []byte("old"), true)
		operationDone <- operationErr
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("compatibility write did not reach the identity-retention barrier")
	}
	closeDone := make(chan error, 1)
	go func() { closeDone <- root.Close() }()
	select {
	case err := <-closeDone:
		t.Fatalf("Root.Close() returned before compatibility identity operation completed: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	close(release)
	if err := <-operationDone; err != nil {
		t.Fatalf("WriteFileIfSameWithInfo() error = %v", err)
	}
	if err := <-closeDone; err != nil {
		t.Fatalf("Root.Close() error = %v", err)
	}
}

func TestCreateNewFileAtomicWithInfoRootCloseWaitsForIdentityRetention(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	entered := make(chan struct{})
	release := make(chan struct{})
	root.beforePublicationOpenForTest = func(*os.Root, string) {
		close(entered)
		<-release
	}
	operationDone := make(chan error, 1)
	go func() {
		_, operationErr := root.CreateNewFileAtomicWithInfo("receipt.json", []byte("receipt"), 0o600)
		operationDone <- operationErr
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("compatibility create did not reach the identity-retention barrier")
	}
	closeDone := make(chan error, 1)
	go func() { closeDone <- root.Close() }()
	select {
	case err := <-closeDone:
		t.Fatalf("Root.Close() returned before compatibility identity operation completed: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	close(release)
	if err := <-operationDone; err != nil {
		t.Fatalf("CreateNewFileAtomicWithInfo() error = %v", err)
	}
	if err := <-closeDone; err != nil {
		t.Fatalf("Root.Close() error = %v", err)
	}
}

func TestCreateNewFileAtomicWithIdentityRetainsTokenAfterPublicationObservationFailure(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	injected := errors.New("injected publication observation failure")
	root.postPublicationLstatForTest = func(*os.Root, string) (os.FileInfo, error) {
		return nil, injected
	}

	identity, err := root.CreateNewFileAtomicWithIdentity("receipt.json", []byte("receipt"), 0o600)
	requireBoundedRecoveryToken(t, identity)
	if !errors.Is(err, injected) {
		t.Fatalf("CreateNewFileAtomicWithIdentity() error = %v, want injected observation failure", err)
	}
	if got := mustRead(t, filepath.Join(dir, "receipt.json")); got != "receipt" {
		t.Fatalf("published contents = %q, want receipt", got)
	}
	if err := root.RemoveFileIfSameIdentity("receipt.json", identity); err != nil {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want retained publication token to clean up", err)
	}
	if _, statErr := os.Lstat(filepath.Join(dir, "receipt.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("destination after token cleanup = %v, want absent", statErr)
	}
}

func TestCreateNewFileAtomicWithIdentityRejectsInPlaceChangeDuringPublication(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "receipt.json")
	root.afterPublicationOpenForTest = func(_ *os.Root, _ string) {
		root.afterPublicationOpenForTest = nil
		if err := os.WriteFile(path, []byte("racer!!"), 0o600); err != nil {
			t.Fatalf("change published file in place: %v", err)
		}
	}

	identity, err := root.CreateNewFileAtomicWithIdentity("receipt.json", []byte("receipt"), 0o600)
	requireBoundedRecoveryToken(t, identity)
	if !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("CreateNewFileAtomicWithIdentity() error = %v, want ErrFileIdentityChanged", err)
	}
	if got := mustRead(t, path); got != "racer!!" {
		t.Fatalf("racing contents = %q, want racer!!", got)
	}
	if removeErr := root.RemoveFileIfSameIdentity("receipt.json", identity); !errors.Is(removeErr, ErrFileIdentityChanged) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want changed destination preserved", removeErr)
	}
	if got := mustRead(t, path); got != "racer!!" {
		t.Fatalf("destination after rejected recovery = %q, want racer!!", got)
	}
}

func TestCreateNewFileAtomicWithIdentityRejectsInPlaceDataChangeAfterContentVerification(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "receipt.json")
	root.afterPublicationContentVerificationForTest = func(_ *os.Root, _ string) {
		root.afterPublicationContentVerificationForTest = nil
		if err := os.WriteFile(path, []byte("racing!"), 0o600); err != nil {
			t.Fatalf("change published file in place: %v", err)
		}
	}

	identity, err := root.CreateNewFileAtomicWithIdentity("receipt.json", []byte("receipt"), 0o600)
	requireBoundedRecoveryToken(t, identity)
	if !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("CreateNewFileAtomicWithIdentity() error = %v, want ErrFileIdentityChanged", err)
	}
	if got := mustRead(t, path); got != "racing!" {
		t.Fatalf("racing contents = %q, want racing!", got)
	}
	if removeErr := root.RemoveFileIfSameIdentity("receipt.json", identity); !errors.Is(removeErr, ErrFileIdentityChanged) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want changed destination preserved", removeErr)
	}
	if got := mustRead(t, path); got != "racing!" {
		t.Fatalf("destination after rejected recovery = %q, want racing!", got)
	}
}

func TestCreateNewFileAtomicWithIdentityRejectsInPlaceMetadataChangeAfterContentVerification(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "receipt.json")
	root.afterPublicationContentVerificationForTest = func(_ *os.Root, _ string) {
		root.afterPublicationContentVerificationForTest = nil
		if err := os.Chmod(path, 0o640); err != nil {
			t.Fatalf("change published file metadata in place: %v", err)
		}
	}

	identity, err := root.CreateNewFileAtomicWithIdentity("receipt.json", []byte("receipt"), 0o600)
	requireBoundedRecoveryToken(t, identity)
	if !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("CreateNewFileAtomicWithIdentity() error = %v, want ErrFileIdentityChanged", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Fatalf("racing mode = %o, want 640", got)
	}
	if removeErr := root.RemoveFileIfSameIdentity("receipt.json", identity); !errors.Is(removeErr, ErrFileIdentityChanged) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want changed destination preserved", removeErr)
	}
	info, err = os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Fatalf("destination after rejected recovery mode = %o, want 640", got)
	}
}

func TestCreateNewFileAtomicWithIdentityRejectsReplacementAtFinalRootedCheck(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "receipt.json")
	replacementPath := filepath.Join(dir, "replacement.json")
	if err := os.WriteFile(replacementPath, []byte("racer"), 0o600); err != nil {
		t.Fatal(err)
	}
	replacementInfo, err := os.Stat(replacementPath)
	if err != nil {
		t.Fatalf("Stat(replacement) error = %v", err)
	}
	var hookCalls int
	root.beforePublicationFinalRootedCheckForTest = func(parent *os.Root, name string) {
		hookCalls++
		if err := parent.Rename("replacement.json", name); err != nil {
			t.Fatalf("install replacement before final rooted check: %v", err)
		}
	}

	identity, err := root.CreateNewFileAtomicWithIdentity("receipt.json", []byte("receipt"), 0o600)
	requireBoundedRecoveryToken(t, identity)
	if !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("CreateNewFileAtomicWithIdentity() error = %v, want ErrFileIdentityChanged", err)
	}
	if hookCalls != 1 {
		t.Fatalf("final-rooted-check hook calls = %d, want one", hookCalls)
	}
	racerInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(replacement destination) error = %v", err)
	}
	if os.SameFile(identity.Info(), racerInfo) || os.SameFile(replacementInfo, identity.Info()) {
		t.Fatal("replacement reused the published receipt inode")
	}
	if !os.SameFile(replacementInfo, racerInfo) {
		t.Fatal("replacement destination does not retain the renamed replacement inode")
	}
	if got := mustRead(t, path); got != "racer" {
		t.Fatalf("replacement content = %q, want racer", got)
	}
	removeErr := root.RemoveFileIfSameIdentity("receipt.json", identity)
	if !errors.Is(removeErr, ErrFileIdentityChanged) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want ErrFileIdentityChanged", removeErr)
	}
	if errors.Is(removeErr, ErrFileIdentityRemoved) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, replacement still occupies destination", removeErr)
	}
	if got := mustRead(t, path); got != "racer" {
		t.Fatalf("destination after rejected recovery = %q, want racer", got)
	}
}

func TestCreateNewFileAtomicWithIdentityRejectsMetadataChangeBeforePublishedIdentityBaseline(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "receipt.json")
	root.beforePublicationOpenForTest = func(_ *os.Root, _ string) {
		if err := os.Chmod(path, 0o640); err != nil {
			t.Fatalf("change published file metadata before identity baseline: %v", err)
		}
	}

	identity, err := root.CreateNewFileAtomicWithIdentity("receipt.json", []byte("receipt"), 0o600)
	requireBoundedRecoveryToken(t, identity)
	if !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("CreateNewFileAtomicWithIdentity() error = %v, want ErrFileIdentityChanged", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Fatalf("racing mode = %o, want 640", got)
	}
	if removeErr := root.RemoveFileIfSameIdentity("receipt.json", identity); !errors.Is(removeErr, ErrFileIdentityChanged) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want changed destination preserved", removeErr)
	}
}

func TestCreateNewFileAtomicWithIdentityRejectsReplacementDuringIdentityCheck(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	const content = "receipt"
	if err := os.WriteFile(filepath.Join(dir, "replacement"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	root.afterPublicationOpenForTest = func(parent *os.Root, name string) {
		if err := parent.Rename(name, "published-original"); err != nil {
			t.Fatalf("move original publication: %v", err)
		}
		if err := parent.Rename("replacement", name); err != nil {
			t.Fatalf("install same-content replacement: %v", err)
		}
	}

	identity, err := root.CreateNewFileAtomicWithIdentity("receipt.json", []byte(content), 0o600)
	requireBoundedRecoveryToken(t, identity)
	if !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("CreateNewFileAtomicWithIdentity() error = %v, want ErrFileIdentityChanged", err)
	}
	if got := mustRead(t, filepath.Join(dir, "receipt.json")); got != content {
		t.Fatalf("replacement contents = %q, want %q", got, content)
	}
	if got := mustRead(t, filepath.Join(dir, "published-original")); got != content {
		t.Fatalf("original publication contents = %q, want %q", got, content)
	}
	if removeErr := root.RemoveFileIfSameIdentity("receipt.json", identity); !errors.Is(removeErr, ErrFileIdentityChanged) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want replacement preserved", removeErr)
	}
	if got := mustRead(t, filepath.Join(dir, "receipt.json")); got != content {
		t.Fatalf("replacement after rejected recovery = %q, want %q", got, content)
	}
}

func TestCreateNewFileAtomicWithIdentityRetainsTokenWhenDestinationDisappearsAfterPublication(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "receipt.json")
	root.afterPublicationOpenForTest = func(parent *os.Root, name string) {
		if err := parent.Remove(name); err != nil {
			t.Fatalf("remove published destination: %v", err)
		}
	}

	identity, err := root.CreateNewFileAtomicWithIdentity("receipt.json", []byte("receipt"), 0o600)
	requireBoundedRecoveryToken(t, identity)
	if !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("CreateNewFileAtomicWithIdentity() error = %v, want ErrFileIdentityChanged", err)
	}
	if removeErr := root.RemoveFileIfSameIdentity("receipt.json", identity); !errors.Is(removeErr, ErrFileIdentityChanged) || !errors.Is(removeErr, ErrFileIdentityRemoved) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want known-absent identity result", removeErr)
	}
	if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("destination after rejected recovery = %v, want absent", statErr)
	}
}

func TestRemoveFileIfSameIdentityRestoresReplacementMovedBeforeQuarantineObservation(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "receipt.json")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "replacement"), []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("receipt.json")
	if err != nil {
		t.Fatal(err)
	}
	root.beforeConditionalQuarantineForTest = func(parent *os.Root, name string) {
		if err := parent.Rename(name, "original-moved"); err != nil {
			t.Fatalf("move original before quarantine: %v", err)
		}
		if err := parent.Rename("replacement", name); err != nil {
			t.Fatalf("install replacement before quarantine: %v", err)
		}
	}

	err = root.RemoveFileIfSameIdentity("receipt.json", identity)
	if !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want identity change", err)
	}
	if got := mustRead(t, path); got != "replacement" {
		t.Fatalf("replacement contents = %q, want replacement", got)
	}
}

func TestReplaceFileIfSameIdentityRestoresReplacementMovedBeforeQuarantineObservation(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("original"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "replacement"), []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("settings.xcconfig")
	if err != nil {
		t.Fatal(err)
	}
	root.beforeConditionalQuarantineForTest = func(parent *os.Root, name string) {
		if err := parent.Rename(name, "original-moved"); err != nil {
			t.Fatalf("move original before quarantine: %v", err)
		}
		if err := parent.Rename("replacement", name); err != nil {
			t.Fatalf("install replacement before quarantine: %v", err)
		}
	}

	installed, err := root.ReplaceFileIfSame("settings.xcconfig", identity, []byte("updated"), 0o640, true)
	if installed != nil {
		t.Fatal("ReplaceFileIfSame() returned an identity after pre-publication mismatch")
	}
	if !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("ReplaceFileIfSame() error = %v, want identity change", err)
	}
	if got := mustRead(t, path); got != "replacement" {
		t.Fatalf("replacement contents = %q, want replacement", got)
	}
}

func TestRemoveFileIfSameIdentityLeavesChangedQuarantine(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "receipt.json")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "replacement"), []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("receipt.json")
	if err != nil {
		t.Fatal(err)
	}
	var changedQuarantine string
	root.beforeConditionalQuarantineRemovalForTest = func(parent *os.Root, quarantineName string) {
		changedQuarantine = quarantineName
		if err := parent.Rename(quarantineName, "original-quarantine"); err != nil {
			t.Fatalf("move original quarantine: %v", err)
		}
		if err := parent.Rename("replacement", quarantineName); err != nil {
			t.Fatalf("install replacement quarantine: %v", err)
		}
	}

	err = root.RemoveFileIfSameIdentity("receipt.json", identity)
	if !errors.Is(err, ErrQuarantineCleanupUncertain) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want quarantine uncertainty", err)
	}
	if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("destination after quarantine race = %v, want absent", statErr)
	}
	if got := mustRead(t, filepath.Join(dir, "original-quarantine")); got != "original" {
		t.Fatalf("original quarantine contents = %q, want original", got)
	}
	if changedQuarantine == "" {
		t.Fatal("quarantine race hook did not capture the random name")
	}
	if got := mustRead(t, filepath.Join(dir, changedQuarantine)); got != "replacement" {
		t.Fatalf("replacement quarantine contents = %q, want replacement", got)
	}
}

func TestReplaceFileIfSameIdentityLeavesChangedQuarantineWithInstalledToken(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("original"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "replacement"), []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("settings.xcconfig")
	if err != nil {
		t.Fatal(err)
	}
	var changedQuarantine string
	root.beforeConditionalQuarantineRemovalForTest = func(parent *os.Root, quarantineName string) {
		changedQuarantine = quarantineName
		if err := parent.Rename(quarantineName, "original-quarantine"); err != nil {
			t.Fatalf("move original quarantine: %v", err)
		}
		if err := parent.Rename("replacement", quarantineName); err != nil {
			t.Fatalf("install replacement quarantine: %v", err)
		}
	}

	installed, err := root.ReplaceFileIfSame("settings.xcconfig", identity, []byte("updated"), 0o640, true)
	requireBoundedRecoveryToken(t, installed)
	if !errors.Is(err, ErrQuarantineCleanupUncertain) {
		t.Fatalf("ReplaceFileIfSame() error = %v, want quarantine uncertainty", err)
	}
	if got := mustRead(t, path); got != "updated" {
		t.Fatalf("published contents = %q, want updated", got)
	}
	if got := mustRead(t, filepath.Join(dir, "original-quarantine")); got != "original" {
		t.Fatalf("original quarantine contents = %q, want original", got)
	}
	if changedQuarantine == "" {
		t.Fatal("quarantine race hook did not capture the random name")
	}
	if got := mustRead(t, filepath.Join(dir, changedQuarantine)); got != "replacement" {
		t.Fatalf("replacement quarantine contents = %q, want replacement", got)
	}
}

func requireStrictIdentityPlatform(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("strict identity mutation requires a native no-replace rename platform")
	}
}

func requireBoundedRecoveryToken(t *testing.T, identity *FileIdentity) {
	t.Helper()
	if identity == nil {
		t.Fatal("strict publication returned nil recovery token")
	}
	if identity.Info() == nil {
		t.Fatal("strict publication recovery token has nil metadata")
	}
	if got := identity.Data(); int64(len(got)) > fileIdentityDataLimit {
		t.Fatalf("strict publication recovery token retained %d bytes, want at most %d", len(got), fileIdentityDataLimit)
	}
}

func TestIdentityChecksRejectSpecialPermissionBitDrift(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "settings.xcconfig")
	const original = "CODE_SIGN_STYLE = Automatic\n"
	if err := os.WriteFile(path, []byte(original), 0o640); err != nil {
		t.Fatal(err)
	}

	identity, err := root.CaptureFile("settings.xcconfig")
	if err != nil {
		t.Fatalf("CaptureFile() error = %v", err)
	}
	// setgid lives outside the ordinary 0777 bits and changes neither the
	// modification time nor the ownership, so it is invisible to every other
	// strict comparison.
	if err := os.Chmod(path, 0o640|os.ModeSetgid); err != nil {
		t.Skipf("set the setgid bit: %v", err)
	}
	drifted, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(source) error = %v", err)
	}
	if drifted.Mode()&os.ModeSetgid == 0 {
		t.Skip("filesystem dropped the setgid bit")
	}

	if err := root.CheckFileIdentity("settings.xcconfig", identity); !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("CheckFileIdentity() error = %v, want ErrFileIdentityChanged", err)
	}
	installed, err := root.ReplaceFileIfSame("settings.xcconfig", identity, []byte("CODE_SIGN_STYLE = Manual\n"), 0o640, true)
	if !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("ReplaceFileIfSame() error = %v, want ErrFileIdentityChanged", err)
	}
	if installed != nil {
		t.Fatal("ReplaceFileIfSame() returned an identity after special-bit drift")
	}
	if err := root.RemoveFileIfSameIdentity("settings.xcconfig", identity); !errors.Is(err, ErrFileIdentityChanged) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want ErrFileIdentityChanged", err)
	}
	if got := mustRead(t, path); got != original {
		t.Fatalf("source after special-bit drift = %q, want %q", got, original)
	}
	matches, globErr := filepath.Glob(filepath.Join(dir, ".asc-tmp-*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) != 0 {
		t.Fatalf("staging or quarantine entries remain after special-bit drift: %v", matches)
	}
}

func TestRootIdentityReleaseFileDropsAndClosesRetainedDescriptor(t *testing.T) {
	dir := t.TempDir()
	root := mustRoot(t, dir)
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("CODE_SIGN_STYLE = Automatic\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open(source) error = %v", err)
	}

	if !root.selectedIdentity.retainFile(file) {
		t.Fatal("retainFile() = false, want the descriptor retained")
	}
	root.selectedIdentity.mu.RLock()
	retained := len(root.selectedIdentity.retainedFiles)
	root.selectedIdentity.mu.RUnlock()
	if retained != 1 {
		t.Fatalf("retained descriptors = %d, want 1", retained)
	}

	// A capture rejected after retention has no FileIdentity to hand back, so
	// the descriptor must be released rather than pinned until Root.Close.
	if err := root.selectedIdentity.releaseFile(file); err != nil {
		t.Fatalf("releaseFile() error = %v", err)
	}
	root.selectedIdentity.mu.RLock()
	retained = len(root.selectedIdentity.retainedFiles)
	root.selectedIdentity.mu.RUnlock()
	if retained != 0 {
		t.Fatalf("retained descriptors after release = %d, want 0", retained)
	}
	if _, err := file.Stat(); err == nil {
		t.Fatal("released descriptor is still open")
	}
	// Releasing again must not double-close, and neither must Root.Close.
	if err := root.selectedIdentity.releaseFile(file); err != nil {
		t.Fatalf("second releaseFile() error = %v", err)
	}
	if err := root.Close(); err != nil {
		t.Fatalf("Close() error = %v after releasing a retained descriptor", err)
	}
}

func TestRemoveFileIfSameIdentityRejectsInPlaceQuarantineWrite(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "receipt.json")
	const original = `{"completed":true}`
	const concurrent = `{"completed":fals}`
	if len(original) != len(concurrent) {
		t.Fatalf("fixture lengths differ: %d vs %d", len(original), len(concurrent))
	}
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	identity, err := root.CaptureFile("receipt.json")
	if err != nil {
		t.Fatalf("CaptureFile() error = %v", err)
	}
	// Overwrite the quarantined inode in place, keeping its length, after the
	// verification handle closed but before the final recheck and unlink.
	var wrote bool
	root.beforeConditionalQuarantineRemovalForTest = func(parent *os.Root, quarantineName string) {
		file, openErr := parent.OpenFile(quarantineName, os.O_WRONLY, 0)
		if openErr != nil {
			t.Fatalf("open quarantined file: %v", openErr)
		}
		if _, writeErr := file.WriteAt([]byte(concurrent), 0); writeErr != nil {
			t.Fatalf("overwrite quarantined file: %v", writeErr)
		}
		if closeErr := file.Close(); closeErr != nil {
			t.Fatalf("close quarantined file: %v", closeErr)
		}
		wrote = true
	}

	err = root.RemoveFileIfSameIdentity("receipt.json", identity)
	if !errors.Is(err, ErrQuarantineCleanupUncertain) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want ErrQuarantineCleanupUncertain", err)
	}
	if !errors.Is(err, ErrFileIdentityRemoved) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want removed sentinel while canonical path is absent", err)
	}
	if !wrote {
		t.Fatal("quarantine removal seam was not invoked")
	}
	matches, globErr := filepath.Glob(filepath.Join(dir, ".asc-tmp-rollback-*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) != 1 {
		t.Fatalf("quarantine entries = %v, want the concurrently edited copy preserved", matches)
	}
	if got := mustRead(t, matches[0]); got != concurrent {
		t.Fatalf("preserved quarantine content = %q, want the concurrent edit %q", got, concurrent)
	}
}

func TestRemoveFileIfSameIdentityReportsRemovedAfterQuarantineCleanupFailure(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "receipt.json")
	if err := os.WriteFile(path, []byte("receipt"), 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("receipt.json")
	if err != nil {
		t.Fatalf("CaptureFile() error = %v", err)
	}
	var quarantineName string
	root.beforeConditionalQuarantineRemovalForTest = func(parent *os.Root, name string) {
		quarantineName = name
		file, openErr := parent.OpenFile(name, os.O_WRONLY, 0)
		if openErr != nil {
			t.Fatalf("open quarantined file: %v", openErr)
		}
		if chmodErr := file.Chmod(0o640); chmodErr != nil {
			_ = file.Close()
			t.Fatalf("change quarantined file mode: %v", chmodErr)
		}
		if closeErr := file.Close(); closeErr != nil {
			t.Fatalf("close changed quarantine: %v", closeErr)
		}
	}

	err = root.RemoveFileIfSameIdentity("receipt.json", identity)
	if !errors.Is(err, ErrQuarantineCleanupUncertain) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want quarantine uncertainty", err)
	}
	if !errors.Is(err, ErrFileIdentityRemoved) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want removed sentinel after canonical absence", err)
	}
	if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("destination after cleanup failure = %v, want absent", statErr)
	}
	if quarantineName == "" {
		t.Fatal("quarantine cleanup hook did not capture the random name")
	}
	quarantinePath := filepath.Join(dir, quarantineName)
	quarantineInfo, statErr := os.Stat(quarantinePath)
	if statErr != nil {
		t.Fatalf("stat preserved quarantine: %v", statErr)
	}
	if got := quarantineInfo.Mode().Perm(); got != 0o640 {
		t.Fatalf("preserved quarantine mode = %o, want changed mode 640", got)
	}
	if !strings.Contains(err.Error(), quarantineName) {
		t.Fatalf("cleanup error = %v, want quarantine name %q", err, quarantineName)
	}
}

func TestRemoveFileIfSameIdentityDoesNotReportRemovedWhenCleanupReplacementAppears(t *testing.T) {
	requireStrictIdentityPlatform(t)
	dir := t.TempDir()
	root := mustRoot(t, dir)
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "receipt.json")
	if err := os.WriteFile(path, []byte("receipt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "replacement"), []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	identity, err := root.CaptureFile("receipt.json")
	if err != nil {
		t.Fatalf("CaptureFile() error = %v", err)
	}
	var quarantineName string
	root.beforeConditionalQuarantineRemovalForTest = func(parent *os.Root, name string) {
		quarantineName = name
		file, openErr := parent.OpenFile(name, os.O_WRONLY, 0)
		if openErr != nil {
			t.Fatalf("open quarantined file: %v", openErr)
		}
		if chmodErr := file.Chmod(0o640); chmodErr != nil {
			_ = file.Close()
			t.Fatalf("change quarantined file mode: %v", chmodErr)
		}
		if closeErr := file.Close(); closeErr != nil {
			t.Fatalf("close changed quarantine: %v", closeErr)
		}
		if renameErr := parent.Rename("replacement", "receipt.json"); renameErr != nil {
			t.Fatalf("install replacement receipt: %v", renameErr)
		}
	}

	err = root.RemoveFileIfSameIdentity("receipt.json", identity)
	if !errors.Is(err, ErrQuarantineCleanupUncertain) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, want quarantine uncertainty", err)
	}
	if errors.Is(err, ErrFileIdentityRemoved) {
		t.Fatalf("RemoveFileIfSameIdentity() error = %v, must not claim the replacement was removed", err)
	}
	if got := mustRead(t, path); got != "replacement" {
		t.Fatalf("replacement receipt = %q, want preserved replacement", got)
	}
	if quarantineName == "" {
		t.Fatal("quarantine cleanup hook did not capture the random name")
	}
	if got := mustRead(t, filepath.Join(dir, quarantineName)); got != "receipt" {
		t.Fatalf("preserved quarantine = %q, want original receipt", got)
	}
}
