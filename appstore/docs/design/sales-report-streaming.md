# Stream scoped sales reports

Sales report aggregation previously retained every TSV row, even when the app SKU was already known. The known-SKU path now reads records sequentially with `csv.Reader.ReuseRecord`; both paths use one accumulator with the existing filters and arithmetic order. The accumulator retains column indexes and totals, not input records.

Unknown or whitespace-only SKU retains full-file enrichment so a later app row can identify earlier IAP rows. Both paths consume the entire gzip stream. A late CSV or checksum error returns zero metrics and still precedes a missing-header error. Commands, report parameters, metric availability, numeric parsing, and output shapes remain unchanged.

Characterization tests passed before and after the refactor: existing totals and column availability, late SKU discovery, earlier IAP rows, floating-point summation order, corrupt trailers with valid/missing/absent headers, and empty/header-only reports. Adjacent insights, analytics and command tests plus focused race checks passed. The reuse contract follows the [standard CSV reader documentation](https://pkg.go.dev/encoding/csv#Reader).

The fixture contains 100,000 IAP rows with one percent matching the selected SKU. Three samples of three iterations each, on Apple M5 with Go 1.26.6 and `ASC_BYPASS_KEYCHAIN=1 GOMAXPROCS=1 GOFLAGS=-p=4`, gave:

| Scope | Median before | Median after | Allocated bytes before | Allocated bytes after |
| --- | ---: | ---: | ---: | ---: |
| Known SKU | 17.071 ms | 10.377 ms | 29,025,488 | 3,255,394 |
| Unknown SKU | 18.305 ms | 18.730 ms | 29,025,488 | 29,025,488 |

Known-SKU allocations decrease 88.8%; the measurements are total allocated bytes, not peak resident memory. Unknown-SKU allocations are identical and its small timing variation is not a claimed improvement. These samples characterize local parsing, not Apple request latency.

Reproduce with `go test ./internal/cli/insights -run '^$' -bench '^BenchmarkParseSalesReportMetrics$' -benchtime=3x -count=3 -benchmem` using the environment above. Raw before/after and regression logs are retained under ignored `build/performance/sales-parser-*`. Full validation and final branch review are recorded in the pull request.
