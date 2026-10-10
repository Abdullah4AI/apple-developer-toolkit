//go:build !darwin

package signing

import (
	"context"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

func platformSigningRunDeps() signingRunDeps {
	return signingRunDeps{GOOS: "unsupported", Stderr: os.Stderr}
}

func signingRunSecurityAvailable() bool { return false }

func systemSigningRunRoots() (*x509.CertPool, error) { return x509.SystemCertPool() }

func platformSigningRunContext(ctx context.Context) (context.Context, func()) {
	return ctx, func() {}
}

func runSigningRunChild(context.Context, []string) error {
	return shared.NewValidationError(fmt.Errorf("signing run is supported only on macOS"))
}
