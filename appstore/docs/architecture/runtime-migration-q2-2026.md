# Incremental Rewrite Plan (Q2 2026)

Status: Draft  
Window: April 1, 2026 to June 30, 2026  
Primary owner: Rudrank Riyam  
Scope: `asc` CLI architecture, workflow-critical command families, shared runtime concerns, and selected `internal/asc` cleanup

## Why This Exists

This file is the source of truth for the Q2 2026 effort to rewrite the execution model of the CLI without doing a flag-day rewrite.

The problem is not that the repo is "bad" across the board. The problem is that a few cross-cutting layers have become overloaded while command surface area kept growing quickly:

- `79` root subcommands in [internal/cli/registry/registry.go](../../internal/cli/registry/registry.go)
- `1,383` tracked Go files
- `440,790` total Go lines
- `230,251` non-test Go lines
- `210,539` test Go lines
- [internal/cli/shared/shared.go](../../internal/cli/shared/shared.go) at `1,482` lines
- [internal/asc/client_options.go](../../internal/asc/client_options.go) at `3,590` lines
- high-churn workflow families in `status`, `release`, `submit`, `builds`, `testflight`, `metadata`, and `apps`

The architectural goal for Q2 is not "make every file pretty." The goal is to replace the foundation that future work keeps re-accumulating on: `shared`, `root`, registry wiring, and giant client option files.

## Rewrite Framing

Think about Q2 as if we were rebuilding `asc` from scratch, but keeping the existing user-facing command contract stable wherever practical.

That means:

- preserve the best parts of the current CLI contract
- replace the execution foundation underneath it
- prefer rebuilding command families on the new foundation over wrapping old internals forever
- treat migration scaffolding as demolition scaffolding, not architecture
- end the quarter with the new system as the default and the old system mostly gone

The question for each family is not "how do we move this code with the fewest edits?" The question is "if we were building this command family today, what foundation would we want it standing on?"

## Q2 Outcomes

By the end of Q2 2026, we want all of the following to be true:

1. Root/runtime concerns are carried through an explicit runtime object instead of process-wide shared state.
2. The monolithic `shared` package is retired as a real dependency owner, and any temporary bridge wrappers used during migration are removed before Q2 closes.
3. Every root command family is constructed against the new runtime by the end of the quarter.
4. Workflow-critical commands are re-implemented cleanly on the new foundation before lower-value long-tail cleanup absorbs the schedule.
5. Command taxonomy churn is constrained by rules instead of growing organically.
6. `internal/asc` is easier to navigate, especially option and query definitions.
7. The repo feels like a better version of `asc`, not old `asc` with a new helper layer taped onto it.

## Non-Goals

- Doing a single flag-day rewrite that breaks the repo for days
- Switching away from `ffcli`
- Replacing the entire `internal/asc` client in one quarter
- Deleting all deprecated paths immediately
- Chasing line-count reduction as the main success metric
- Redesigning user-facing command contracts just for aesthetics while the architectural rewrite is in flight
- Pretending a temporary facade is a valid end-state for the quarter

## Guiding Principles

### 1. Migrate by vertical slice

Each meaningful migration unit should be a command family, not "some helpers."

Good:

- `status now uses runtime`
- `submit now resolves auth/output/client through runtime`

Bad:

- `moved 12 helper functions out of shared`

### 2. Rewrite the foundation, migrate the surface incrementally

New runtime code must live beside existing code first. Old paths can delegate into new paths until enough of the tree is migrated, but the destination is replacement, not coexistence.

### 3. Temporary bridge code must expire

Bridge wrappers are allowed only to keep the repo moving while large slices are ported. They are not architecture. If a bridge still matters at the end of Q2, the migration is incomplete.

### 4. Freeze surface expansion where possible

If a new top-level command family does not materially improve release, review, metadata, build, signing, or CI workflows, it should usually wait until after Q2.

### 5. Keep compatibility work explicit

Deprecated aliases and migration shims must be inventoried, owned, and removed on a schedule. No "temporary" alias should be allowed to become indefinite by default.

## Architectural Target

Introduce a runtime package that owns cross-cutting CLI behavior, and rebuild command construction around it.

Proposed shape:

