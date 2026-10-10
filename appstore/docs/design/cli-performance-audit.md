# CLI performance audit — 2026-10-07

The audit used ten independent investigation lanes and implemented four measured improvements on `c7e9c6466698de6736f1037e042932d1185d65f3`. That commit was refreshed from `origin/main`; the original local checkout was stale and was preserved. All changes are in an isolated worktree.

The investigation covered HTTP transport/authentication, pagination, metadata, TestFlight, assets, startup/search/output, commerce, analytics/Xcode Cloud, shared/review/signing workflows, and current Go guidance. Source findings were rechecked against the refreshed baseline before implementation. Edits and validation were serialized.

## Implemented changes

| Path | Change | Compatibility and resource bounds |
| --- | --- | --- |
| [TestFlight config export](../../internal/cli/testflight/testflight_sync.go) | Fetch independent groups in waves of four. | Cursor chains remain sequential; builds finish before testers start. Merge precedence and first input-order error are preserved. Failure cancels and joins peers. |
| [Metadata collection reads](../../internal/cli/metadata/localization_fetch.go) | Overlap app-info and version localization reads in pull and push/plan preparation. | At most two reads; unmanaged scopes remain unfetched. App-info errors retain precedence and cancel the sibling request. Existing per-page deadlines/retries and mutation sequencing remain intact. |
| [Analytics comparison](../../internal/cli/analytics/analytics_compare.go) | Download and parse up to four report dates concurrently. | Reduce in original date order, preserving floating-point totals, coverage, missing-date diagnostics, and first chronological error. Baseline/comparison periods remain sequential. Cancel and join workers and close response bodies before returning. |
| [Search matching](../../internal/cli/search/search.go) and [suggestions](../../internal/cli/shared/suggest/suggest.go) | Reject impossible weak-match lengths before edit distance; use three fixed token-form slots and `strings.SplitSeq`. | Preserve prefix/component exceptions, exact distances, ranking, normalization order, plurals, Unicode treatment, and empty-component behavior. No cache or dependency is added. |

Public flags, help, output formats, endpoint parameters, and the Go requirement are unchanged. Exact legacy-algorithm differential tests characterize the search optimization; barrier tests establish concurrent behavior without asserting fragile timing thresholds.

## Measurements

Measurements used Go 1.26.6 on darwin/arm64 (Apple M5), with `ASC_BYPASS_KEYCHAIN=1 GOMAXPROCS=2`. These are local characterizations, not live Apple timings or whole-CLI speed guarantees.

| Benchmark | Fixture | Before median | After median | Elapsed-time reduction |
| --- | --- | ---: | ---: | ---: |
| `BenchmarkPullTestFlightConfigLatency` | Eight groups, builds and testers, 2 ms delay per fake read | 37.19 ms | 9.49 ms | 74.5% |
| `BenchmarkMetadataLocalizationCollections` | Forty localizations, two collections, 25 ms transport delay per read | 52.23 ms | 27.11 ms | 48.1% |
| `BenchmarkFetchAndAggregateHTTP` | Sixteen gzip reports from a local HTTP server, 20 ms delay per response | 347.31 ms | 85.88 ms | 75.3% |

Each latency comparison used three samples with the same fixtures before and after. Analytics allocated approximately 1.07 MB before and 1.13–1.15 MB after per operation, an increase of about 6–8%. Concurrent parsing also permits up to four report working sets in flight. Allocated-byte totals do not measure peak resident memory.

Search benchmarks include full-registry document collection and ranking; command catalog construction is outside the timed loop. Final comparisons alternated immutable baseline/final binaries for ten samples, analyzed with `benchstat`.

| Search query | Allocated bytes before → after | Allocation count reduction | Median time before → after |
| --- | ---: | ---: | ---: |
| `upload build` | 96.95 → 67.58 MiB (−30.3%) | 70.6% | 246.0 → 186.8 ms |
| `external testers` | 304.0 → 130.6 MiB (−57.0%) | 83.8% | 758.3 → 441.7 ms |
| `cert profiles` | 192.84 → 75.80 MiB (−60.7%) | 86.1% | 407.7 → 251.5 ms |

Search allocation results were stable, while background host activity produced 32–73% timing dispersion. The cumulative runtime comparisons had p-values of .043, .002, and <.001 respectively, but allocation reduction is the more dependable result. Earlier non-interleaved and fast-rejection-only timing comparisons were inconclusive and are not used to claim a speedup.

An allocation profile justified expanding the search change beyond edit distance: `searchTokenForms` accounted for 40.49% of cumulative allocation space, and `strings.genSplit` for 9.90% flat allocation space after the initial fast rejection. Their percentages overlap with caller totals and must not be summed indiscriminately.

Reproduction commands, from the repository root:

