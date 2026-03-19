package apps

import (
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

func TestParseContentRightsValueAcceptsFriendlyVariants(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  asc.ContentRightsDeclaration
	}{
		{name: "false", input: "false", want: asc.ContentRightsDeclarationDoesNotUseThirdPartyContent},
		{name: "no", input: "no", want: asc.ContentRightsDeclarationDoesNotUseThirdPartyContent},
		{name: "does-not-use", input: "does-not-use", want: asc.ContentRightsDeclarationDoesNotUseThirdPartyContent},
		{name: "raw does not use", input: "DOES_NOT_USE_THIRD_PARTY_CONTENT", want: asc.ContentRightsDeclarationDoesNotUseThirdPartyContent},
		{name: "true", input: "true", want: asc.ContentRightsDeclarationUsesThirdPartyContent},
		{name: "yes", input: "yes", want: asc.ContentRightsDeclarationUsesThirdPartyContent},
		{name: "uses", input: "uses", want: asc.ContentRightsDeclarationUsesThirdPartyContent},
		{name: "raw uses", input: "USES_THIRD_PARTY_CONTENT", want: asc.ContentRightsDeclarationUsesThirdPartyContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseContentRightsValue(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestParseContentRightsValueRejectsUnknownValue(t *testing.T) {
	_, err := parseContentRightsValue("maybe")
	if err == nil {
		t.Fatal("expected invalid value error")
	}
	if !strings.Contains(err.Error(), "invalid value") {
		t.Fatalf("expected invalid value message, got %v", err)
	}
}

func TestContentRightsRowsShowUnsetWhenDeclarationMissing(t *testing.T) {
	rows := contentRightsRows(contentRightsResult{AppID: "app-1"})
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[1][0] != "content_rights_declaration" {
		t.Fatalf("expected declaration row label, got %q", rows[1][0])
	}
	if rows[1][1] != "unset" {
		t.Fatalf("expected unset declaration, got %q", rows[1][1])
	}
}
