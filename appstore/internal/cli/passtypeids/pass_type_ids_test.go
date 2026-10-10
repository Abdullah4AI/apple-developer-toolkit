package passtypeids

import (
	"testing"
)

func TestPassTypeIDHelpers(t *testing.T) {
	if got, err := normalizePassTypeIDInclude("certificates"); err != nil || len(got) != 1 {
		t.Fatalf("expected valid include, got %v err=%v", got, err)
	}
	if _, err := normalizePassTypeIDInclude("bad"); err == nil {
		t.Fatal("expected include validation error")
	}

	if _, err := normalizePassTypeIDFields("invalid", "--fields"); err == nil {
		t.Fatal("expected fields validation error")
	}
}

func TestPassTypeIDFromCertificatesNextURL(t *testing.T) {
	tests := []struct {
		name         string
		next         string
		relationship bool
		want         string
	}{
		{
			name: "certificates",
			next: "https://api.appstoreconnect.apple.com/v1/passTypeIds/pass-1/certificates?cursor=AQ",
			want: "pass-1",
		},
		{
			name:         "certificate relationships",
			next:         "https://api.appstoreconnect.apple.com/v1/passTypeIds/pass-1/relationships/certificates?cursor=AQ",
			relationship: true,
			want:         "pass-1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := passTypeIDFromCertificatesNextURL(test.next, test.relationship)
			if err != nil {
				t.Fatalf("passTypeIDFromCertificatesNextURL() error: %v", err)
			}
			if got != test.want {
				t.Fatalf("passTypeIDFromCertificatesNextURL() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestPassTypeIDFromCertificatesNextURLRejectsWrongEndpoint(t *testing.T) {
	tests := []struct {
		name         string
		next         string
		relationship bool
	}{
		{name: "users endpoint", next: "https://api.appstoreconnect.apple.com/v1/users?cursor=AQ"},
		{name: "missing pass type ID", next: "https://api.appstoreconnect.apple.com/v1/passTypeIds//certificates?cursor=AQ"},
		{name: "relationship passed to list", next: "https://api.appstoreconnect.apple.com/v1/passTypeIds/pass-1/relationships/certificates?cursor=AQ"},
		{name: "list passed to relationship", next: "https://api.appstoreconnect.apple.com/v1/passTypeIds/pass-1/certificates?cursor=AQ", relationship: true},
		{name: "encoded slash in ID", next: "https://api.appstoreconnect.apple.com/v1/passTypeIds/pass%2Fone/certificates?cursor=AQ"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := passTypeIDFromCertificatesNextURL(test.next, test.relationship); err == nil {
				t.Fatalf("expected error for %q", test.next)
			}
		})
	}
}
