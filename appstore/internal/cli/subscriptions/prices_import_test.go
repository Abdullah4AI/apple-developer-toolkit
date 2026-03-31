package subscriptions

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

func TestReadSubscriptionPricesImportCSV_SupportsHeaderAliases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prices.csv")
	body := "" +
		"Countries or Regions,Currency Code,Price,start_date,preserved,ignored\n" +
		"USA,USD,19.99,2026-03-01,false,foo\n" +
		"Afghanistan,AFN,299.00,2026-03-01,true,bar\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	rows, err := readSubscriptionPricesImportCSV(path)
	if err != nil {
		t.Fatalf("readSubscriptionPricesImportCSV() error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].territory != "USA" || rows[0].currencyCode != "USD" || rows[0].price != "19.99" {
		t.Fatalf("unexpected row[0]: %+v", rows[0])
	}
	if !rows[1].preserveSet || !rows[1].preserveCurrentPrice {
		t.Fatalf("expected row[1] preserved=true, got %+v", rows[1])
	}
}

func TestReadSubscriptionPricesImportCSV_DuplicateKnownColumnReturnsUsageError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prices.csv")
	body := "" +
		"territory,Countries or Regions,price\n" +
		"USA,USA,19.99\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, err := readSubscriptionPricesImportCSV(path)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}
}

func TestReadSubscriptionPricesImportCSV_InvalidDateReturnsUsageError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prices.csv")
	body := "" +
		"territory,price,start_date\n" +
		"USA,19.99,2026-13-01\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, err := readSubscriptionPricesImportCSV(path)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}
}

func TestReadSubscriptionPricesImportCSV_InvalidBooleanReturnsUsageError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prices.csv")
	body := "" +
		"territory,price,preserve_current_price\n" +
		"USA,19.99,yes\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, err := readSubscriptionPricesImportCSV(path)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}
}

func TestResolveSubscriptionPriceImportTerritoryID_MapsCommonNames(t *testing.T) {
	got, err := resolveSubscriptionPriceImportTerritoryID("Afghanistan")
	if err != nil {
		t.Fatalf("resolveSubscriptionPriceImportTerritoryID() error: %v", err)
	}
	if got != "AFG" {
		t.Fatalf("expected AFG, got %q", got)
	}
}

func TestResolveSubscriptionPriceImportTerritoryID_RejectsUnknownThreeLetterCode(t *testing.T) {
	_, err := resolveSubscriptionPriceImportTerritoryID("ZZZ")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestResolveSubscriptionPriceImportTerritoryID_AcceptsSupportedThreeLetterCodeWithoutDisplayName(t *testing.T) {
	got, err := resolveSubscriptionPriceImportTerritoryID("ANT")
	if err != nil {
		t.Fatalf("resolveSubscriptionPriceImportTerritoryID() error: %v", err)
	}
	if got != "ANT" {
		t.Fatalf("expected ANT, got %q", got)
	}
}

func TestResolveSubscriptionPriceImportTerritoryID_RejectsTerritoriesOutsideASCSet(t *testing.T) {
	tests := []string{"ATA", "AQ", "Antarctica"}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := resolveSubscriptionPriceImportTerritoryID(input)
			if err == nil {
				t.Fatalf("expected error for %q, got nil", input)
			}
		})
	}
}
