package asc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestPaginateEach_MultiPage(t *testing.T) {
	const totalPages = 3
	const perPage = 2

	firstPage := makeAppsPage(1, perPage, totalPages)
	var ids []string

	err := PaginateEach(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		page, err := parseMockPageNum(nextURL)
		if err != nil {
			return nil, err
		}
		return makeAppsPage(page, perPage, totalPages), nil
	}, func(page PaginatedResponse) error {
		apps, ok := page.(*AppsResponse)
		if !ok {
			return fmt.Errorf("unexpected page type %T", page)
		}
		for _, app := range apps.Data {
			ids = append(ids, app.ID)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("PaginateEach() error: %v", err)
	}

	expected := totalPages * perPage
	if len(ids) != expected {
		t.Fatalf("expected %d ids, got %d", expected, len(ids))
	}
}

func TestPaginateEach_ConsumerErrorIncludesPage(t *testing.T) {
	firstPage := makeAppsPage(1, 1, 2)
	consumerErr := errors.New("consumer failed")

	err := PaginateEach(context.Background(), firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		return makeAppsPage(2, 1, 2), nil
	}, func(page PaginatedResponse) error {
		apps := page.(*AppsResponse)
		if len(apps.Data) > 0 && apps.Data[0].ID == "app-2-0" {
			return consumerErr
		}
		return nil
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, consumerErr) {
		t.Fatalf("expected wrapped consumer error, got %v", err)
	}
	if got := err.Error(); !strings.Contains(got, "page 2") {
		t.Fatalf("expected page 2 context in error, got %q", got)
	}
}
