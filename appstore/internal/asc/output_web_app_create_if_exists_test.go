package asc

import (
	"reflect"
	"testing"
)

func TestWebAppCreateIfExistsResultRows(t *testing.T) {
	t.Parallel()

	headers, rows := webAppCreateIfExistsResultRows(&WebAppCreateIfExistsResult{
		ID:       "6759231657",
		Name:     "ASC  Test",
		BundleID: "com.example.app",
		SKU:      "SKU123",
		IdempotentWriteReceipt: IdempotentWriteReceipt{
			AlreadyExists: true,
			Action:        IdempotentWriteActionSkipped,
		},
	})

	wantHeaders := []string{"ID", "Name", "Bundle ID", "SKU", "Already Exists", "Action"}
	if !reflect.DeepEqual(headers, wantHeaders) {
		t.Fatalf("headers = %v, want %v", headers, wantHeaders)
	}
	wantRows := [][]string{{"6759231657", "ASC Test", "com.example.app", "SKU123", "true", IdempotentWriteActionSkipped}}
	if !reflect.DeepEqual(rows, wantRows) {
		t.Fatalf("rows = %v, want %v", rows, wantRows)
	}
}
