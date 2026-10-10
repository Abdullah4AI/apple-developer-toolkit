package publish

import (
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

func TestPublishAppStoreTimeoutHelpMatchesScope(t *testing.T) {
	timeoutFlag := PublishAppStoreCommand().FlagSet.Lookup("timeout")
	if timeoutFlag == nil {
		t.Fatal("expected --timeout flag")
	}
	want := "Override upload + processing timeout; also applies to submission with --submit (e.g., 30m)"
	if timeoutFlag.Usage != want {
		t.Fatalf("expected timeout help %q, got %q", want, timeoutFlag.Usage)
	}
}

func TestPublishTestFlightUploadOnlyHelpIsDiscoverable(t *testing.T) {
	uploadOnlyFlag := PublishTestFlightCommand().FlagSet.Lookup("upload-only")
	if uploadOnlyFlag == nil {
		t.Fatal("expected --upload-only flag")
	}
	want := "Upload the build without adding it to beta groups or submitting beta review"
	if uploadOnlyFlag.Usage != want {
		t.Fatalf("expected upload-only help %q, got %q", want, uploadOnlyFlag.Usage)
	}
}

func TestReportSkippedInternalAllBuildsGroupsSanitizesProviderID(t *testing.T) {
	const groupID = "group-control-\x1b[31mID\nNEXT"

	_, stderr := capturePublishCommandOutput(t, func() error {
		reportSkippedInternalAllBuildsGroups([]shared.ResolvedBetaGroup{{
			ID:   groupID,
			Name: "QA-\x1b[31mNAME\nINJECT",
		}})
		return nil
	})

	want := `Skipped internal group "QA-\x1b[31mNAME\nINJECT" (group-control-[31mID NEXT) because it already receives all builds` + "\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	if strings.Contains(stderr, "\x1b") || strings.Contains(stderr, groupID) {
		t.Fatalf("stderr contains unsanitized provider ID: %q", stderr)
	}
}