- `internal/cli/runtime`
- `Runtime`
- `ClientFactory`
- `AuthResolver`
- `OutputResolver`
- `Timeouts`
- `ProfileState`
- `LoggingConfig`
- `IO` or terminal/output capabilities

The runtime should be constructed once near [cmd/root.go](../../cmd/root.go) and passed into command constructors.

Q2 rewrite posture:

- re-implement the execution foundation under the existing command contract
- port the entire root surface to that foundation over the quarter
- allow shallow ports for low-priority families
- require deeper redesign for workflow-critical families
- delete migration scaffolding before quarter close

If we were starting greenfield, the target shape would look like this:

- a small root assembly layer
- one explicit runtime object
- thin command constructors
- workflow/domain logic separated from CLI wiring
- dedicated packages for auth, output, usage, and compatibility instead of one monolithic `shared`
- `internal/asc` request-shape code organized by domain rather than accumulated in umbrella files

Q2 does not need to reach a perfect greenfield state, but every phase should move the repo toward that shape rather than preserving current coupling.

Target command constructor style:

```go
func StatusCommand(rt *runtime.Runtime) *ffcli.Command
func ReleaseCommand(rt *runtime.Runtime) *ffcli.Command
func SubmitCommand(rt *runtime.Runtime) *ffcli.Command
```

Test-friendly target style for logic-heavy commands:

```go
func StatusCommand(rt *runtime.Runtime, runF func(context.Context, *StatusOptions) error) *ffcli.Command
```

## Baseline Hotspots

### Cross-cutting layers

- [cmd/root.go](../../cmd/root.go)
- [internal/cli/shared/shared.go](../../internal/cli/shared/shared.go)
- [internal/cli/shared/compat_aliases.go](../../internal/cli/shared/compat_aliases.go)
- [internal/cli/registry/registry.go](../../internal/cli/registry/registry.go)

### CLI packages with the most non-test weight

- `gamecenter`: `15,717`
- `web`: `12,874`
- `subscriptions`: `7,938`
- `testflight`: `7,851`
- `shared`: `7,722`
- `apps`: `5,953`
- `builds`: `5,879`
- `iap`: `5,772`
- `metadata`: `4,830`
- `submit`: `2,486` non-test, `6,552` total with tests

### `internal/asc` hotspots

- [internal/asc/client_options.go](../../internal/asc/client_options.go)
- [internal/asc/client_queries.go](../../internal/asc/client_queries.go)
- [internal/asc/client.go](../../internal/asc/client.go)
- [internal/asc/client_http.go](../../internal/asc/client_http.go)
- the resource-specific `client_*` files with mixed query, request, and transport concerns

## Workstreams

### Workstream A: Runtime Foundation

Owner outcome:

- introduce `internal/cli/runtime`
- stop adding new cross-cutting behavior directly to `shared.go`
- route auth, output, profile selection, timeout resolution, and client creation through runtime
- establish the replacement foundation that the rest of the quarter ports onto

Primary targets:

- [cmd/root.go](../../cmd/root.go)
- [internal/cli/shared/shared.go](../../internal/cli/shared/shared.go)

### Workstream B: Workflow-Critical Vertical Slices

Owner outcome:

- rebuild the commands that define the product story first

Primary families:

- `status`
- `release`
- `submit`
- `validate`
- `builds`
- `testflight`
- `metadata`
- `publish`

### Workstream C: Client Layer Simplification

Owner outcome:

- reduce the maintenance burden in `internal/asc`
- split giant option/query files by domain or resource
- keep transport and option plumbing from becoming a single catch-all layer
- make the client layer look more like deliberate domain code and less like sediment

Primary targets:

- [internal/asc/client_options.go](../../internal/asc/client_options.go)
- [internal/asc/client_queries.go](../../internal/asc/client_queries.go)
- resource-specific option/query helpers

### Workstream D: Taxonomy and Compatibility Discipline

Owner outcome:

- stop root command churn from spreading migration cost across the tree
- inventory deprecated aliases and remove the expired ones
- keep canonical surfaces stable

Primary targets:

- [internal/cli/registry/registry.go](../../internal/cli/registry/registry.go)
- [internal/cli/shared/compat_aliases.go](../../internal/cli/shared/compat_aliases.go)
- command-specific deprecated wrappers

### Workstream E: Testing and Guardrails

Owner outcome:

