package subscriptions

import "testing"

func TestParseSubscriptionIntroductoryOffersImportCSVHeader_StripsUTF8BOM(t *testing.T) {
	got, err := parseSubscriptionIntroductoryOffersImportCSVHeader([]string{"\ufeffterritory", "offer_mode"})
	if err != nil {
		t.Fatalf("parseSubscriptionIntroductoryOffersImportCSVHeader() error: %v", err)
	}
	if got["territory"] != 0 {
		t.Fatalf("expected territory column at index 0, got %d", got["territory"])
	}
	if got["offer_mode"] != 1 {
		t.Fatalf("expected offer_mode column at index 1, got %d", got["offer_mode"])
	}
}
