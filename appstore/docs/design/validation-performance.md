# Validation performance follow-up

This change builds on PR #2951 at `f53e60ec57055ca83ed8f41135b9ff004bf0a1bd`. It changes test fixtures, documentation validation, formatting, and CI caching. Production CLI behavior, test selection, retry counts, lint rules, and required CI checks remain intact.

## Work removed

- CLI web and command tests select the existing 200 ms minimum request interval for mocked HTTP. Core web tests continue checking the one-second default, invalid-value fallback, clamping, and actual pacing. Per-test overrides remain effective.
- Four mock-client fixtures select a 1 ms retry base delay. They retain all retry attempts, maximum delays, and `Retry-After` behavior. Dedicated ASC and iTunes timing/deadline/default tests remain unchanged.
- Two signing-budget tests use shallow graphs with the same real source counts, wrappers, traversal order, and terminal settings. They still check rejection beyond the 4,096-source global limit and preservation of existing artifacts. A real 4,097-file deep collector, allocation-scaling tests, cycles, identity checks, and parser cases retain depth and parsing coverage. The in-memory source-limit fixture now uses one temporary directory with 4,097 distinct paths.
- The two version compatibility fixtures still exceed 8 MiB, using a long comment instead of hundreds of thousands of comment lines. A separate large multiline rollback fixture remains unchanged.
- Website validation shares raw help-process results between strict command discovery and alias probes. All command paths are still traversed, failed help remains an error, and each invocation clears its cache even on failure. Root generated-reference and example validation share one fresh root-help result.
- `make format` runs gofumpt once instead of preceding it with `go fmt ./...`. Byte-for-byte fixture comparisons covered ordinary, generated, platform-tagged, and nested-module sources. `format-check` retains its previous scope. [gofumpt documents its gofmt compatibility and directory exclusions](https://github.com/mvdan/gofumpt#readme).
- PR and main CI persist the actual `.golangci-cache` analysis directory. Cache compatibility includes OS, architecture, Go/dependency files, tool configuration, and linter configuration; each source SHA can save updated analysis. Restoring analysis never skips lint. This follows the [linter's cache support](https://golangci-lint.run/docs/configuration/cli/#cache); it does not replace the Go compilation cache.

## Local measurements

Apple M5, darwin/arm64, Go 1.26.8. Targeted comparisons used `ASC_BYPASS_KEYCHAIN=1 GOMAXPROCS=1` and bounded package compilation. No other agent Go gate ran alongside the retained timing samples.

| Check | Before | After | Scope |
| --- | ---: | ---: | --- |
| Full `make test` | 302.189 s | 228.149 s | Same four-CPU budget; complete suite |
| Four web cases | 20.25 s | 4.09 s | Sum of selected test execution times |
| Nine retry/error cases | 63.87 s | 0.10 s | Sum of selected test execution times |
| Five Xcode cases | 223.585 s | 107.019 s | Selected package execution, same command |
| Website command checks | 58.915 s | 48.981 s | Same retained binary; builds excluded |
| Root command docs | 1.173 s | 0.410 s | Warm check, two help invocations reduced to one |
| `make format` | 5.659 s | 3.168 s | Median of three alternating warm samples |

The full suite and first five targeted comparisons are single before/after characterizations, not statistical estimates. The full-suite pair saved 74.040 seconds (24.5%), with the same 112 passing packages, 28,750 passing test/subtest lines, and 22 skips. The Xcode SetVersion case alone did not improve in its sample; the aggregate did. Targeted test times cannot be added to infer full-suite wall-clock savings. Initial cold compilation and a contended formatter sample are excluded from speed claims.

Website traversal retains 2,003 command specifications and identical successful diagnostics. Help subprocesses drop from 2,825 to 2,035: exactly 790 duplicate executions removed.

## Verification and limits

Focused checks preserve core pacing and retry contracts, the real filesystem boundaries, deep-graph and parser behavior, strict help failures, alias diagnostics, generated-document drift, invocation-local cache lifetime, formatter output, and all workflow gates. Repository-wide results and exact input hashes are recorded in local ignored `build/performance/` evidence. Final `make format`, `make build`, `make check-docs`, `make lint`, `make test`, docs/workflow script tests, and focused race checks across seven affected packages all passed. The focused race command took 176.844 seconds including compilation; no before/after race speedup is claimed. Build took 8.186 seconds and complete docs validation took 45.560 seconds; these are final-run durations, not paired improvements.

With an initially absent local linter analysis cache and an already populated Go compilation cache, consecutive identical lint checks took 31.511 seconds and 2.652 seconds, both with zero issues. The first used four linter workers; the second reused its analysis. This characterizes local reuse, not network cache restore/save cost.

The previous PR's hosted quality job took 730 seconds, with overlapping docs, lint, and platform-vet steps of 649, 639, and 600 seconds. Those durations suggest contention but do not prove that changing job topology helps. This patch preserves scheduling. Hosted cache transfer and warm-run benefits require a separate same-head observation; local cache timings are not hosted CI guarantees. Compiler and race-detector behavior are unchanged.