- make architecture migration observable and safe
- add targeted runtime and command-construction tests
- prevent new shared-state regressions

Primary targets:

- `internal/cli/cmdtest`
- runtime package tests
- root/registry wiring tests

## Phase Plan

## Phase 0: Baseline and Rewrite Contract

Window: April 1 to April 10

Goals:

- land this document
- define the runtime package shape
- inventory compatibility aliases and long-lived shims
- freeze non-essential top-level command expansion during Q2
- decide which experimental surfaces are explicitly deferred
- write down the greenfield target so migrations are judged against an end-state, not just against current code

Deliverables:

- this roadmap committed
- alias inventory table added to this file or a linked appendix
- initial runtime package skeleton
- root wiring plan approved
- explicit statement that bridge code is temporary and scheduled for deletion

Exit criteria:

- a runtime type exists, even if thin
- every new architecture PR links back to this document

## Phase 1: Rebuild the Foundation

Window: April 13 to May 1

Goals:

- create `internal/cli/runtime`
- route the following through runtime:
  - profile selection
  - auth resolution
  - ASC client creation
  - default output resolution
  - timeout resolution
  - root logging overrides
- convert `shared` from owner of state into owner of compatibility wrappers where needed
- make new construction patterns attractive enough that future code naturally lands on them

Primary files:

- [cmd/root.go](../../cmd/root.go)
- [internal/cli/shared/shared.go](../../internal/cli/shared/shared.go)

Exit criteria:

- root command constructs a runtime object once
- new code paths no longer call process-wide mutable setters for profile or output state
- a thin shim layer exists for legacy callers
- the foundation is good enough that Phase 2 can rebuild workflow families on top of it instead of compensating for it

## Phase 2: Workflow Core Migration

Window: May 4 to May 22

Goals:

- rebuild the highest-value command families on runtime-backed construction and execution

Priority families:

1. `status`
2. `release`
3. `submit`
4. `validate`

Why these first:

- they define the Q2 product story
- they already orchestrate multiple lower-level operations
- they currently lean hardest on shared cross-cutting behavior

Exit criteria:

- these families resolve auth, client, output, and timeout behavior through runtime
- constructors are testable without depending on shared global state
- command-level tests remain green
- these families look like the template for the rest of Q2, not like one-off ports

## Phase 3: Build, TestFlight, Metadata, and Client Layer

Window: May 25 to June 12

Goals:

- migrate the next set of workflow-heavy families
- split `internal/asc` option/query sprawl along resource boundaries
- start the bulk tree-wide import migration away from the monolithic `shared` package
- replace "dumping ground" files with domain ownership that would still make sense in a cleaner repo

Priority families:

1. `builds`
2. `testflight`
3. `metadata`
4. `publish`
5. `apps`
6. `reviews`
7. `versions`
8. `localizations`

Client cleanup targets:

- split [internal/asc/client_options.go](../../internal/asc/client_options.go) into domain-specific files
- split query helpers where resource or workflow ownership is obvious
- keep transport/core client code distinct from request-shape catalogs

Exit criteria:

- major workflow families use the same runtime path
- `client_options.go` is no longer the default destination for unrelated additions
- the bulk of root command families no longer import the monolithic `shared` package
- the new architecture is visibly winning in everyday development, not just in diagrams

## Phase 4: Long Tail, Deletions, and Release Hardening

Window: June 15 to June 30

Goals:

- finish the whole-tree port to the new foundation
- remove temporary migration shims and expired compatibility layers
- delete or nearly delete the old monolithic `shared.go`
- move long-tail families onto runtime-backed construction even where deep domain cleanup is deferred
- leave Q3 with feature-level follow-ups, not architecture debt from this rewrite
- make the repo feel like it has crossed a boundary, not like it is still mid-migration

Priority actions:

- port remaining root families and stragglers
- remove low-value wrappers made obsolete by runtime migration
- prune deprecated aliases whose replacement paths are stable
- remove bridge helpers and imports that exist only because the rewrite was staged

Exit criteria:

- runtime is the default pattern for all root command families
- the monolithic `shared` package is no longer a meaningful dependency surface
- compatibility inventory has owners and dates
- Q3 backlog is explicit and feature-oriented rather than "finish the migration"
- there is no plausible reason to route new code through the old execution model

