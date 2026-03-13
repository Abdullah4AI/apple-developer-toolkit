package asc

import "testing"

func TestAutoResolveAppInfoIDByVersionState(t *testing.T) {
	tests := []struct {
		name         string
		versionState string
		candidates   []appInfoCandidate
		wantID       string
		wantOK       bool
	}{
		{
			name:         "matches exact shared state",
			versionState: "WAITING_FOR_REVIEW",
			candidates: []appInfoCandidate{
				{id: "info-1", state: "PREPARE_FOR_SUBMISSION"},
				{id: "info-2", state: "WAITING_FOR_REVIEW"},
			},
			wantID: "info-2",
			wantOK: true,
		},
		{
			name:         "maps pending developer release to pending release",
			versionState: "PENDING_DEVELOPER_RELEASE",
			candidates: []appInfoCandidate{
				{id: "info-1", state: "PREPARE_FOR_SUBMISSION"},
				{id: "info-2", state: "PENDING_RELEASE"},
			},
			wantID: "info-2",
			wantOK: true,
		},
		{
			name:         "maps pending apple release to pending release",
			versionState: "PENDING_APPLE_RELEASE",
			candidates: []appInfoCandidate{
				{id: "info-1", state: "WAITING_FOR_REVIEW"},
				{id: "info-2", state: "PENDING_RELEASE"},
			},
			wantID: "info-2",
			wantOK: true,
		},
		{
			name:         "maps replaced with new version to replaced with new info",
			versionState: "REPLACED_WITH_NEW_VERSION",
			candidates: []appInfoCandidate{
				{id: "info-1", state: "READY_FOR_REVIEW"},
				{id: "info-2", state: "REPLACED_WITH_NEW_INFO"},
			},
			wantID: "info-2",
			wantOK: true,
		},
		{
			name:         "maps ready for sale fallback to ready for distribution",
			versionState: "READY_FOR_SALE",
			candidates: []appInfoCandidate{
				{id: "info-1", state: "READY_FOR_REVIEW"},
				{id: "info-2", state: "READY_FOR_DISTRIBUTION"},
			},
			wantID: "info-2",
			wantOK: true,
		},
		{
			name:         "returns false when alias remains ambiguous",
			versionState: "PENDING_DEVELOPER_RELEASE",
			candidates: []appInfoCandidate{
				{id: "info-1", state: "PENDING_RELEASE"},
				{id: "info-2", state: "PENDING_RELEASE"},
			},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotOK := autoResolveAppInfoIDByVersionState(tt.candidates, tt.versionState)
			if gotOK != tt.wantOK {
				t.Fatalf("autoResolveAppInfoIDByVersionState() ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotID != tt.wantID {
				t.Fatalf("autoResolveAppInfoIDByVersionState() id = %q, want %q", gotID, tt.wantID)
			}
		})
	}
}
