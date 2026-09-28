package asc

// Signing sync profile lifecycle actions reported by push.
const (
	SigningSyncLifecycleUnchanged        = "unchanged"
	SigningSyncLifecycleRenewed          = "renewed"
	SigningSyncLifecycleDevicesRefreshed = "devices-refreshed"
)

// Replaced profile states reported by push when a profile is recreated.
const (
	SigningSyncReplacedProfileDeleted = "deleted"
	SigningSyncReplacedProfileUnknown = "unknown"
)

// SigningSyncProfileLifecycle reports how signing sync push treated an
// existing profile when --renew-expired or --force-for-new-devices is set.
//
// Action is unchanged when the existing profile was reused, renewed when an
// expired profile was replaced, or devices-refreshed when a profile was
// replaced because its devices differed from the requested device set.
// ReplacedProfileState is deleted after Apple confirmed the deletion of the
// replaced profile and unknown when the deletion request failed.
type SigningSyncProfileLifecycle struct {
	Action                 string   `json:"action"`
	ProfileID              string   `json:"profileId,omitempty"`
	ReplacedProfileID      string   `json:"replacedProfileId,omitempty"`
	ReplacedProfileName    string   `json:"replacedProfileName,omitempty"`
	ReplacedExpirationDate string   `json:"replacedExpirationDate,omitempty"`
	ReplacedProfileState   string   `json:"replacedProfileState,omitempty"`
	DevicesAdded           []string `json:"devicesAdded,omitempty"`
	DevicesRemoved         []string `json:"devicesRemoved,omitempty"`
}
