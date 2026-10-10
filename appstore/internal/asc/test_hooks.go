package asc

import "time"

// ResetConfigCacheForTest clears cached config state for tests.
func ResetConfigCacheForTest() {
	resetConfigCacheForTest()
}

// SetAssetLibraryProcessingPollIntervalForTest replaces how often Asset Library
// uploads poll for processing. The returned function restores the previous value.
func SetAssetLibraryProcessingPollIntervalForTest(interval time.Duration) func() {
	previous := assetLibraryProcessingPollInterval
	assetLibraryProcessingPollInterval = interval
	return func() { assetLibraryProcessingPollInterval = previous }
}
