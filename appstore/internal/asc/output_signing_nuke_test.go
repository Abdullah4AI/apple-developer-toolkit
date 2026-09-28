package asc

import (
	"reflect"
	"testing"
)

func TestSigningSyncNukeResultRendererRegisteredAndRenders(t *testing.T) {
	ensureOutputRegistryPopulated()
	handler := requireOutputHandlerFor[SigningSyncNukeResult](t, "SigningSyncNukeResult")
	result := &SigningSyncNukeResult{
		Operation:        "nuke",
		ProfileType:      "IOS_APP_DEVELOPMENT",
		CertificateTypes: []string{"IOS_DEVELOPMENT"},
		Profiles: SigningSyncNukeProfiles{
			Planned: []SigningSyncNukeResource{{ID: "profile-1", Name: "Dev", Type: "IOS_APP_DEVELOPMENT"}, {ID: "profile-2", Name: "Dev 2", Type: "IOS_APP_DEVELOPMENT"}},
			Deleted: []SigningSyncNukeResource{{ID: "profile-1", Name: "Dev", Type: "IOS_APP_DEVELOPMENT"}},
			Failed:  []SigningSyncNukeFailure{{ID: "profile-2", Name: "Dev 2", Error: "boom"}},
		},
		Certificates: SigningSyncNukeCertificates{
			Planned: []SigningSyncNukeResource{{ID: "cert-1", Type: "IOS_DEVELOPMENT", SerialNumber: "SERIAL"}},
			Revoked: []SigningSyncNukeResource{{ID: "cert-1", Type: "IOS_DEVELOPMENT", SerialNumber: "SERIAL"}},
		},
		RepositoryFiles: SigningSyncNukeFiles{
			Planned: []string{"certs/development/SERIAL.cer", "profiles/development/Dev 2.mobileprovision"},
			Removed: []string{"certs/development/SERIAL.cer"},
			Kept:    []string{"profiles/development/Dev 2.mobileprovision"},
		},
	}

	headers, rows, err := handler(result)
	if err != nil {
		t.Fatalf("nuke rows handler: %v", err)
	}
	if want := []string{"Kind", "ID", "Name", "Type", "Status"}; !reflect.DeepEqual(headers, want) {
		t.Fatalf("headers = %v, want %v", headers, want)
	}
	want := [][]string{
		{"profile", "profile-1", "Dev", "IOS_APP_DEVELOPMENT", "deleted"},
		{"profile", "profile-2", "Dev 2", "IOS_APP_DEVELOPMENT", "failed: boom"},
		{"certificate", "cert-1", "SERIAL", "IOS_DEVELOPMENT", "revoked"},
		{"file", "certs/development/SERIAL.cer", "", "", "removed"},
		{"file", "profiles/development/Dev 2.mobileprovision", "", "", "kept"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %v, want %v", rows, want)
	}

	result.DryRun = true
	_, rows, _ = handler(result)
	for _, row := range rows {
		if row[4] != "planned" {
			t.Fatalf("dry-run row status = %q, want planned: %v", row[4], row)
		}
	}

	for _, renderer := range []struct {
		name string
		fn   func(any) error
	}{
		{name: "table", fn: PrintTable},
		{name: "markdown", fn: PrintMarkdown},
	} {
		t.Run(renderer.name, func(t *testing.T) {
			assertRenderedNonJSONContains(t, renderer.fn, result, "profile-1", "cert-1", "planned")
		})
	}
}
