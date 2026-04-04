# Publishing Process Simplification

## Goal

Collapse App Store publishing guidance to one canonical answer while keeping
older automation working during the migration window.

## Canonical command map

- `asc publish appstore` - canonical App Store publish path
- `asc release stage` - canonical pre-submit preparation path
- `asc publish testflight` - canonical TestFlight publish path
- `asc validate` - canonical App Store submission readiness path
- `asc submit status|cancel` - lower-level submission lifecycle tools
- `asc review ...` - raw review-submission resource management

## What changed

Two App Store-facing command paths were causing avoidable confusion:

- `asc release run`
- `asc submit create`

Both still work as deprecated compatibility paths, but neither should be taught
as the primary answer to "how do I publish to the App Store?" The primary
high-level answer is `asc publish appstore`.

## Use-when guidance

### Use `asc publish appstore` when

- you want to publish an App Store release
- you have an IPA or build-oriented App Store publish flow
- you want one deterministic command that can upload, ensure the version exists,
  apply metadata, attach the build, validate readiness, and submit for review
- you want the command agents and humans should reach for first

### Use `asc release stage` when

- you want the same high-level preparation flow
- you are not ready to submit yet
- you need to stage metadata and build attachment before a later manual or
  automated approval step

### Use `asc publish testflight` when

- you are distributing to TestFlight
- you want an IPA-first high-level flow for beta delivery

### Use `asc submit ...` when

- you want submission status or cancellation commands
- you are debugging review state
- you are maintaining an older direct-submit script and have not migrated off
  `asc submit create` yet

### Use `asc release run` when

- you are maintaining older automation that still shells out to the legacy
  release command group
- you need a compatibility path during the migration window while moving to
  `asc publish appstore`

`asc submit preflight` remains available as a deprecated compatibility wrapper
for older scripts that still expect the legacy preflight-style output.

### Use `asc review ...` when

- you need direct access to review-submission resources, items, attachments, or
  history
- you are doing advanced or API-shaped review workflow debugging

## Why `publish appstore` is the best App Store command

- It matches the real user intent: publish an App Store release.
- It uses the same `publish` taxonomy as TestFlight, so the top-level mental
  model stays consistent.
- It includes the surrounding steps users routinely forget when they jump
  straight to submission.
- It aligns the CLI with the documentation and with agent expectations.
- It preserves `release run` as a migration shim instead of the primary
  learning path.
- It preserves `submit` for lifecycle/tooling duties instead of overloading it
  as both a publish command and a submission-debug command.

## Migration policy

- Keep `asc release run` runnable with a deprecation warning.
- Keep `asc submit create` runnable with a deprecation warning.
- Hide deprecated App Store entry points from primary discovery where practical.
- Prefer `asc publish appstore` in help text, templates, migration hints,
  examples, and CI docs.
