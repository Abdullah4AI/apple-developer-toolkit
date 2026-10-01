package shared

import (
	"strings"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

// ResolveAppStoreVersionState prefers the app version state when available.
func ResolveAppStoreVersionState(attrs asc.AppStoreVersionAttributes) string {
	if attrs.AppVersionState != "" {
		return attrs.AppVersionState
	}
	return attrs.AppStoreState
}

// IsLiveAppStoreVersionState reports whether a resolved version state means
// the version is live on the App Store. appVersionState reports this as
// READY_FOR_DISTRIBUTION, while the legacy appStoreState uses READY_FOR_SALE.
func IsLiveAppStoreVersionState(state string) bool {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "READY_FOR_DISTRIBUTION", "READY_FOR_SALE":
		return true
	default:
		return false
	}
}