## Week-by-Week Execution Plan

This is the operating schedule for Q2 2026. Treat each week as a real delivery slice with a visible artifact, not just a status label.

Weekly operating rules:

- each week must have one primary architecture objective
- each week should end with at least one mergeable PR or a clearly reviewable stack
- every Friday, update the measurement table and tracking template in this file
- if schedule pressure appears, protect workflow-core migration and deletion work before protecting low-value long-tail cleanup

| Week | Dates | Primary objective | Expected output | Exit gate |
| --- | --- | --- | --- | --- |
| 1 | April 1 to April 3, 2026 | Lock the rewrite contract | land this roadmap, define greenfield target shape, freeze non-essential root-family expansion | architecture work is now judged against this plan rather than ad hoc instinct |
| 2 | April 6 to April 10, 2026 | Baseline the rewrite | alias inventory, shared-responsibility inventory, runtime package skeleton, root wiring design note | runtime package exists and baseline metrics are captured in-repo |
| 3 | April 13 to April 17, 2026 | Rewire root construction | `cmd/root.go` constructs runtime once; root tests added or updated around construction and help/output behavior | runtime is the single assembly point for root execution |
| 4 | April 20 to April 24, 2026 | Move core cross-cutting concerns | auth resolution, default output resolution, timeout handling, and logging overrides routed through runtime | new code no longer needs process-wide setters for these concerns |
| 5 | April 27 to May 1, 2026 | Finish foundation slice | ASC client creation and profile state moved behind runtime; first focused `shared` splits land | Phase 1 exits with a usable replacement foundation, not just scaffolding |
| 6 | May 4 to May 8, 2026 | Rewrite `status` | `status` constructor and execution path rebuilt on runtime; tests updated around output and behavior | `status` becomes the first real template for the new architecture |
| 7 | May 11 to May 15, 2026 | Rewrite `release` and `validate` | `release` and `validate` moved to runtime-backed construction, with shared-state reads removed where touched | two more workflow-critical families prove the pattern is repeatable |
| 8 | May 18 to May 22, 2026 | Rewrite `submit` and harden workflow core | `submit` moved to runtime; workflow-core edge cases, docs, and tests tightened | Phase 2 exits with `status`, `release`, `submit`, and `validate` all on the new foundation |
| 9 | May 25 to May 29, 2026 | Rewrite `builds` and `testflight` | both families moved to runtime-backed construction; shared build-selection logic cleaned up where needed | build-oriented workflow surface is now on the same execution model |
| 10 | June 1 to June 5, 2026 | Rewrite `metadata` and `publish` | both families moved to runtime; related `versions` or `localizations` work starts where tightly coupled | metadata and publish flows stop depending on old foundations |
| 11 | June 8 to June 12, 2026 | Split `internal/asc` and port adjacent families | meaningful `client_options.go` split lands; `apps`, `reviews`, `versions`, and `localizations` move onto runtime where practical | Phase 3 exits with workflow-heavy families migrated and client sprawl shrinking visibly |
| 12 | June 15 to June 19, 2026 | Port long-tail batch one | `screenshots`, `videopreviews`, signing families, device/user families, and xcode-oriented families move to runtime | the remaining work is now mostly demolition and stragglers, not major foundation uncertainty |
| 13 | June 22 to June 26, 2026 | Port long-tail batch two and delete old paths | `web`, `gamecenter`, `iap`, `subscriptions`, and remaining root families ported; obsolete wrappers and aliases removed aggressively | every root family is now runtime-backed and shared-import burn-down is near zero |
| 14 | June 29 to June 30, 2026 | Stabilize and close Q2 | final bridge deletions, final metric snapshot, doc updates, Q3 carryover list limited to domain polish | Q2 definition of done is either met or any miss is explicit, narrow, and justified |

### Weekly Review Rhythm

Use the same cadence every week:

- Monday: pick the week slice, confirm scope, and note the target PR set
- Wednesday: verify the migration is reducing shared/global state instead of only moving code around
- Friday: update metrics, update the tracking table, and decide whether the next week keeps the same family or advances to the next slice

### Slip Policy

If the quarter slips, cut in this order:

1. deepest cleanup in low-value long-tail families
2. optional `internal/asc` generator exploration
3. non-essential taxonomy tidy-up that does not remove real migration debt

