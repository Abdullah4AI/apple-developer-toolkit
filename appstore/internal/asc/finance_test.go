package asc

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"testing/synctest"
	"time"
)

func TestDownloadFinanceReportSurvivesShortClientTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := newTestClient(t, nil, rawResponse("unused"))
		client.httpClient = &http.Client{
			Timeout: 40 * time.Millisecond,
			Transport: streamingTransport(func(w http.ResponseWriter, req *http.Request) {
				if req.URL.Path != "/v1/financeReports" || req.Header.Get("Authorization") == "" {
					t.Errorf("unexpected finance request: %s, authorized=%v", req.URL.Path, req.Header.Get("Authorization") != "")
				}
				_, _ = io.WriteString(w, "gz")
				select {
				case <-time.After(150 * time.Millisecond):
					_, _ = io.WriteString(w, "data")
				case <-req.Context().Done():
				}
			}),
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		download, err := client.DownloadFinanceReport(ctx, FinanceReportParams{
			VendorNumber: "12345678",
			ReportType:   FinanceReportTypeFinancial,
			RegionCode:   "US",
			ReportDate:   "2025-12",
		})
		if err != nil {
			t.Fatalf("DownloadFinanceReport() error = %v", err)
		}
		defer download.Body.Close()
		body, err := io.ReadAll(download.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if client.httpClient.Timeout != 40*time.Millisecond {
			t.Fatal("streaming mutated the shared client timeout")
		}
		if string(body) != "gzdata" {
			t.Fatalf("body = %q", body)
		}
	})
}

func TestDownloadFinanceReport_SendsRequest(t *testing.T) {
	response := rawResponse("gzdata")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/financeReports" {
			t.Fatalf("expected path /v1/financeReports, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[vendorNumber]") != "12345678" {
			t.Fatalf("expected vendorNumber filter, got %q", values.Get("filter[vendorNumber]"))
		}
		if values.Get("filter[reportType]") != "FINANCIAL" {
			t.Fatalf("expected reportType filter, got %q", values.Get("filter[reportType]"))
		}
		if values.Get("filter[regionCode]") != "US" {
			t.Fatalf("expected regionCode filter, got %q", values.Get("filter[regionCode]"))
		}
		if values.Get("filter[reportDate]") != "2025-12" {
			t.Fatalf("expected reportDate filter, got %q", values.Get("filter[reportDate]"))
		}
		if req.Header.Get("Accept") != "application/a-gzip" {
			t.Fatalf("expected gzip Accept header, got %q", req.Header.Get("Accept"))
		}
		assertAuthorized(t, req)
	}, response)

	download, err := client.DownloadFinanceReport(context.Background(), FinanceReportParams{
		VendorNumber: "12345678",
		ReportType:   FinanceReportTypeFinancial,
		RegionCode:   "US",
		ReportDate:   "2025-12",
	})
	if err != nil {
		t.Fatalf("DownloadFinanceReport() error: %v", err)
	}
	_ = download.Body.Close()
}

func TestDownloadFinanceReport_ErrorResponse(t *testing.T) {
	response := jsonResponse(http.StatusForbidden, `{"errors":[{"code":"FORBIDDEN","title":"Forbidden","detail":"nope"}]}`)
	client := newTestClient(t, nil, response)
	_, err := client.DownloadFinanceReport(context.Background(), FinanceReportParams{
		VendorNumber: "12345678",
		ReportType:   FinanceReportTypeFinancial,
		RegionCode:   "US",
		ReportDate:   "2025-12",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}
