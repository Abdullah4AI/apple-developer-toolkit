package versions

import (
	"bytes"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

// newVersionsTestDeps returns deps that write to buffers and take their client
// from client, with the process timeouts. It returns the stderr buffer.
func newVersionsTestDeps(client func() (*asc.Client, error)) (*shared.Deps, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return &shared.Deps{
		Client:  client,
		IO:      &shared.IO{Out: &stdout, ErrOut: &stderr},
		Now:     time.Now,
		Timeout: shared.ContextWithTimeout,
	}, &stderr
}
