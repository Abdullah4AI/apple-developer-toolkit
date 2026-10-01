//go:build windows

package screenshots

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/rootfs"
)

// windowsDirectoryLink creates a Windows directory redirection at link that
// resolves to target, using only Go APIs. It skips the test when the host
// does not grant the privilege the link kind needs.
type windowsDirectoryLink struct {
	name   string
	create func(t *testing.T, link, target string)
}

var windowsDirectoryLinks = []windowsDirectoryLink{
	{name: "junction", create: createJunctionForTest},
	{name: "directory symlink", create: createDirectorySymlinkForTest},
}

func createDirectorySymlinkForTest(t *testing.T, link, target string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		if errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
			t.Skip("directory symlink creation requires SeCreateSymbolicLinkPrivilege or Developer Mode")
		}
		t.Fatalf("create directory symlink: %v", err)
	}
}

// createJunctionForTest creates an NTFS mount-point reparse point (junction)
// at link. Junctions need no special privilege, so this fixture always runs.
func createJunctionForTest(t *testing.T, link, target string) {
	t.Helper()
	absTarget, err := filepath.Abs(target)
	if err != nil {
		t.Fatal(err)
	}
	substitute, err := windows.UTF16FromString(`\??\` + absTarget)
	if err != nil {
		t.Fatal(err)
	}
	printName, err := windows.UTF16FromString(absTarget)
	if err != nil {
		t.Fatal(err)
	}
	substituteBytes := (len(substitute) - 1) * 2
	printBytes := (len(printName) - 1) * 2
	pathBuffer := append(append([]uint16{}, substitute...), printName...)

	// REPARSE_DATA_BUFFER with a MountPointReparseBuffer: an 8-byte header,
	// four USHORT name offsets and lengths, then both NUL-terminated names.
	dataLength := 8 + len(pathBuffer)*2
	buffer := make([]byte, 8+dataLength)
	binary.LittleEndian.PutUint32(buffer[0:], windows.IO_REPARSE_TAG_MOUNT_POINT)
	binary.LittleEndian.PutUint16(buffer[4:], uint16(dataLength))
	binary.LittleEndian.PutUint16(buffer[8:], 0)
	binary.LittleEndian.PutUint16(buffer[10:], uint16(substituteBytes))
	binary.LittleEndian.PutUint16(buffer[12:], uint16(substituteBytes+2))
	binary.LittleEndian.PutUint16(buffer[14:], uint16(printBytes))
	for index, unit := range pathBuffer {
		binary.LittleEndian.PutUint16(buffer[16+2*index:], unit)
	}

	if err := os.Mkdir(link, 0o755); err != nil {
		t.Fatalf("create junction directory: %v", err)
	}
	linkPtr, err := windows.UTF16PtrFromString(link)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(
		linkPtr,
		windows.GENERIC_WRITE,
		0,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		t.Fatalf("open junction directory: %v", err)
	}
	defer windows.CloseHandle(handle)
	var returned uint32
	if err := windows.DeviceIoControl(handle, windows.FSCTL_SET_REPARSE_POINT, &buffer[0], uint32(len(buffer)), nil, 0, &returned, nil); err != nil {
		t.Fatalf("set junction reparse point: %v", err)
	}
}

// installDirectoryLink replaces the directory at path with a link to target,
// proves the link really redirects there, and removes the link (never its
// target's contents) when the test ends.
func installDirectoryLink(t *testing.T, kind windowsDirectoryLink, path, target string) {
	t.Helper()
	kind.create(t, path, target)
	t.Cleanup(func() { _ = os.Remove(path) })
	probe := filepath.Join(path, "probe")
	if err := os.WriteFile(probe, []byte("probe"), 0o600); err != nil {
		t.Fatalf("write through %s: %v", kind.name, err)
	}
	if _, err := os.Stat(filepath.Join(target, "probe")); err != nil {
		t.Fatalf("%s does not redirect to its target: %v", kind.name, err)
	}
	if err := os.Remove(probe); err != nil {
		t.Fatal(err)
	}
}

func writeGeneratedScreenshotSource(t *testing.T) *os.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "generated.png")
	if err := os.WriteFile(path, []byte("generated"), 0o600); err != nil {
		t.Fatal(err)
	}
	source, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = source.Close() })
	return source
}

func assertDirectoryEmpty(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("outside directory gained %v, want no write through the swapped link", names)
	}
}

// TestPublishGeneratedScreenshotWindowsRejectsSwappedParentLink replaces a
// nested output directory with a junction or directory symlink after the
// output root is anchored. Publication must fail without writing outside.
func TestPublishGeneratedScreenshotWindowsRejectsSwappedParentLink(t *testing.T) {
	for _, kind := range windowsDirectoryLinks {
		t.Run(kind.name, func(t *testing.T) {
			dir := t.TempDir()
			outside := t.TempDir()
			nested := filepath.Join(dir, "framed")
			if err := os.Mkdir(nested, 0o755); err != nil {
				t.Fatal(err)
			}
			outputRoot, err := rootfs.New(dir)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = outputRoot.Close() })

			if err := os.Rename(nested, filepath.Join(dir, "framed-original")); err != nil {
				t.Fatalf("move nested output directory: %v", err)
			}
			installDirectoryLink(t, kind, nested, outside)

			source := writeGeneratedScreenshotSource(t)
			if _, err := publishGeneratedScreenshot(t.Context(), source, outputRoot, filepath.Join("framed", "home.png"), maxMatrixArtifactBytes); err == nil {
				t.Fatalf("publishGeneratedScreenshot() error = nil, want %s parent rejection", kind.name)
			}
			assertDirectoryEmpty(t, outside)
			if _, err := os.Lstat(filepath.Join(dir, "framed-original", "home.png")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("original nested directory stat = %v, want no publication", err)
			}
		})
	}
}

// TestPublishGeneratedScreenshotWindowsStaysInAnchoredRootAfterSwap replaces
// the anchored output directory itself with a junction or directory symlink,
// as matrixFrameRootBeforePublishForTest does between render and publication.
// The image must land in the anchored directory or be rejected; it must never
// follow the replacement link.
func TestPublishGeneratedScreenshotWindowsStaysInAnchoredRootAfterSwap(t *testing.T) {
	for _, kind := range windowsDirectoryLinks {
		t.Run(kind.name, func(t *testing.T) {
			dir := t.TempDir()
			outside := t.TempDir()
			selected := filepath.Join(dir, "framed")
			anchored := filepath.Join(dir, "framed-anchored")
			if err := os.Mkdir(selected, 0o755); err != nil {
				t.Fatal(err)
			}
			outputRoot, err := rootfs.New(selected)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = outputRoot.Close() })

			source := writeGeneratedScreenshotSource(t)
			if err := os.Rename(selected, anchored); err != nil {
				// The retained root handle may deny rename sharing, which
				// already prevents the swap. Publication must then succeed in
				// the anchored directory.
				t.Logf("anchored output directory cannot be renamed: %v", err)
				if _, err := publishGeneratedScreenshot(t.Context(), source, outputRoot, "home.png", maxMatrixArtifactBytes); err != nil {
					t.Fatalf("publishGeneratedScreenshot() error = %v", err)
				}
				if data, err := os.ReadFile(filepath.Join(selected, "home.png")); err != nil || string(data) != "generated" {
					t.Fatalf("published image = %q, %v; want generated", data, err)
				}
				return
			}
			installDirectoryLink(t, kind, selected, outside)

			_, publishErr := publishGeneratedScreenshot(t.Context(), source, outputRoot, "home.png", maxMatrixArtifactBytes)
			assertDirectoryEmpty(t, outside)
			if publishErr != nil {
				return
			}
			if data, err := os.ReadFile(filepath.Join(anchored, "home.png")); err != nil || string(data) != "generated" {
				t.Fatalf("anchored image = %q, %v; want generated in the anchored directory", data, err)
			}
		})
	}
}
