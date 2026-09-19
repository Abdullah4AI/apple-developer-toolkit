package asc

import (
	"encoding/json"
	"testing"
)

func TestBuildBetaGroupsUpdateResultDryRunOutputIsAdditive(t *testing.T) {
	normal, err := json.Marshal(&BuildBetaGroupsUpdateResult{
		BuildID:  "build-1",
		GroupIDs: []string{"group-1"},
		Action:   "added",
	})
	if err != nil {
		t.Fatalf("marshal normal result: %v", err)
	}
	if got, want := string(normal), `{"buildId":"build-1","groupIds":["group-1"],"action":"added"}`; got != want {
		t.Fatalf("normal result JSON = %s, want %s", got, want)
	}

	dryRun, err := json.Marshal(&BuildBetaGroupsUpdateResult{
		BuildID:  "build-1",
		GroupIDs: []string{"group-1"},
		Action:   "would-add",
		DryRun:   true,
	})
	if err != nil {
		t.Fatalf("marshal dry-run result: %v", err)
	}
	if got, want := string(dryRun), `{"buildId":"build-1","groupIds":["group-1"],"action":"would-add","dryRun":true}`; got != want {
		t.Fatalf("dry-run result JSON = %s, want %s", got, want)
	}
}

func TestExtractPreReleaseVersionMap(t *testing.T) {
	included := json.RawMessage(`[
		{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.3","platform":"IOS"}},
		{"type":"preReleaseVersions","id":"prv-2","attributes":{"version":"1.2.3","platform":"TV_OS"}},
		{"type":"otherType","id":"other-1","attributes":{}}
	]`)

	m := extractPreReleaseVersionMap(included)
	if len(m) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(m))
	}
	if m["prv-1"].Version != "1.2.3" || m["prv-1"].Platform != "IOS" {
		t.Fatalf("unexpected prv-1: %+v", m["prv-1"])
	}
	if m["prv-2"].Platform != "TV_OS" {
		t.Fatalf("unexpected prv-2 platform: %s", m["prv-2"].Platform)
	}
}

func TestExtractPreReleaseVersionMapEmpty(t *testing.T) {
	m := extractPreReleaseVersionMap(nil)
	if m != nil {
		t.Fatalf("expected nil for empty included, got %v", m)
	}
}

func TestBuildPreReleaseVersionID(t *testing.T) {
	rels := json.RawMessage(`{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"prv-1"}}}`)
	id := buildPreReleaseVersionID(rels)
	if id != "prv-1" {
		t.Fatalf("expected prv-1, got %q", id)
	}
}

func TestBuildPreReleaseVersionIDEmpty(t *testing.T) {
	id := buildPreReleaseVersionID(nil)
	if id != "" {
		t.Fatalf("expected empty string, got %q", id)
	}
}

func TestBuildsRowsWithPreReleaseVersion(t *testing.T) {
	resp := &BuildsResponse{
		Data: []Resource[BuildAttributes]{
			{
				Type:          "builds",
				ID:            "build-1",
				Attributes:    BuildAttributes{Version: "9", UploadedDate: "2026-03-13", ProcessingState: "VALID"},
				Relationships: json.RawMessage(`{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"prv-1"}}}`),
			},
		},
		Included: json.RawMessage(`[{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.3","platform":"TV_OS"}}]`),
	}

	headers, rows := buildsRows(resp)
	if len(headers) != 8 {
		t.Fatalf("expected 8 headers (with Version+Platform), got %d: %v", len(headers), headers)
	}
	if headers[1] != "Build" || headers[2] != "Version" || headers[3] != "Platform" {
		t.Fatalf("unexpected headers: %v", headers)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	row := rows[0]
	if row[1] != "9" {
		t.Fatalf("expected build number 9, got %q", row[1])
	}
	if row[2] != "1.2.3" {
		t.Fatalf("expected marketing version 1.2.3, got %q", row[2])
	}
	if row[3] != "TV_OS" {
		t.Fatalf("expected platform TV_OS, got %q", row[3])
	}
}

// TestDSYMDownloadResultJSONByteCompat pins the exported DSYMDownloadResult
// JSON to the exact bytes previously produced by the struct declared in
// internal/cli/builds/builds_dsyms.go, proving the move to internal/asc
// changed code organization only, not output.
func TestDSYMDownloadResultJSONByteCompat(t *testing.T) {
	t.Run("all fields", func(t *testing.T) {
		result := &DSYMDownloadResult{
			BuildID:     "build-1",
			Version:     "1.2.3",
			BuildNumber: "42",
			Dir:         "./dsyms",
			Files: []DSYMDownloadFile{
				{
					BundleID: "com.example.app",
					FileName: "com.example.app-1.2.3-42.dSYM.zip",
					FilePath: "dsyms/com.example.app-1.2.3-42.dSYM.zip",
					FileSize: 1024,
				},
			},
		}
		got, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("marshal dSYM result: %v", err)
		}
		// Fixture: byte-for-byte JSON emitted by the pre-move struct.
		want := `{"buildId":"build-1","version":"1.2.3","buildNumber":"42","dir":"./dsyms",` +
			`"files":[{"bundleId":"com.example.app","fileName":"com.example.app-1.2.3-42.dSYM.zip",` +
			`"filePath":"dsyms/com.example.app-1.2.3-42.dSYM.zip","fileSize":1024}]}`
		if string(got) != want {
			t.Fatalf("dSYM result JSON drifted:\n got: %s\nwant: %s", got, want)
		}
	})

	t.Run("omitempty fields stay absent and files stay non-null", func(t *testing.T) {
		result := &DSYMDownloadResult{
			BuildID: "build-1",
			Dir:     ".",
			Files:   []DSYMDownloadFile{},
		}
		got, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("marshal dSYM result: %v", err)
		}
		want := `{"buildId":"build-1","dir":".","files":[]}`
		if string(got) != want {
			t.Fatalf("dSYM result JSON drifted:\n got: %s\nwant: %s", got, want)
		}
	})
}

