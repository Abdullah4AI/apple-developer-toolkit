# Build, test, and request performance follow-up

This follow-up starts from `c7e9c6466698de6736f1037e042932d1185d65f3`, independently of the earlier HTTP/search performance branch. It preserves public commands, output contracts, test coverage, and the Go 1.26 platform baseline.

## Changes

- Signing configuration collection indexes normalized protected paths and previously observed file identities instead of repeatedly scanning every prior entry. Ordered output slices remain. Unicode folding only selects candidate identities; the existing filesystem case checks, `os.SameFile`, authorization order, replacement detection, and include-graph limits still decide behavior.
- Five exhaustive signing inventory readers request the supported maximum of 200 records per page. Each continuation URL remains untouched. The exact OpenAPI operations are `GET /v1/certificates`, `GET /v1/bundleIds/{id}/profiles`, and `GET /v1/profiles/{id}/certificates`.
- Subscription pricing enumerates independent groups in waves of four, with sequential pagination within each group. Ordered results and errors are preserved; failure cancels and joins peers. Enumeration completes before the existing four-worker pricing phase, avoiding nested request fan-out.
- `make test` and `make test-short` explicitly use `-count=1`. The isolated configuration directory, keychain bypass, all tests, vet behavior, and Go compilation cache remain. Test-result caching and its filesystem-access recording are disabled for these full validation commands. Direct `go test` remains available for cacheable focused development runs.
- The pre-commit hook uses the existing non-writing `make format-check`. Previously it ran the formatter and then rejected any unstaged change, including unrelated work that formatting had not touched. Formatting failures still block lint and tests. This remains a whole-working-tree formatting gate: unrelated non-Go edits and already-formatted Go edits no longer cause a false rejection, but any unformatted Go source still blocks the commit without being rewritten. It is not a staged-snapshot validator.
- The Go requirement moves from 1.26.6 to 1.26.8. Installed CI tool-cache keys now include architecture and `go.mod`, so a compiler-pin change rebuilds tools. No dependency versions or compiler optimization flags change.

## Measurements

Measurements used an Apple M5, darwin/arm64, Go 1.26.8, and `ASC_BYPASS_KEYCHAIN=1 GOMAXPROCS=2`. These are local measurements, not live Apple latency or published-release results.

Ten alternating samples from immutable baseline/final test binaries isolate the Xcode production changes from test-cache logging. Both binaries use the same compiler and fixtures; filesystem setup is outside the timed loop, and each iteration receives a fresh source budget.

| Operation | Before median | After median | Change |
| --- | ---: | ---: | ---: |
| Identity-aware collector, 128 files | 2.585 ms | 2.234 ms | 13.6% faster |
| Identity-aware collector, 512 files | 15.71 ms | 10.18 ms | 35.2% faster |
| Complete signing collection, 128 files | 104.8 ms | 102.1 ms | 2.6% faster |
| Complete signing collection, 512 files | 463.6 ms | 421.7 ms | 9.0% faster |

The 512-file comparisons had `p < .001`; the smaller complete-signing comparison had `p = .023`. Standalone collector allocation increased about 9% because identities are indexed. Complete signing collection allocated approximately 0.4% more bytes and 0.14% more objects. The existing 4,096-file tests remain unchanged; single before/after test durations are not used as statistical speed claims.

The subscription fixture contains 12 groups, two pages per group, and 10 ms injected latency per HTTP read. A three-iteration benchmark characterization improved from 277.718 to 70.065 ms/op, approximately 3.96 times faster. This is a controlled latency result, not a whole-command or provider speed guarantee.

The signing HTTP regression serves 201 records with a fixture default page size of 50. All five readers went from five requests to two while returning the same complete inventory, filtered results, or latest-expired selection. Apple's current default page size was not measured.

## Test-cache investigation

An actual cmdtest run produced a 149.3 MB snapshot containing 3.22 million recorded events: 2,343,304 `stat`, 530,841 `open`, and 350,070 environment reads. Approximately 99.1%, 97.9%, and 99.97% respectively repeated an already recorded input. Go's test-result cache replays these events, including path-containment checks for external temporary fixtures. Fresh per-run configuration paths also prevent reuse for configuration-dependent packages.

A separate warmed-compilation fixture performed 50,000 repeated stat/open/close operations and read a different missing configuration path on every run. Three alternating samples measured median whole-command times of 4.475 seconds with result caching versus 0.813 seconds with `-count=1`. Every sample executed and passed the same test. This demonstrates logging/replay overhead; it is not a claim that the entire suite improves by the same factor. Some other packages can reuse cached results, so disabling result caching has a tradeoff for repeated full gates.

The implementation follows [Go's test-cache semantics](https://pkg.go.dev/cmd/go#hdr-Testing_flags): disabling result reuse does not disable compilation caching. Existing CI shards already request fresh execution.

## Compatibility and validation

Go 1.26.8 contains compiler/runtime/library fixes; no particular CLI speedup is attributed to that patch upgrade. Go 1.26 retains the macOS 12 baseline, while Go 1.27 requires macOS 13. Adopting 1.27 is a separate platform-support decision. Both local cgo-enabled binaries (before and after this patch upgrade) declare macOS 27.0 in their Mach-O build metadata with this host's SDK. The toolchain support floor therefore does not prove the deployment target of a built artifact; release packaging and older-OS execution need separate verification. See the [official patch history](https://go.dev/doc/devel/release) and [Darwin compatibility notice](https://go.dev/doc/go1.26#darwin).

Focused validation covers the existing 4,096-file limits and unchanged-artifact checks, Unicode/case-sensitive aliases, hard links and replacement identities, subscription ordering/errors/cancellation, complete signing pagination, and hook failure propagation. Relevant race checks pass. One direct-test-binary run used a relative executable path and failed after a child changed directory; the exact case passed when invoked by absolute path. No production fix was needed for that invocation error.

The real disposable-repository hook reproduction passed with unrelated unstaged work and rejected malformed formatting without rewriting it. The permanent Python regression uses the existing isolated hook harness so documentation-only jobs do not acquire a Go formatter dependency.

Repository-wide validation and its exact input hashes are recorded in local `build/performance/` logs. Live Apple behavior, hosted follow-up CI, release artifacts, and macOS 12 runtime execution are not established by local tests.

Reproduction commands:

```sh
ASC_BYPASS_KEYCHAIN=1 GOMAXPROCS=2 go test -p=1 ./internal/xcode -run '^$' -bench 'Benchmark(XCConfigIdentityCollector|SigningXCConfigConsumers)$' -benchtime=150ms -count=10 -benchmem
ASC_BYPASS_KEYCHAIN=1 GOMAXPROCS=2 go test -p=1 ./internal/cli/subscriptions -run '^$' -bench '^BenchmarkFetchSubscriptionPricingGroupsLatency$' -benchtime=3x -count=1
ASC_BYPASS_KEYCHAIN=1 GOMAXPROCS=2 go test -p=1 ./internal/cli/signing -run '^TestSigningInventoryPageSizeReducesRequests$' -count=1
PYTHONPATH=scripts python3 -m unittest test_check_docs.HookChecksTest
```

For comparisons, compile the same benchmark sources against both production implementations and alternate binaries. Raw samples, the cache experiment, and saved binaries are local ignored evidence. Broader transport-session reuse, asset-download concurrency, and CI cache restructuring remain unmeasured candidates; they are not bundled into this change.
