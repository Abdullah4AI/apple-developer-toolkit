package status

import (
	"slices"
	"testing"
)

func TestResolveNextCommands(t *testing.T) {
	validBuild := &latestBuild{ID: "build-9", Version: "1.2.3", BuildNumber: "9", ProcessingState: "VALID", Platform: "IOS"}
	tests := []struct {
		name     string
		resp     *dashboardResponse
		platform string
		readOnly bool
		want     []string
	}{
		{
			name: "prepare with matching valid build validates then submits",
			resp: &dashboardResponse{
				Builds:   &buildsSection{Latest: validBuild},
				AppStore: &appStoreSection{VersionID: "ver-1", Version: "1.2.3", Platform: "IOS", State: "PREPARE_FOR_SUBMISSION"},
			},
			want: []string{
				"asc validate --app 123 --version-id ver-1",
				"asc review submit --app 123 --version-id ver-1 --build-id build-9 --confirm",
			},
		},
		{
			name: "prepare with build from another version only validates",
			resp: &dashboardResponse{
				Builds:   &buildsSection{Latest: validBuild},
				AppStore: &appStoreSection{VersionID: "ver-2", Version: "1.3.0", Platform: "IOS", State: "PREPARE_FOR_SUBMISSION"},
			},
			want: []string{"asc validate --app 123 --version-id ver-2"},
		},
		{
			name: "processing build and version in review",
			resp: &dashboardResponse{
				Builds:   &buildsSection{Latest: &latestBuild{ID: "build-10", ProcessingState: "PROCESSING"}},
				AppStore: &appStoreSection{VersionID: "ver-1", State: "IN_REVIEW"},
			},
			platform: "MAC_OS",
			want: []string{
				"asc builds wait --build-id build-10",
				"asc status --app 123 --platform MAC_OS --until review-done",
			},
		},
		{
			name: "rejected version",
			resp: &dashboardResponse{AppStore: &appStoreSection{VersionID: "ver-1", State: "METADATA_REJECTED"}},
			want: []string{"asc review doctor --app 123 --version-id ver-1"},
		},
		{
			name:     "read-only mode omits mutating commands",
			resp:     &dashboardResponse{AppStore: &appStoreSection{VersionID: "ver-1", State: "PENDING_DEVELOPER_RELEASE"}},
			readOnly: true,
			want:     []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			readOnly := ""
			if test.readOnly {
				readOnly = "1"
			}
			t.Setenv("ASC_READ_ONLY", readOnly)
			got := []string{}
			for _, next := range resolveNextCommands(test.resp, "123", test.platform) {
				got = append(got, next.Command)
			}
			if !slices.Equal(got, test.want) {
				t.Fatalf("commands = %q, want %q", got, test.want)
			}
		})
	}
}
