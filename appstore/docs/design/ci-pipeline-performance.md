# CI compilation and documentation reuse

Audit cutoff: 2026-10-07 17:07:51 UTC. Baseline main is c7e9c6466698de6736f1037e042932d1185d65f3; this change builds on the Go 1.27 migration at 8125cbc58f85e1e7dd5c0e2a0b65ebc1b7509b1b.

## Evidence

Completed PR runs 37649487022, 37640024685, and 37628896681 spent 561, 710, and 730 seconds in quality checks. Docs, lint, platform vet, and workflow contract checks ran concurrently on each single runner. Those durations overlap and must not be added together. Logs show the quality jobs losing setup-go's shared cache save race to other jobs. A subsequent run restored an approximately 71 MB exact-key cache and declined to save the additional compiled outputs.

The release audit found separate costs: guardrails took 9:01–19:14 across six releases, before five target builds. Release 5.13.0 also waited 36:13 for two sequential Apple notarizations in its successful attempt. Runner queues were only 6–12 seconds. This CI change makes no claim to reduce provider processing time and does not change the trusted release-build cache policy.

## Change

A composite action sets up the exact go.mod toolchain and restores Go's module and compilation caches per operating system, architecture, resolved toolchain, dependency files, and workload. Source SHA rotates the save key, so successful work can add newly compiled outputs rather than preserving an incomplete dependency-only cache indefinitely. Restore prefixes remain within the compatible workload. PR and main use the same workload names; each test matrix entry has its own identity. Cache restoration never skips a validator or test. More cache entries and transfers are the tradeoff; hosted elapsed time and cache sizes must be measured.

Quality checks execute one after another so independent Go processes do not compile the same packages concurrently on one runner. The existing test shards and native platform checks are unchanged. The packages shard now sets ASC_BYPASS_KEYCHAIN for its complete shell step, including the command after &&.

The docs gate builds one temporary CLI and passes it to the command reference and website command validators. It always invokes Go's build driver against the current tree, including uncommitted changes. It does not reuse a previous executable by filename, timestamp, or commit metadata. Standalone validators retain their default build behavior. All original validators remain in the gate.

## Validation

Regression checks reject missing cache identity dimensions, a dependency-only save key, shared workload keys, concurrent quality compilation, or loss of the inherited test environment. Docs tests cover one build shared by both consumers, rebuild per invocation, build and validator failures, cleanup, and standalone behavior. Run the existing workflow contracts, actionlint, docs self-tests, and complete repository gates. Compare hosted cold and warm runs; source changes alone do not establish an elapsed-time improvement.

## References

- [GoReleaser's workload-specific test cache](https://github.com/goreleaser/goreleaser/blob/ede6721811494be236a2bba898ecf22a4b717ae5/.github/workflows/build.yml#L147-L158).
- [setup-go cache guidance for parallel jobs](https://github.com/actions/setup-go/blob/90ad2b35f69faf97585ad74d28fa006d2739b7af/docs/advanced-usage.md#parallel-builds).
- [GitHub CLI platform release jobs](https://github.com/cli/cli/blob/996be976cad960a726aabb94be67dfc7e31e317b/.github/workflows/deployment.yml#L50).
