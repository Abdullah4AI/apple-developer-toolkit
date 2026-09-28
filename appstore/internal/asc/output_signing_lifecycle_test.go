package asc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSigningSyncProfileLifecycleJSONIsAdditive(t *testing.T) {
	single := SigningSyncResult{Operation: "push", Files: []string{}}
	data, err := json.Marshal(single)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "profileLifecycle") {
		t.Fatalf("result without lifecycle must omit profileLifecycle: %s", data)
	}

	single.ProfileLifecycle = &SigningSyncProfileLifecycle{
		Action:               SigningSyncLifecycleDevicesRefreshed,
		ProfileID:            "profile-new",
		ReplacedProfileID:    "profile-old",
		ReplacedProfileName:  "Dev",
		ReplacedProfileState: SigningSyncReplacedProfileDeleted,
		DevicesAdded:         []string{"device-2"},
		DevicesRemoved:       []string{"device-3"},
	}
	data, err = json.Marshal(single)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{
		`"profileLifecycle":{"action":"devices-refreshed"`,
		`"replacedProfileId":"profile-old"`,
		`"replacedProfileState":"deleted"`,
		`"devicesAdded":["device-2"]`,
		`"devicesRemoved":["device-3"]`,
	} {
		if !strings.Contains(string(data), field) {
			t.Fatalf("lifecycle JSON missing %s: %s", field, data)
		}
	}

	target := SigningSyncTargetResult{BundleID: "com.example.app", Files: []string{}, ProfileLifecycle: &SigningSyncProfileLifecycle{Action: SigningSyncLifecycleUnchanged}}
	data, err = json.Marshal(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"profileLifecycle":{"action":"unchanged"}`) {
		t.Fatalf("target lifecycle JSON = %s", data)
	}
}
