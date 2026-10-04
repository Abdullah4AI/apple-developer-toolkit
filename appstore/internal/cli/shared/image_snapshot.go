package shared

import (
	"fmt"
	"io"
	"os"
)

// SnapshotImageFile copies the selected image into a private temporary file
// before any checksum or upload work begins. Hashing and uploading the source
// pathname independently would allow an in-place rewrite between those reads
// to commit a checksum for different bytes than the upload sent.
func SnapshotImageFile(source *os.File, size int64) (*os.File, func(), error) {
	if source == nil {
		return nil, func() {}, fmt.Errorf("image source file is required")
	}
	if size <= 0 {
		return nil, func() {}, fmt.Errorf("image source file must not be empty")
	}

	snapshot, err := os.CreateTemp("", ".asc-image-*")
	if err != nil {
		return nil, func() {}, fmt.Errorf("create image snapshot: %w", err)
	}
	snapshotPath := snapshot.Name()
	// Unlink immediately where the OS permits removing an open file. Some
	// platforms keep the path until cleanup closes the snapshot descriptor.
	snapshotPathLinked := os.Remove(snapshotPath) != nil
	cleanup := func() {
		_ = snapshot.Close()
		if snapshotPathLinked {
			_ = os.Remove(snapshotPath)
		}
	}

	if _, err := io.CopyN(snapshot, io.NewSectionReader(source, 0, size), size); err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("copy image snapshot: %w", err)
	}
	if _, err := snapshot.Seek(0, io.SeekStart); err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("rewind image snapshot: %w", err)
	}
	return snapshot, cleanup, nil
}
