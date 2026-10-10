package shared

import (
	"fmt"
	"os"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

var mkdirTempForTest = os.MkdirTemp

// SetASCClientFactoryForTesting replaces the general command ASC client factory.
// It returns a restore function to reset the previous handler.
func SetASCClientFactoryForTesting(fn func() (*asc.Client, error)) func() {
	ascClientFactoryMu.Lock()
	previous := ascClientFactory
	if fn == nil {
		ascClientFactory = getASCClient
	} else {
		ascClientFactory = fn
	}
	ascClientFactoryMu.Unlock()
	return func() {
		ascClientFactoryMu.Lock()
		ascClientFactory = previous
		ascClientFactoryMu.Unlock()
	}
}

// SetAvailabilityClientFactory replaces the ASC client factory for availability tests.
// It returns a restore function to reset the previous handler.
func SetAvailabilityClientFactory(fn func() (*asc.Client, error)) func() {
	previous := availabilityClientFactory
	if fn == nil {
		availabilityClientFactory = getASCClient
	} else {
		availabilityClientFactory = fn
	}
	return func() {
		availabilityClientFactory = previous
	}
}

// ResetTierCacheForTest routes tier-cache reads and writes to an isolated temp dir for tests.
func ResetTierCacheForTest() {
	tierCacheDirOverrideMu.Lock()
	override := tierCacheDirOverride
	if override == "" {
		tempDir, err := mkdirTempForTest("", "asc-tier-cache-*")
		if err != nil {
			tierCacheDirOverrideMu.Unlock()
			panic(fmt.Errorf("create isolated tier cache dir: %w", err))
		}
		override = tempDir
		tierCacheDirOverride = override
	}
	tierCacheDirOverrideMu.Unlock()

	if err := os.RemoveAll(override); err != nil {
		panic(fmt.Errorf("reset isolated tier cache dir: %w", err))
	}
	if err := os.MkdirAll(override, 0o755); err != nil {
		panic(fmt.Errorf("recreate isolated tier cache dir: %w", err))
	}
}

func resetTierCacheDirOverrideForTest() {
	tierCacheDirOverrideMu.Lock()
	override := tierCacheDirOverride
	tierCacheDirOverride = ""
	tierCacheDirOverrideMu.Unlock()

	if override == "" {
		return
	}
	_ = os.RemoveAll(override)
}

// SetPricingNowForTesting replaces the clock pricing commands use to decide
// today's pricing date, such as the default --start-date and the current
// price. It returns a restore function to reset the clock.
func SetPricingNowForTesting(fn func() time.Time) func() {
	previous := pricingNow
	if fn == nil {
		pricingNow = time.Now
	} else {
		pricingNow = fn
	}
	return func() {
		pricingNow = previous
	}
}

// ResetEquivalentVersionNotesForTest forgets which equivalent-version notes
// were already printed, so each in-process command run reports them again.
func ResetEquivalentVersionNotesForTest() {
	equivalentVersionNoteMu.Lock()
	defer equivalentVersionNoteMu.Unlock()

	equivalentVersionNotes = map[string]struct{}{}
}
