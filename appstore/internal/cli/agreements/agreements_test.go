package agreements

import (
	"testing"
)

func TestExtractEULATerritoryIDFromNextURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantID  string
		wantErr bool
	}{
		{
			name:   "resource path",
			input:  "https://api.appstoreconnect.apple.com/v1/endUserLicenseAgreements/123/territories?limit=50",
			wantID: "123",
		},
		{
			name:   "relationship path",
			input:  "https://api.appstoreconnect.apple.com/v1/endUserLicenseAgreements/abc/relationships/territories?limit=50",
			wantID: "abc",
		},
		{
			name:    "invalid path",
			input:   "https://api.appstoreconnect.apple.com/v1/apps",
			wantErr: true,
		},
		{
			name:    "malformed host localhost double port",
			input:   "http://localhost:80:80/v1/endUserLicenseAgreements/123/territories?limit=50",
			wantErr: true,
		},
		{
			name:    "malformed host ipv6 without brackets",
			input:   "http://::1/v1/endUserLicenseAgreements/123/territories?limit=50",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractEULATerritoryIDFromNextURL(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got id=%q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantID {
				t.Fatalf("got id=%q, want %q", got, tc.wantID)
			}
		})
	}
}
