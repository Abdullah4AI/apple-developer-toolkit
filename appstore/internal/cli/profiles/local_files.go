package profiles

import (
	"bytes"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

func writeProfileFile(path string, content []byte, force bool) error {
	if !force {
		return shared.WriteProfileFile(path, content)
	}
	_, err := shared.WriteFileNoSymlinkOverwrite(path, bytes.NewReader(content), 0o644, ".asc-profile-*", ".asc-profile-backup-*")
	return err
}
