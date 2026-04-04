package asc

import (
	"fmt"
	"strings"
)

func testFlightPublishResultRows(result *TestFlightPublishResult) ([]string, [][]string) {
	headers := []string{"Build ID", "Version", "Build Number", "Processing", "Groups", "Uploaded", "Notified", "Notification Action"}
	notified := ""
	if result.Notified != nil {
		notified = fmt.Sprintf("%t", *result.Notified)
	}
	rows := [][]string{{
		result.BuildID,
		result.BuildVersion,
		result.BuildNumber,
		result.ProcessingState,
		strings.Join(result.GroupIDs, ", "),
		fmt.Sprintf("%t", result.Uploaded),
		notified,
		string(result.NotificationAction),
	}}
	return headers, rows
}

func appStorePublishResultRows(result *AppStorePublishResult) ([]string, [][]string) {
	headers := []string{"Build ID", "Version", "Build Number", "Version ID", "Submission ID", "Uploaded", "Attached", "Submitted"}
	rows := [][]string{{
		result.BuildID,
		result.BuildVersion,
		result.BuildNumber,
		result.VersionID,
		result.SubmissionID,
		fmt.Sprintf("%t", result.Uploaded),
		fmt.Sprintf("%t", result.Attached),
		fmt.Sprintf("%t", result.Submitted),
	}}
	return headers, rows
}

func publishArchiveStageRows(stage *PublishArchiveStageResult) ([]string, [][]string) {
	if stage == nil {
		return []string{"Field", "Value"}, nil
	}
	rows := [][]string{
		{"archive_path", stage.ArchivePath},
		{"bundle_id", stage.BundleID},
		{"version", stage.Version},
		{"build_number", stage.BuildNumber},
		{"scheme", stage.Scheme},
	}
	if strings.TrimSpace(stage.Configuration) != "" {
		rows = append(rows, []string{"configuration", stage.Configuration})
	}
	return []string{"Field", "Value"}, rows
}

func publishExportStageRows(stage *PublishExportStageResult) ([]string, [][]string) {
	if stage == nil {
		return []string{"Field", "Value"}, nil
	}
	ipaPath := stage.IPAPath
	if strings.TrimSpace(ipaPath) == "" {
		ipaPath = "(direct upload - no local artifact)"
	}
	rows := [][]string{
		{"archive_path", stage.ArchivePath},
		{"ipa_path", ipaPath},
		{"bundle_id", stage.BundleID},
		{"version", stage.Version},
		{"build_number", stage.BuildNumber},
		{"export_options_path", stage.ExportOptionsPath},
		{"direct_upload", fmt.Sprintf("%t", stage.DirectUpload)},
	}
	return []string{"Field", "Value"}, rows
}
