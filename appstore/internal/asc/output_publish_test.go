package asc

import "testing"

func TestAppStorePublishResultRowsDryRunUsesPlanSummaryColumns(t *testing.T) {
	result := &AppStorePublishResult{
		Mode:         PublishModeIPAUpload,
		DryRun:       true,
		BuildVersion: "1.2.3",
		BuildNumber:  "42",
		Plan: []PublishPlanStep{
			{Name: "upload_build", Status: "planned"},
			{Name: "wait_for_build_processing", Status: "planned"},
			{Name: "submit_review", Status: "planned"},
		},
	}

	headers, rows := appStorePublishResultRows(result)
	if len(headers) != 6 {
		t.Fatalf("expected 6 headers, got %d (%v)", len(headers), headers)
	}
	expectedHeaders := []string{"Dry Run", "Mode", "Version", "Build Number", "Will Wait", "Will Submit"}
	for i, want := range expectedHeaders {
		if headers[i] != want {
			t.Fatalf("expected header %d to be %q, got %q", i, want, headers[i])
		}
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if len(rows[0]) != len(headers) {
		t.Fatalf("expected row to have %d columns, got %d (%v)", len(headers), len(rows[0]), rows[0])
	}
	if rows[0][0] != "true" {
		t.Fatalf("expected dry-run column to be true, got %q", rows[0][0])
	}
	if rows[0][1] != string(PublishModeIPAUpload) {
		t.Fatalf("expected mode column %q, got %q", PublishModeIPAUpload, rows[0][1])
	}
	if rows[0][2] != "1.2.3" {
		t.Fatalf("expected version column 1.2.3, got %q", rows[0][2])
	}
	if rows[0][3] != "42" {
		t.Fatalf("expected build number column 42, got %q", rows[0][3])
	}
	if rows[0][4] != "true" {
		t.Fatalf("expected will-wait column true, got %q", rows[0][4])
	}
	if rows[0][5] != "true" {
		t.Fatalf("expected will-submit column true, got %q", rows[0][5])
	}
}

func TestPublishExportStageRowsUsesPKGPath(t *testing.T) {
	_, rows := publishExportStageRows(&PublishExportStageResult{
		ArchivePath: "Demo.xcarchive",
		PKGPath:     "Demo.pkg",
	})
	foundPKG := false
	for _, row := range rows {
		if len(row) == 2 && row[0] == "pkg_path" && row[1] == "Demo.pkg" {
			foundPKG = true
		}
		if len(row) == 2 && row[0] == "ipa_path" {
			t.Fatalf("PKG export must not render an IPA row: %v", rows)
		}
	}
	if !foundPKG {
		t.Fatalf("expected pkg_path row, got %v", rows)
	}
}
