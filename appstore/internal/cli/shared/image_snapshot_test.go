package shared

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"
)

func TestSnapshotImageFileUnlinksSnapshotPath(t *testing.T) {
	probe, err := os.CreateTemp(t.TempDir(), "unlink-probe-*")
	if err != nil {
		t.Fatalf("create unlink probe: %v", err)
	}
	probePath := probe.Name()
	canUnlinkOpenFile := os.Remove(probePath) == nil
	if err := probe.Close(); err != nil {
		t.Fatalf("close unlink probe: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(probePath) })
	if !canUnlinkOpenFile {
		if _, err := os.Stat(probePath); err != nil {
			t.Fatalf("unlink probe path stat error = %v, want path to remain until close", err)
		}
	}

	source, err := os.CreateTemp(t.TempDir(), "source-*.png")
	if err != nil {
		t.Fatalf("create source image: %v", err)
	}
	defer source.Close()

	want := []byte("image bytes")
	if _, err := source.Write(want); err != nil {
		t.Fatalf("write source image: %v", err)
	}

	snapshot, cleanup, err := SnapshotImageFile(source, int64(len(want)))
	if err != nil {
		t.Fatalf("snapshot image: %v", err)
	}
	defer cleanup()

	snapshotPath := snapshot.Name()
	if _, err := os.Stat(snapshotPath); canUnlinkOpenFile {
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("snapshot path stat error = %v, want not-exist when unlinking open files is supported", err)
		}
	} else if err != nil {
		t.Fatalf("snapshot path stat error = %v, want path to remain until cleanup", err)
	}
	got, err := io.ReadAll(snapshot)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("snapshot bytes = %q, want %q", got, want)
	}

	cleanup()
	if _, err := os.Stat(snapshotPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("snapshot path stat after cleanup = %v, want not-exist", err)
	}
}