```sh
ASC_BYPASS_KEYCHAIN=1 GOMAXPROCS=2 go test -p=1 ./internal/cli/testflight -run '^$' -bench '^BenchmarkPullTestFlightConfigLatency$' -benchtime=10x -count=3
ASC_BYPASS_KEYCHAIN=1 GOMAXPROCS=2 go test -p=1 ./internal/cli/metadata -run '^$' -bench '^BenchmarkMetadataLocalizationCollections$' -benchtime=10x -count=3
ASC_BYPASS_KEYCHAIN=1 GOMAXPROCS=2 go test -p=1 ./internal/cli/analytics -run '^$' -bench '^BenchmarkFetchAndAggregateHTTP$' -benchtime=3x -count=3 -benchmem
ASC_BYPASS_KEYCHAIN=1 GOMAXPROCS=2 go test -p=1 ./internal/cli/registry -run '^$' -bench '^BenchmarkSearchCommandsPerformance$' -benchtime=150ms -count=10 -benchmem
```

For before/after comparison, run identical benchmark sources against both implementations, preferably alternating compiled binaries. Raw samples, profiles, baseline binaries, and validation logs from this audit are retained locally in ignored `build/performance/`; they are not repository deliverables.

## Validation boundaries

Focused tests and adjacent command tests passed for all changed paths. Race checks passed for TestFlight, metadata and its command tests, and analytics/insights/affected command tests. Coverage includes bounded overlap, deterministic ordering, pagination, missing reports, unmanaged scopes, preview prerequisites, peer cancellation, blocked response-body reads, and body closure. Search has exact differential coverage plus existing ranking tests.

Repository-wide `make build`, `make format`, `make check-docs`, `make lint`, and `ASC_BYPASS_KEYCHAIN=1 make test` passed. The mandatory Codex review is recorded separately in the local validation logs. No live Apple measurement, release, deployment, or customer acceptance is established by these fixtures.

## Remaining candidates

These are source-confirmed opportunities, not measured wins or implemented changes:

| Area | Candidate and constraint |
| --- | --- |
| Subscription pricing | Fetch independent subscription-group inventories concurrently, then flatten in original order. Existing price-summary concurrency should remain bounded. |
| App/IAP pricing | Overlap independent schedule reads. Avoid multiplying the existing outer IAP worker limit through nested workers. |
| Review history/status | Parallelize independent submission item chains or independent detail/submission reads, preserving error attribution and 404 behavior. |
| Signing | Exhaustive certificate/profile reads omit supported `limit=200` on their initial requests. Verify provider page-size savings; preserve complete inventory before cleanup. |
| Asset downloads | Screenshot/preview downloads remain serial. Parallel writes need destination-collision checks, deterministic receipts, and a memory budget for PNG equivalence checks. |
| Asset upload transport | Reuse command-scoped upload transports; consider chunk concurrency only under a total request budget shared with existing screenshot workers. |
| Rooted file operations | Some screenshot helpers rely on GC cleanup of root directory descriptors. Explicit scoped closes merit a descriptor-pressure reproduction. |
| Apple Ads pagination | Change-history aggregation repeatedly processes growing nested data. Benchmark before refactoring its unknown-field and positional-fallback semantics. |
| Config/startup | Missing-config results are not cached; key validation and loading also duplicate parsing. Measure these smaller costs before changing cache lifetime or key-error contracts. |
| Reports | Overlapping compare date ranges can reuse fetched metrics. Large sales parsers also merit streaming work, preserving SKU discovery when IAP rows precede their parent app row. |
| Telemetry | Foreground spool maintenance repeats encoding and small writes. Preserve locking, atomic replacement, limits, recovery, and durable sync when measuring alternatives. |

## Current Go and reference-client findings

The ASC client already clones the standard transport, configures 128 total idle connections and 32 per host, retains HTTP/2, caches JWTs, and applies cancellation-aware retry behavior. Reusing clients and transports follows [Go's HTTP guidance](https://pkg.go.dev/net/http) and [AWS SDK guidance](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-http.html). Increasing pool limits or replacing `net/http` was not supported by the audit evidence.

[GitHub CLI's API implementation](https://github.com/cli/cli/blob/trunk/pkg/cmd/api/api.go) is a useful reference for shared clients and response-driven pagination. ASC cursor URLs depend on the preceding response, so this audit parallelizes independent collections instead of guessing later pages. [go-github](https://github.com/google/go-github/blob/master/github/github.go) is another useful reference for shared rate-limit state; its provider-specific policy should not be copied blindly.

[Go 1.27](https://go.dev/doc/go1.27) offers allocator and JSON implementation improvements and generally available goroutine-leak profiling. A separate toolchain comparison should retain existing JSON imports and check error text, compressed-output fixtures, packaging, and its macOS 13 minimum before adoption. No runtime-wide benefit is inferred from release-note percentages.

Further measurements should follow [Go diagnostics guidance](https://go.dev/doc/diagnostics), [HTTP connection tracing](https://pkg.go.dev/net/http/httptrace), and [benchstat's repeated-comparison guidance](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat). Representative profiling should precede any [PGO experiment](https://go.dev/doc/pgo).
