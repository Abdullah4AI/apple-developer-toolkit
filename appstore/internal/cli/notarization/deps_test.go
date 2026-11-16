package notarization

import (
	"bytes"
	"os"
	"strings"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

// newNotarizationTestDeps returns deps that write to buffers and take their
// client from client, with the process timeouts, timer and working directory.
func newNotarizationTestDeps(client func() (*asc.Client, error)) *shared.Deps {
	return &shared.Deps{
		Client:        client,
		IO:            &shared.IO{In: strings.NewReader(""), Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}},
		Timeout:       shared.ContextWithTimeout,
		UploadTimeout: shared.ContextWithUploadTimeout,
		After:         time.After,
		Getwd:         os.Getwd,
	}
}
