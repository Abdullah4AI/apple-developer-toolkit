package shared

import "testing"

func TestSortResolvedPrices(t *testing.T) {
	rows := []ResolvedPriceRow{
		{Territory: "USA", StartDate: "2025-01-02", PriceID: "b", PricePointID: "pp-2"},
		{Territory: "GBR", StartDate: "2025-01-02", PriceID: "c", PricePointID: "pp-3"},
		{Territory: "USA", StartDate: "2025-01-01", PriceID: "a", PricePointID: "pp-1"},
	}

	SortResolvedPrices(rows)

	if rows[0].Territory != "GBR" {
		t.Fatalf("expected GBR first, got %+v", rows[0])
	}
	if rows[1].Territory != "USA" || rows[1].StartDate != "2025-01-01" {
		t.Fatalf("expected earliest USA row second, got %+v", rows[1])
	}
	if rows[2].Territory != "USA" || rows[2].StartDate != "2025-01-02" {
		t.Fatalf("expected latest USA row third, got %+v", rows[2])
	}
}
