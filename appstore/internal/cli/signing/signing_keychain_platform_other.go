//go:build !darwin

package signing

import "github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"

func platformSigningKeychainInstallDeps(*shared.Deps, bool) signingKeychainInstallDeps {
	return signingKeychainInstallDeps{GOOS: "unsupported"}
}
