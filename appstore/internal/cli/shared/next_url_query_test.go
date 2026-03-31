package shared

import (
	"net/url"
	"testing"
)

func TestMergeNextURLQuery(t *testing.T) {
	nextURL := "https://api.appstoreconnect.apple.com/v1/appPriceSchedules/schedule-1/manualPrices?cursor=Mg"
	values := url.Values{
		"include":                []string{"appPricePoint,territory"},
		"fields[appPrices]":      []string{"manual,startDate,endDate,appPricePoint,territory"},
		"fields[appPricePoints]": []string{"customerPrice,proceeds,territory"},
		"fields[territories]":    []string{"currency"},
		"limit":                  []string{"200"},
	}

	merged, err := MergeNextURLQuery(nextURL, values)
	if err != nil {
		t.Fatalf("MergeNextURLQuery() error = %v", err)
	}

	parsed, err := url.Parse(merged)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	query := parsed.Query()
	if query.Get("cursor") != "Mg" {
		t.Fatalf("expected cursor=Mg, got %q", query.Get("cursor"))
	}
	if query.Get("include") != "appPricePoint,territory" {
		t.Fatalf("expected include query, got %q", query.Get("include"))
	}
	if query.Get("fields[appPrices]") != "manual,startDate,endDate,appPricePoint,territory" {
		t.Fatalf("unexpected fields[appPrices]: %q", query.Get("fields[appPrices]"))
	}
	if query.Get("limit") != "200" {
		t.Fatalf("expected limit=200, got %q", query.Get("limit"))
	}
}

func TestMergeNextURLQueryRejectsInvalidURL(t *testing.T) {
	if _, err := MergeNextURLQuery("http://api.appstoreconnect.apple.com/v1/apps?cursor=Mg", url.Values{"limit": []string{"200"}}); err == nil {
		t.Fatal("expected error, got nil")
	}
}