Do not cut:

- runtime foundation completion
- workflow-core migration
- deletion of temporary bridge code
- whole-root-family porting by June 30, 2026

## Command Family Triage

This table is the "whole codebase" view for Q2. Nothing in the root command tree is deferred from the foundation rewrite. The only thing that may be deferred is deep domain cleanup after the port is done. In other words: every family gets onto the new house this quarter, even if not every room is renovated to the same depth.

| Area | Q2 treatment | Phase | Notes |
| --- | --- | --- | --- |
| `auth` | Foundation rewrite | 1 | Must move with runtime first. |
| `shared` | Full split and retirement | 1-4 | Temporary bridge code allowed only during migration; delete it by quarter end. |
| `registry` | Rewrite root construction | 1-4 | Registry becomes runtime-aware and stops carrying migration debt indirectly. |
| `status` | Deep rewrite | 2 | Anchor workflow dashboard and prove the new pattern. |
| `release` | Deep rewrite | 2 | Canonical high-level shipping path. |
| `submit` | Deep rewrite | 2 | Core submission flow and validations. |
| `validate` | Deep rewrite | 2 | Pairs directly with release/submit. |
| `builds` | Deep rewrite | 3 | High churn and selector complexity. |
| `testflight` | Deep rewrite | 3 | High workflow value and shared build concepts. |
| `metadata` | Deep rewrite | 3 | Core workflow surface. |
| `publish` | Deep rewrite | 3 | Keep aligned with release path. |
| `apps` | Rewrite on new foundation | 3 | Large family; deep cleanup only where workflow-critical. |
| `reviews` | Rewrite on new foundation | 3-4 | Port fully; deepen only as needed. |
| `versions` | Rewrite on new foundation | 3-4 | Port with release/submit/builds work. |
| `localizations` | Rewrite on new foundation | 3-4 | Port with metadata. |
| `screenshots` and `videopreviews` | Rewrite on new foundation | 4 | Port fully; redesign can follow later. |
| `signing`, `bundleids`, `certificates`, `profiles`, `users`, `devices` | Rewrite on new foundation | 4 | Operational domains; full port, selective cleanup. |
| `xcode`, `xcodecloud`, `workflow`, `webhooks`, `notify`, `schema`, `snitch` | Rewrite on new foundation | 4 | Port to runtime-backed construction. |
| `web`, `gamecenter`, `iap`, `subscriptions` | Runtime port plus import cleanup | 4 | Full port in Q2; deeper domain refactors may continue in Q3. |
| all remaining root families | Runtime port plus import cleanup | 4 | No root family should still depend on old foundations after June 30. |

## `shared` Split Target

Current `shared` responsibilities include all of the following:

- root flag binding
- usage rendering
- output validation and rendering
- profile selection
- auth resolution
- client construction
- timeout helpers
- CSV and date helpers
- deprecation support

Q2 target package shape:

- `internal/cli/runtime`
- `internal/cli/shared/auth`
- `internal/cli/shared/output`
- `internal/cli/shared/usage`
- `internal/cli/shared/compat`
- `internal/cli/shared/parse`

Important rule:

No new cross-cutting behavior should be added to [internal/cli/shared/shared.go](../../internal/cli/shared/shared.go) once Phase 1 starts unless it is part of reducing or delegating the file.

Scratch-rebuild mindset:

- if a responsibility belongs in `runtime`, move it there
- if a responsibility belongs in a focused package, move it there
- do not preserve current placement just because it already exists

Retirement rule:

- temporary bridge wrappers may exist in Phase 1 and Phase 2
- no new code may be written against those wrappers once replacement packages exist
- all callers must be migrated by Phase 4
- [internal/cli/shared/shared.go](../../internal/cli/shared/shared.go) should be deleted, or reduced to a tiny non-authoritative shell under `150` lines, before Q2 closes

## `internal/asc` Cleanup Target

Q2 target is not "generate the entire client." Q2 target is to stop defaulting to giant umbrella files.

Rewrite stance:

- prefer reorganizing the client as if we were naming files fresh today
- keep compatibility only at call sites that still need old paths temporarily
- do not let file splits become cosmetic if ownership is still muddled

Rules:

