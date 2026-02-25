package asc

import (
	"context"
	"testing"
)

func BenchmarkPaginateAllAggregation(b *testing.B) {
	const totalPages = 25
	const perPage = 200

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		first := makeAppsPage(1, perPage, totalPages)
		result, err := PaginateAll(context.Background(), first, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
			page, err := parseMockPageNum(nextURL)
			if err != nil {
				return nil, err
			}
			return makeAppsPage(page, perPage, totalPages), nil
		})
		if err != nil {
			b.Fatalf("PaginateAll() error: %v", err)
		}
		apps := result.(*AppsResponse)
		if len(apps.Data) != totalPages*perPage {
			b.Fatalf("items = %d, want %d", len(apps.Data), totalPages*perPage)
		}
	}
}

func BenchmarkPaginateEachStreaming(b *testing.B) {
	const totalPages = 25
	const perPage = 200

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		first := makeAppsPage(1, perPage, totalPages)
		total := 0
		err := PaginateEach(context.Background(), first, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
			page, err := parseMockPageNum(nextURL)
			if err != nil {
				return nil, err
			}
			return makeAppsPage(page, perPage, totalPages), nil
		}, func(page PaginatedResponse) error {
			apps := page.(*AppsResponse)
			total += len(apps.Data)
			return nil
		})
		if err != nil {
			b.Fatalf("PaginateEach() error: %v", err)
		}
		if total != totalPages*perPage {
			b.Fatalf("items = %d, want %d", total, totalPages*perPage)
		}
	}
}