func TestDSYMDownloadResultRows(t *testing.T) {
	result := &DSYMDownloadResult{
		BuildID: "build-1",
		Dir:     "./dsyms",
		Files: []DSYMDownloadFile{
			{BundleID: "com.example.app", FileName: "a.dSYM.zip", FilePath: "./dsyms/a.dSYM.zip", FileSize: 1024},
			{FileName: "b.dSYM.zip", FilePath: "./dsyms/b.dSYM.zip", FileSize: 2048},
		},
	}

	headers, rows := dsymDownloadResultRows(result)
	wantHeaders := []string{"Build ID", "Bundle ID", "File Name", "File Size", "Dir"}
	if len(headers) != len(wantHeaders) {
		t.Fatalf("unexpected headers: %v", headers)
	}
	for i, want := range wantHeaders {
		if headers[i] != want {
			t.Fatalf("headers[%d] = %q, want %q", i, headers[i], want)
		}
	}
	if len(rows) != 2 {
		t.Fatalf("expected one row per file, got %d", len(rows))
	}
	if rows[0][0] != "build-1" || rows[0][1] != "com.example.app" || rows[0][3] != "1024" {
		t.Fatalf("unexpected first row: %v", rows[0])
	}
	if rows[1][1] != "" || rows[1][2] != "b.dSYM.zip" || rows[1][4] != "./dsyms" {
		t.Fatalf("unexpected second row: %v", rows[1])
	}
}

func TestBuildWaitResultRows(t *testing.T) {
	result := &BuildWaitResult{
		BuildID:         "build-1",
		Version:         "1.2.3",
		BuildNumber:     "42",
		ProcessingState: "VALID",
		Elapsed:         "1m30s",
	}

	headers, rows := buildWaitResultRows(result)
	if len(headers) != 5 {
		t.Fatalf("unexpected headers: %v", headers)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	want := []string{"build-1", "1.2.3", "42", "VALID", "1m30s"}
	for i, cell := range want {
		if rows[0][i] != cell {
			t.Fatalf("rows[0][%d] = %q, want %q", i, rows[0][i], cell)
		}
	}
}

func TestBuildsRowsWithoutPreReleaseVersion(t *testing.T) {
	resp := &BuildsResponse{
		Data: []Resource[BuildAttributes]{
			{
				Type:       "builds",
				ID:         "build-1",
				Attributes: BuildAttributes{Version: "9", UploadedDate: "2026-03-13", ProcessingState: "VALID"},
			},
		},
	}

	headers, rows := buildsRows(resp)
	if len(headers) != 6 {
		t.Fatalf("expected 6 headers (backward compat), got %d: %v", len(headers), headers)
	}
	if headers[1] != "Version" {
		t.Fatalf("expected Version header at index 1, got %q", headers[1])
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0][1] != "9" {
		t.Fatalf("expected version 9, got %q", rows[0][1])
	}
}
