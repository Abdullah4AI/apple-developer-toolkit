# Signing reconciliation for release testing

## Placement and command contract

`asc signing fetch` can find or create one provisioning profile, but it does not
prove that an existing profile contains a newly registered device, matches every
embedded target in an archive, or remains valid for a requested time window.
`asc signing reconcile` is therefore an additive, artifact-backed command group
beside `signing fetch` and `signing sync`:

```text
asc signing reconcile plan \
  --archive-path .asc/artifacts/App.xcarchive \
  --devices-file .asc/distribution/devices.json \
  [--certificate CERTIFICATE_ID] \
  [--minimum-validity-days 7] \
  [--max-mutations 32] \
  [--state-dir .asc/distribution/signing] \
  [--overwrite]

asc signing reconcile apply \
  [--plan .asc/distribution/signing/plan.json] \
  --confirm
```

Both leaves support normal `--output json|table|markdown`. Planning is read-only
and writes a mode-0600 `plan.json`. Apply reads and verifies that plan, re-reads
remote state before every planned mutation, performs only the additive actions
recorded in it, downloads and verifies each selected or created profile, and
writes a mode-0600 `receipt.json` plus `profiles/<UUID>.mobileprovision`. A
partial receipt is written after every completed action. On retry, every
idempotent ensure and verification action is rerun against current state, so
the receipt is evidence of progress rather than authority to skip checks.

The devices input is strict JSON and rejects unknown fields, duplicate UDIDs,
non-iOS devices, and an empty device list:

```json
{"schemaVersion":1,"devices":[{"name":"Test iPhone","udid":"...","platform":"IOS"}]}
```

The devices and plan inputs are protected, bounded regular files: on Unix they
must be mode 0600 or stricter and no symlink component is followed. Protected
input and parse failures are usage errors with path- and value-safe diagnostics;
raw device names and UDIDs are redacted from remote preflight failures.

Neither the plan nor normal output contains raw UDIDs. Device references use a
SHA-256-derived fingerprint and the App Store Connect resource ID when known.
Validation failures are usage errors (exit 2); remote and artifact failures are
ordinary non-zero errors. A valid but blocked plan is successfully emitted with
`ready=false`; apply refuses it.

## API contract

The offline OpenAPI snapshot defines the exact operations used here:

- `GET /v1/devices`, paginated with `filter[platform]`, to resolve enabled
  devices; `POST /v1/devices` with required `name`, `udid`, and `platform`.
- `GET /v1/bundleIds?filter[identifier]=...`; `POST /v1/bundleIds` with required
  `name`, `identifier`, and `platform` for a missing explicit iOS identifier.
- `GET /v1/certificates`, paginated and filtered to iOS distribution types.
- `GET /v1/bundleIds/{id}/profiles`, followed by paginated
  `GET /v1/profiles/{id}/certificates` and `/devices`.
- `POST /v1/profiles` with `IOS_APP_ADHOC`, one bundle ID, one selected
  certificate, and the desired devices. `GET /v1/profiles/{id}` supplies the
  base64 profile content for verification and download.

Profiles are immutable in this API. Reconciliation creates a successor when a
device set changes and retains every old profile. It never sends `DELETE` or
`PATCH`, enables or renames a device, creates a certificate, or changes a
capability.

## Planning and apply semantics

The archive inventory contains the main application and embedded application,
extension, Watch, and App Clip bundles in stable path order. Each target records
its bundle identifier and signed executable entitlements. All targets must use
one team. The main application must unambiguously declare the `iPhoneOS`
platform in `CFBundleSupportedPlatforms` and/or `DTPlatformName`; other or
unverifiable archive platforms are rejected before planning iOS resources.
Missing explicit App IDs may be planned only for baseline entitlement sets; a
target requiring a capability that has not already been registered is a blocker
because this command does not mutate capabilities.

For an existing App ID, `seedId` must exactly match the archive target's
`AppIDPrefix`. Apply repeats this check against the exact App ID returned after
concurrent-create convergence and again before profile creation. Capability
presence alone is insufficient for entitlements with value-specific settings
such as App Groups, Apple Pay merchant identifiers, Network Extensions, and
Wallet pass identifiers. Those entitlements remain blocked as unverifiable
rather than risking creation of an unusable profile.

Certificate selection considers only active, unexpired iOS distribution
certificates. More than one eligible certificate is a blocker unless
`--certificate` selects one. An explicit certificate that is absent or
ineligible is also a blocker. The selected certificate's DER is parsed and its
SHA-256 fingerprint is bound into the plan.

An existing profile is reusable only when it is active, valid through the
minimum window, belongs to the exact App ID, uses the selected certificate,
has exactly the desired enabled-device set, and its authenticated CMS content
semantically contains the target's signed entitlements and the selected
certificate fingerprint. Device supersets are not reused because they broaden
distribution beyond the explicit input. Suitable profiles rank by later
expiration, then resource ID. Otherwise the plan contains a deterministic
profile-create action. The profile name contains a hash of the bundle,
certificate, and device set, so retries converge.

The plan hash covers the archive target descriptors, team and entitlements,
desired device fingerprints, selected certificate, observed remote
preconditions, ordered actions, mutation ceiling, and output paths. It excludes
`generatedAt` and the hash itself. Apply re-inventories the archive and devices
file, compares entitlement numbers with exact JSON numeric semantics, recomputes
the hash, and then re-resolves remote preconditions, including the App ID
platform, seed, and capabilities immediately before profile creation. Numeric
comparisons retain integer exactness beyond the lossless `float64` range rather
than rounding. Only monotonic, already-satisfied drift is accepted. A conflict
response is followed by an exact reread rather than a blind retry. Resume
receipts are rebound to the hash-protected plan state directory before any
persistence, so receipt fields cannot redirect recovery writes.
Downloaded profiles are written create-only. A retry may reuse the same UUID
only when the existing bytes have the identical SHA-256 digest. Atomic
publication tolerates only platform/filesystem errors that specifically report
directory durability sync as unsupported after a successful no-replace publish;
other sync failures remain fatal.

## Compatibility, tests, and alternatives

This is an additive surface. Existing signing commands and output
remain unchanged. Tests start RED at the command boundary, then cover strict
input validation before auth, protected bounded input, deterministic hashing
and ordering, GET-only planning, pagination, App ID and profile creation
payloads, idempotent resume, entitlement/profile verification,
forged CMS rejection, embedded-certificate binding, exact device-set privacy,
same-UUID content conflicts, file modes, and symlink/path containment. Focused
package tests, command tests, a built-binary smoke test, generated command
documentation, and the repository gate complete verification.

Updating an existing profile is unavailable in the API. Deleting stale profiles
would make this workflow destructive and is intentionally excluded. Accepting
raw bundle/device flags instead of archive and versioned input files would be
shorter, but would omit embedded targets and make agent retries difficult to
audit. Automatically enabling capabilities would reduce blockers but materially
expand account mutation and is reserved for a separate explicit workflow.
