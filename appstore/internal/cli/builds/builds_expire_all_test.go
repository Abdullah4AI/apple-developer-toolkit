package builds

import (
	"testing"
	"time"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

func TestParseOlderThanDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{name: "days", input: "90d", want: 90 * 24 * time.Hour},
		{name: "weeks", input: "2w", want: 14 * 24 * time.Hour},
		{name: "months", input: "3m", want: 90 * 24 * time.Hour},
		{name: "uppercase unit", input: "10D", want: 10 * 24 * time.Hour},
		{name: "empty", input: "", wantErr: true},
		{name: "missing unit", input: "10", wantErr: true},
		{name: "zero", input: "0d", wantErr: true},
		{name: "bad unit", input: "10y", wantErr: true},
		{name: "bad number", input: "xd", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseOlderThanDuration(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != test.want {
				t.Fatalf("expected %v, got %v", test.want, got)
			}
		})
	}
}

func TestParseOlderThanThreshold(t *testing.T) {
	now := time.Date(2026, time.February, 10, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{
			name:  "date only",
			input: "2026-01-01",
			want:  time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:  "rfc3339",
			input: "2026-01-01T08:30:00Z",
			want:  time.Date(2026, time.January, 1, 8, 30, 0, 0, time.UTC),
		},
		{
			name:  "duration",
			input: "7d",
			want:  now.Add(-(7 * 24 * time.Hour)),
		},
		{
			name:    "invalid",
			input:   "not-a-threshold",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseOlderThanThreshold(test.input, now)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(test.want) {
				t.Fatalf("expected %s, got %s", test.want, got)
			}
		})
	}
}

func TestBuildExpireAllItem(t *testing.T) {
	item := buildExpireAllItem(buildExpireCandidate{
		resource: asc.Resource[asc.BuildAttributes]{
			ID: "build-1",
			Attributes: asc.BuildAttributes{
				Version:      "1.2.3",
				UploadedDate: "2026-01-01T00:00:00Z",
			},
		},
		ageDays: 40,
	})

	if item.ID != "build-1" || item.Version != "1.2.3" || item.AgeDays != 40 {
		t.Fatalf("unexpected expire-all item: %+v", item)
	}
}
