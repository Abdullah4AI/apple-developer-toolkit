package validate

import (
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

func TestMapAgeRatingDeclarationIncludesSocialMediaFields(t *testing.T) {
	trueValue := true
	falseValue := false

	mapped := mapAgeRatingDeclaration("info-1", asc.AgeRatingDeclarationAttributes{
		SocialMedia:              &asc.NullableBool{Value: &trueValue},
		SocialMediaAgeRestricted: &asc.NullableBool{Value: &falseValue},
	})

	if mapped.SocialMedia == nil || !*mapped.SocialMedia {
		t.Fatalf("expected socialMedia=true, got %#v", mapped.SocialMedia)
	}
	if mapped.SocialMediaAgeRestricted == nil || *mapped.SocialMediaAgeRestricted {
		t.Fatalf("expected socialMediaAgeRestricted=false, got %#v", mapped.SocialMediaAgeRestricted)
	}
}