1. New resource-specific options should not go into [internal/asc/client_options.go](../../internal/asc/client_options.go) unless they truly are cross-resource.
2. Prefer files like:
   - `client_options_builds.go`
   - `client_options_testflight.go`
   - `client_options_metadata.go`
3. Keep transport/core behavior in core files and request-shape catalogs in resource files.
4. Defer generator work unless manual splitting proves insufficient.

## Measurement

Track these every Friday during Q2:

| Metric | Baseline | Q2 target |
| --- | --- | --- |
| Root subcommands | `79` | no net growth without explicit approval |
| Imports of monolithic `internal/cli/shared` | `384` | `0` outside temporary bridge code |
| `shared.go` lines | `1482` | `0-150`, with deletion preferred |
| `client_options.go` lines | `3590` | under `1000`, or deleted into domain files |
| Runtime-backed root subcommands | `0` | `79` |
| Workflow-critical families on runtime | `0` | `status`, `release`, `submit`, `validate`, `builds`, `testflight`, `metadata`, `publish` |
| Compatibility aliases with owner/date | not centralized | `100%` inventoried and reviewed for removal |

## PR Rules for Q2

Every architecture-affecting PR should say:

1. which phase it belongs to
2. which workstream it advances
3. whether it increases or decreases shared/global state
4. whether it adds, removes, or extends a compatibility alias
5. what command family it migrates, if any
6. whether it primarily ports old code or meaningfully rewrites it onto the new shape

Preferred PR size:

- one command family migration
- one runtime slice
- one `internal/asc` split
- one compatibility cleanup batch

Avoid:

- mixed feature + architecture + taxonomy mega-PRs

## Tracking Template

Update this table as work lands.

| Family / Area | Runtime-backed | Shared globals removed | Tests updated | Compat risk | Owner | Status | PRs |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `auth` | no | no | no | medium | unassigned | planned | |
| `shared` | no | no | no | high | unassigned | planned | |
| `status` | no | no | no | low | unassigned | planned | |
| `release` | no | no | no | low | unassigned | planned | |
| `submit` | no | no | no | medium | unassigned | planned | |
| `validate` | no | no | no | low | unassigned | planned | |
| `builds` | no | no | no | high | unassigned | planned | |
| `testflight` | no | no | no | high | unassigned | planned | |
| `metadata` | no | no | no | medium | unassigned | planned | |
| `publish` | no | no | no | medium | unassigned | planned | |
| `internal/asc` options split | no | n/a | no | medium | unassigned | planned | |
| alias inventory | n/a | n/a | no | high | unassigned | planned | |

## Risks

### Risk: package cycles during `shared` split

Mitigation:

- build the runtime package first
- move dependencies outward, not sideways
- keep compatibility wrappers thin

### Risk: root behavior changes accidentally

Mitigation:

- add root/runtime construction tests
- keep help, output, profile, and auth behavior under explicit regression coverage

### Risk: migration stalls after the first slice

Mitigation:

- do Phase 2 immediately after runtime foundation
- prove the pattern on `status` and `submit` before broad expansion
- keep a weekly count of remaining imports of the monolithic `shared` package so drift is visible

### Risk: Q2 fills with new command churn instead of cleanup

Mitigation:

- require explicit approval for new root families
- favor improving canonical workflows over new breadth

## Definition of Done for Q2

Q2 is successful if all of the following are true:

1. Runtime-backed construction is the default pattern for all root command work, not just new work.
2. Every root command family has been ported onto the new foundation.
3. `status`, `release`, `submit`, and `validate` use runtime-backed execution and are cleaner than the code they replaced.
4. `builds`, `testflight`, `metadata`, and `publish` are fully migrated.
5. The monolithic `shared` package is no longer a real dependency surface.
6. `client_options.go` is materially reduced or split enough to stop being a dumping ground.
7. Compatibility aliases are inventoried with owners and removal dates, and expired ones have been deleted.
8. Q3 follow-up work is feature-level or domain-level polish, not "finish the architecture rewrite."
9. A new contributor reading the root/runtime path would learn the intended architecture from the code itself, not from tribal knowledge.

## Immediate Next Step

The first implementation PR after landing this file should do only this:

1. add `internal/cli/runtime`
2. wire root construction through it
3. move auth, output, client creation, and timeout resolution behind it

Do not start with `gamecenter`, `web`, or broad client generation work.
