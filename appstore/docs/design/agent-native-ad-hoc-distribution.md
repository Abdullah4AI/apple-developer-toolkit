# Agent-native ad hoc distribution

## Product direction

ASC should turn a local Xcode archive into a verifiable install result that an
agent can hand to a person or another system. The public contract is based on
the outcome, not on a particular automation framework or storage-provider
vocabulary. Each stage emits
structured output, writes deterministic artifacts under an operator-selected
root, and can be retried without repeating completed account mutations.

The complete workflow is planned as separate reviewable changes:

1. Generate modern Xcode `release-testing` export options and export an IPA.
2. Inspect the IPA and prepare a self-contained web-install bundle.
3. Publish that bundle through a caller-provided S3-compatible endpoint.
4. Reconcile registered devices and ad hoc provisioning profiles.
5. Use an explicit local PKCS#12 identity in an isolated signing environment;
   signing sync remains an optional lower-level preparation step.
6. Compose the stages into a resumable run with a durable receipt.

The first change in this stack owns only item 1. The second change owns only
local inspection and preparation from item 2; publishing remains a separate
network-facing boundary.

## PR 2 public contract

The `distribute` family begins with two provider-neutral local
commands:

```text
asc distribute inspect --ipa ./App.ipa [--include-devices] [--output json|table|markdown]
asc distribute prepare --ipa ./App.ipa [--output-dir DIR] [--title TITLE] \
  [--channel CHANNEL] [--source-revision REVISION] [--source-url URL] \
  [--output json|table|markdown]
```

`inspect` opens the IPA once without following a symlink, validates every ZIP
member name, rejects encrypted members, duplicate paths, an ambiguous main app,
and oversized selected metadata, then reports app, artifact, provisioning
profile, certificate, and metadata-preparation facts. Raw device UDIDs are
omitted unless the caller explicitly requests `--include-devices`; deterministic
device-set and certificate fingerprints are safe to pass between agents.

`prepare` applies the same inspection and requires an unexpired ad hoc profile
whose bundle identifier matches the main app and contains at least one device.
It writes a deterministic descriptor followed by the unchanged IPA in this
layout:

```text
bundle.json
payload/app.ipa
```

The default path is
`.asc/distribution/<safe-bundle-id>/<version>-<build>-<first-12-ipa-sha256>`.
The descriptor contains no timestamp, absolute input path, raw device UDID, URL
manifest, or storage-provider setting. An existing byte-for-byte-equivalent
bundle is reported as reused. Any other existing destination is a conflict and
is never overwritten. A new bundle is assembled in an unpredictable sibling
directory and published with no-replace semantics so `bundle.json` is never a
receipt for a partial bundle.

Both commands print data to stdout, diagnostics to stderr, and use exit code 2
for invalid flags or missing required flags. IPA or preparation validation
failures use the ordinary non-zero command error. This is an additive
surface and requires no migration.

### PR 2 security and verification

An IPA is treated as an untrusted ZIP, never extracted wholesale. Member names
must be canonical relative slash paths without traversal, backslashes, NUL or
control characters. The archive has fixed overall-size, entry, and
declared-expansion limits;
the main `Info.plist` is capped at 4 MiB and the embedded provisioning profile at
16 MiB, with both advertised-size and streamed-size enforcement. Preparation
uses rooted, no-follow reads and writes, copies from the already-open IPA file,
writes the descriptor last in staging, and refuses replacement at publication.

RED-GREEN coverage includes the complete JSON schema, explicit device
disclosure, ad hoc/development/enterprise/App Store classification, expired and
mismatched profiles, missing metadata, malicious ZIP paths, duplicate and
ambiguous app members, compressed-size limit bypasses, deterministic default
paths, exact reuse, conflict/no-overwrite behavior, table/Markdown rendering,
help/registration, built-binary stdout/stderr and exit behavior, command-doc
generation, and the repository validation gate.

Keeping install manifests out of preparation avoids pretending that URLs exist
before a publisher assigns them. Extracting the IPA to a directory would make
symlink and traversal handling much broader without adding needed metadata.
Using a mutable channel directory as the bundle identity would be convenient
for humans but would prevent safe retries and verifiable agent handoffs; channel
is therefore descriptor metadata rather than an output-path key.

The preparation result is `metadataEligible`; there is deliberately no generic
`eligible` or `installable` boolean. Inspection and the persisted descriptor
separately report profile CMS integrity, Apple profile trust, and the scoped
`complete-main-app-code-resources-entitlements-and-profile-certificate-binding`
verification. On macOS the
complete main app is safely materialized into private bounded staging,
`codesign --deep --strict` verifies its resource envelope and nested code, and
each architecture's leaf signer certificate must occur in the embedded profile.
Signed team, application identifier, debugging state, and other entitlements
must be permitted by that profile. This does not claim project-wide verification
when embedded targets exist; those IPAs remain blocked. Profile trust requires
the expected Apple provisioning signer and a chain to an exact pinned Apple
root. The verifier does not use the host root store and fails closed when a
recognized root is not carried in the CMS; supporting a newly introduced Apple
root requires a CLI update. CMS signature integrity alone is not Apple
authenticity. `prepare` refuses to write unless profile integrity, Apple trust,
and the exact complete-main-app signature scope are all `verified`.

Profile certificate fingerprints are explicitly named
`profileCertificateSha256Fingerprints`: they prove which certificates the
embedded profile permits, not which identity signed the IPA. An IPA containing
extensions, watch apps, or App Clips reports those target metadata paths but is
not marked eligible in this change; a later signing slice must validate every
target/profile pair before preparation claims project-wide readiness.

IPA processing is copied once from the already-open input into a private
snapshot so parsing, hashing, and publishing bind the same bytes. It is capped
at 8 GiB before ZIP parsing; selected metadata and the complete materialized
main app have expanded-size limits. Provenance text, IPA metadata, and ZIP member names are
bounded and reject control, Unicode format, and bidirectional-control characters.
`--source-url` must
be an absolute HTTPS URL with no user information, query, or fragment so a
deterministic descriptor cannot become a credential or signed-URL sink.

This boundary matches current production patterns without inheriting their
storage implementation: Blockstream separates local caller-URL bundle
generation from upload; Mattermost uses immutable PR, merge, and commit object
paths; Onym records structured build metadata and caps its index; ipa-server
generates the complete web-install surface across providers. Retention,
encryption, serialization, immutable object keys, channel indexes, comments,
and final install URLs remain publishing concerns rather than local preparation
concerns.

## PR 3: provider-neutral publication

`asc distribute publish` consumes the immutable bundle produced by
`asc distribute prepare` and makes it installable through a caller-owned,
S3-compatible object store. The command intentionally does not create buckets,
change ACLs or policies, or expose an AWS-shaped public API. The required
storage coordinates are `--endpoint`, `--region`, `--bucket`, and `--prefix`;
credentials come from the ordinary SDK chain, with optional `ASC_S3_*` aliases
for agents that should not need AWS-named environment variables. `--receipt` and
`--link-path` are also explicit required destinations outside the immutable
prepared bundle, so publication state never contaminates a bundle that prepare
may later reuse exactly.

Private publication is the default. It stores a content-addressed IPA first,
then an Apple installation manifest, and finally a small first-party HTML page.
All three objects use bounded presigned GET URLs. The install page expires at
the requested `--url-ttl`; the manifest and IPA URLs receive an additional
`--download-grace` period so a tap near expiry can still finish. URLs are
bearer credentials: normal JSON and receipts expose only a redacted install URL,
while the exact URL is written only to a mode-0600 link artifact. Public publication
requires both `--access public` and `--public-base-url`; it assumes the caller
has already configured anonymous reads and never mutates storage policy.
Private recovery validates each SigV4 signing time and lifetime against the
receipt's page deadline, with the configured grace applied only to the manifest
and IPA, before live-verifying any recovered URL.
Public objects can outlive the app's signing profile; the receipt therefore
records the profile expiry and verification facts, and publication requires a
currently valid profile with a safety margin. Private publication additionally
requires the profile to remain valid through the complete requested link and
download-grace lifetime.

The publisher validates a prepared `bundle.json` plus `payload/app.ipa`, rejects
unsafe descriptor paths, and verifies the IPA digest and size before any network
request. Existing objects are reused only when their SHA-256 metadata, length,
and content type match exactly; mismatches are immutable-key conflicts. Every
upload is followed by a no-redirect read verification; the IPA is downloaded
within the declared size bound and hashed end to end.
Retention remains the object-store operator's responsibility, preferably via a
bucket lifecycle rule; this command never deletes older builds.

Publication also fails closed until preparation records both a verified IPA code
signature and verified provisioning-profile integrity and trust. Code-signature
status alone is insufficient: the descriptor must carry the exact full-app scope
`complete-main-app-code-resources-entitlements-and-profile-certificate-binding`,
covering CodeResources, entitlements, the main executable, and profile-certificate
binding. The publisher also requires the verified signer-certificate fingerprints
to be canonical SHA-256 values present in the embedded profile certificate set,
and carries that evidence into recovery receipts. Narrow, missing, `not-verified`,
or unknown verification results are rejected even if another preparation
implementation writes such a descriptor.

The stable output contract is a camelCase JSON receipt containing schema,
provider-neutral object coordinates, artifact identity, verification results,
and a redacted install URL. Diagnostics and progress stay on stderr. Required
or malformed flags are usage errors (exit 2); local validation, authentication,
upload, and verification failures are ordinary command failures (exit 1).

RED-GREEN coverage starts at the CLI boundary, then uses local HTTP servers for
endpoint validation, signed PUT/HEAD/GET behavior, collision reuse/conflict,
upload ordering, generated manifest/page content, presigning, verification,
receipt/link permissions, and secret redaction. A built-binary invalid-invocation
check and an S3-compatible integration smoke test complete verification.

Alternatives considered were an AWS-specific command surface and uploading only
an IPA. The former would leak one provider's deployment model into an agent
workflow; the latter cannot produce Apple's `itms-services` installation flow.
Bundling a web server in ASC was also rejected because distribution ownership,
TLS, retention, and availability belong at the caller's chosen endpoint.

This shape follows the strongest production properties already proven in other
projects: Blockstream separates local manifest generation from provider upload
and leaves short retention to backend lifecycle; Mattermost uses immutable
PR/merge/commit object paths plus bucket-managed lifecycle and server-side
encryption; Onym publishes serially and emits a structured build index and URLs.
`ipa-server` demonstrates the value of arbitrary endpoints and public URL bases,
but its credential-in-configuration-string pattern is explicitly not adopted.

## PR 6: agent-native orchestration

PR 6 composes the lower-level commands into one typed state machine. It does
not add a lane language, execute a caller-supplied shell script, or hide the
effects behind one opaque `distribute` command. Planning, authorization,
execution, recovery, local inspection, and live verification remain distinct:

```text
asc distribute plan --archive-path PATH --config PATH --plan PATH \
  [--state-dir DIR] [--output json|table|markdown]
asc distribute apply --plan PATH --confirm PLAN_HASH [--output json|table|markdown]
asc distribute resume --run RUN_ID [--state-dir DIR] [--output json|table|markdown]
asc distribute status --run RUN_ID [--state-dir DIR] [--output json|table|markdown]
asc distribute verify --run RUN_ID [--state-dir DIR] [--device DEVICE] \
  [--timeout DURATION] [--output json|table|markdown]
```

Every command uses `shared.DefaultUsageFunc`. Long-form flags are canonical in
documentation and examples. There are no interactive
prompts. Machine-readable fields use camelCase; stage identifiers and enum
values use lowercase snake case.

### V1 scope

The first orchestration version deliberately supports one narrow, proven path:

- an existing iOS `.xcarchive` containing one main application target;
- a strict desired-devices file used by signing reconciliation;
- one explicit local PKCS#12 distribution identity and an optional protected
  password file;
- additive device, Bundle ID, and ad hoc provisioning-profile reconciliation;
- manual Xcode `release-testing` export;
- complete main-app, nested-code, resource, entitlement, profile-trust, and
  signer-certificate verification;
- private publication to an existing caller-owned S3-compatible bucket; and
- live fetch verification of the IPA, manifest, and install page.

Extensions, watch apps, embedded apps, and App Clips make the plan
`ready: false`. V1 also excludes archive creation, certificate creation or
revocation, capability mutation, public object access, bucket creation, bucket
policy or lifecycle changes, retention deletion, channel indexes, Git writes,
and automatic install or launch control on a person's device. The storage API
is S3-compatible but provider-neutral: AWS, Cloudflare R2, MinIO, and another
compatible service use the same fields.

`asc signing sync` is not part of the run state machine. An operator may use it
before planning to place a normalized identity on local disk, then reference
that local `.p12` and its password file in the distribution spec. The
orchestrator never clones, pulls, commits, or pushes a signing repository and
never guesses which identity from a repository should be used.

### Strict distribution config and spec

The file passed to `--config` is the distribution spec. It is a bounded
owner-private JSON file. Relative paths resolve against the config's directory.
It does not interpolate environment variables or accept credential values. A
representative V1 document is:

```json
{
  "schemaVersion": 1,
  "devicesFile": "devices.json",
  "signing": {
    "identity": {
      "format": "pkcs12",
      "path": "../signing/distribution.p12",
      "passwordFile": "../secrets/distribution-p12-password",
      "certificateSha256": "OPTIONAL_LOWERCASE_SHA256"
    },
    "minimumValidityDays": 7,
    "maxMutations": 32
  },
  "publication": {
    "endpoint": "https://objects.example.com",
    "downloadEndpoint": "https://downloads.example.com",
    "region": "auto",
    "bucket": "ios-builds",
    "prefix": "team/app",
    "addressingStyle": "path",
    "urlTtl": "24h",
    "downloadGrace": "1h",
    "verifyTimeout": "30s"
  },
  "metadata": {
    "title": "App",
    "channel": "pull-request-42",
    "sourceRevision": "abc123",
    "sourceUrl": "https://example.com/team/app/commit/abc123"
  }
}
```

`signing.identity.passwordFile`, `signing.identity.certificateSha256`,
`publication.downloadEndpoint`, and every metadata field are optional. An
omitted password file means the PKCS#12 must decode with an empty password.
Credentials are intentionally absent from the spec. The publisher keeps its
existing resolution contract: a complete `ASC_S3_ACCESS_KEY_ID` and
`ASC_S3_SECRET_ACCESS_KEY` pair, with optional `ASC_S3_SESSION_TOKEN`, wins;
otherwise it uses the standard AWS SDK credential chain.

The spec schema rejects unknown fields, duplicate keys, trailing JSON values,
invalid enums, non-positive limits, unsafe or unbounded text, credential-bearing
URLs, public-access fields, and inconsistent optional-field combinations. The
spec, devices file, identity, and password file are opened without following
the final path component. Secret inputs must be regular, owner-private files;
platforms with ownership and link-count support also require the current owner
and one link. The devices file keeps its existing strict V1 shape. Raw names and
UDIDs are consumed only from that protected file.

### `plan`

`plan` validates the complete spec before authentication. It then performs
bounded local inspection and read-only App Store Connect preflight. Storage
coordinates are validated locally during planning; apply resolves the configured
SDK credential chain before any Apple mutation. That proves credential
resolution, not future `PutObject` authorization. The first conditional object
write may still fail with a typed operational error from the provider.
It proves that the local identity contains exactly one usable private key and
certificate, selects that exact certificate for reconciliation, hashes the
complete archive tree, config, and devices file, and records the PKCS#12's
verified certificate and private-key relationship. Password bytes are neither
hashed nor serialized. Planning performs no POST, PUT, keychain import, profile
installation, Xcode export, or Git mutation.

The plan destination is required and create-only. A new plan never replaces an
old authorization artifact. An expected safety blocker is represented by exit
0 with `ready: false` and typed `blockers`; malformed input or a failed
preflight is an ordinary command failure. The strict plan artifact has this
shape:

```json
{
  "schemaVersion": 1,
  "planId": "dplan_RANDOM_128_BIT_VALUE",
  "planHash": "LOWERCASE_SHA256",
  "createdAt": "2026-08-13T12:00:00Z",
  "ready": true,
  "configPath": "/absolute/path/config.json",
  "configSha256": "LOWERCASE_SHA256",
  "archive": {
    "path": "/absolute/path/App.xcarchive",
    "treeSha256": "LOWERCASE_SHA256",
    "sizeBytes": 123456,
    "fileCount": 42,
    "bundleId": "com.example.app",
    "title": "App",
    "publishedTitle": "App Preview",
    "version": "1.2.3",
    "buildNumber": "42",
    "minimumOSVersion": "17.0",
    "teamId": "TEAM_ID",
    "targetCount": 1
  },
  "deviceSet": {
    "sha256": "LOWERCASE_SHA256",
    "fileSha256": "LOWERCASE_SHA256",
    "count": 2
  },
  "identity": {
    "certificateResourceId": "ASC_CERTIFICATE_RESOURCE_ID",
    "certificateSha256": "LOWERCASE_SHA256",
    "teamId": "TEAM_ID",
    "expirationDate": "2027-08-13T12:00:00Z",
    "minimumValidUntil": "2026-08-20T12:00:00Z"
  },
  "reconcile": {
    "planPath": "/absolute/path/reconcile/plan.json",
    "planHash": "LOWERCASE_SHA256",
    "receiptPath": "/absolute/path/reconcile/receipt.json",
    "minimumValidityDays": 7,
    "mutationCount": 2,
    "maxMutations": 32
  },
  "publication": {
    "endpoint": "https://objects.example.com",
    "downloadEndpoint": "https://downloads.example.com",
    "region": "auto",
    "bucket": "ios-builds",
    "prefix": "team/app",
    "addressingStyle": "path",
    "urlTtl": "24h",
    "downloadGrace": "1h",
    "verifyTimeout": "30s"
  },
  "effects": [
    {"stage": "account_reconcile", "kind": "register_device", "count": 1},
    {"stage": "account_reconcile", "kind": "create_profile", "bundleId": "com.example.app"},
    {"stage": "account_reconcile", "kind": "write_profile", "bundleId": "com.example.app"},
    {"stage": "export", "kind": "write_export_options", "bundleId": "com.example.app"},
    {"stage": "export", "kind": "write_ipa", "bundleId": "com.example.app"},
    {"stage": "prepare", "kind": "write_bundle", "bundleId": "com.example.app"},
    {"stage": "publish", "kind": "ensure_ipa"},
    {"stage": "publish", "kind": "ensure_manifest"},
    {"stage": "publish", "kind": "ensure_install_page"}
  ],
  "paths": {
    "stateDir": "/absolute/path/.asc/distribution/runs"
  }
}
```

A blocker has exactly `code`, `stage`, and a bounded redacted `message`. For
example:

```json
{
  "code": "embedded_targets_unsupported",
  "stage": "preflight",
  "message": "V1 requires one main application target."
}
```

`ready` is true only when `blockers` is empty.

Each effect has a closed `stage` and `kind` enum, an optional non-secret bundle
identifier or count, and the relationship that makes reuse safe. Account
mutation kinds are `register_device`, `create_bundle_id`, and
`create_profile`. Profile download is a local write, not an account mutation.
Object-write kinds are `ensure_ipa`, `ensure_manifest`, and
`ensure_install_page`; all use immutable keys and exact content evidence.
`write_profile` authorizes the verified local provisioning-profile output even
when no account mutation is required. Protected run snapshots and receipts are
state-machine evidence implicit in applying the plan rather than separate
external effects.

`planHash` is SHA-256 over the canonical complete plan payload excluding
`planHash` and `createdAt`. It therefore binds the random `planId`, every
recorded input digest and path, selected certificate, nested reconcile plan,
effective signing-validity policy, limits, destination, link policy, and effect
inventory. Any material change requires a new plan.

The effective signing-validity policy is the larger of the configured minimum
and a whole-day duration strictly longer than `urlTtl + downloadGrace + 1m`.
The plan binds both that policy and the absolute `minimumValidUntil`; local and
App Store Connect certificates and profiles must outlive it before any account
mutation. Known temporary object-store credentials are checked against the same
publication window before reconciliation and again before a new publication
intent, so the orchestrator never silently shortens an authorized link.

### Exact authorization and `apply`

The only accepted authorization is the full hash printed by `plan`:

```text
asc distribute apply \
  --plan .asc/distribution/app-plan.json \
  --confirm 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
```

Missing, malformed, or unequal `--confirm` is a usage error and is rejected
before authentication, state creation, or another side effect. `apply`
strictly decodes the plan, recomputes its hash, requires `ready: true`, reloads
the spec, and revalidates every input before the first account mutation. The
hash authorizes only the enumerated additive account mutations, local writes,
and immutable object writes. It does not authorize certificate creation,
deletion, overwrite, retention, bucket administration, or a different
destination.

One plan creates at most one run. Reapplying an exact plan returns the existing
run and its typed recovery status instead of creating a duplicate. The run
identifier is deterministically derived from the hash-bound random `planId` and
`planHash`, so the same authorization converges on one unpredictable run
directory even across processes. One process owns the exact original run-lock
inode. A retained state/run-directory lease detects pathname replacement before
and after each stage and checkpoint and fail-stops without publication or a
success result. This is drift detection, not a same-UID filesystem sandbox; a
replacement directory may acquire a distinct lock but cannot be adopted by the
original invocation.

Before reconciliation, apply:

1. recovers any prior ephemeral-signing journal;
2. takes a bounded private snapshot of the complete archive and verifies the
   planned tree digest;
3. reopens the devices file and verifies its digest and device-set fingerprint;
4. reopens and decodes the protected PKCS#12 and verifies its leaf-certificate,
   private-key, team, and validity evidence; and
5. resolves the configured storage credentials and rechecks the read-only
   account observations that make the plan safe.

The ordered execution stages are:

```text
preflight
identity_validate
account_reconcile
export
prepare
publish
fetch_verify
complete
```

Reconciliation remains additive and convergent. The downloaded or newly
created profile must embed the planned identity certificate and match the
planned team, bundle, ad hoc class, entitlements, and exact desired-device set.
Export runs with that explicit profile mapping and the identity in the isolated
temporary keychain. It invokes `xcodebuild` directly with a fixed argv and a
sanitized environment; distribution passwords, S3 credentials, and App Store
Connect credentials are not inherited by archive build scripts. A successful
export is not checkpointed until the profile and keychain cleanup result is
known. Cleanup failure leaves the run recoverable and blocks publication.

Preparation verifies the exported IPA again and binds its app, profile,
device-set, and signer evidence to the plan. Publication persists its intended
random link identity, keys, digests, generated documents, and sensitive URLs in
protected state before issuing the first PUT. It then follows the IPA,
manifest, install-page order and exact reuse rules described in PR 3. The run
does not become complete until all three published resources pass live fetch
verification.

### Run state, status, and receipt

The mutable run document is strict, versioned, typed state rather than an
arbitrary map of command outputs:

```json
{
  "schemaVersion": 1,
  "runId": "drun_DETERMINISTIC_128_BIT_VALUE",
  "planId": "dplan_RANDOM_128_BIT_VALUE",
  "planPath": "/absolute/path/.asc/distribution/app-plan.json",
  "planHash": "LOWERCASE_SHA256",
  "status": "recoverable",
  "stage": "publish",
  "updatedAt": "2026-08-13T12:05:00Z",
  "attempt": 1,
  "artifacts": {
    "archiveSnapshot": {
      "relativePath": "inputs/App.xcarchive",
      "treeSha256": "LOWERCASE_SHA256",
      "sizeBytes": 123456,
      "entryCount": 42,
      "app": {
        "bundleId": "com.example.app",
        "title": "App",
        "version": "1.2.3",
        "buildNumber": "42",
        "minimumOSVersion": "17.0"
      }
    },
    "reconcileReceipt": {
      "path": "reconcile/receipt.json",
      "sha256": "LOWERCASE_SHA256"
    },
    "profile": {
      "resourceId": "ASC_PROFILE_RESOURCE_ID",
      "uuid": "PROFILE-UUID",
      "path": "reconcile/profile.mobileprovision",
      "sha256": "LOWERCASE_SHA256",
      "bundleId": "com.example.app"
    },
    "exportOptions": {
      "path": "export/ExportOptions.plist",
      "sha256": "LOWERCASE_SHA256"
    },
    "signingReceipt": {
      "path": "signing/receipt-000001.json",
      "sha256": "LOWERCASE_SHA256"
    },
    "ipa": {
      "path": "export/App-000001.ipa",
      "sha256": "LOWERCASE_SHA256",
      "sizeBytes": 123456
    },
    "bundle": {
      "path": "prepared/bundle",
      "descriptorSha256": "LOWERCASE_SHA256"
    },
    "publication": {
      "receiptPath": "publish/receipt.json",
      "receiptSha256": "LOWERCASE_SHA256",
      "linkPath": "secrets/publication-intent.json",
      "linkSha256": "LOWERCASE_SHA256",
      "artifactKey": "team/app/objects/sha256/LOWERCASE_SHA256.ipa",
      "manifestKey": "team/app/links/RANDOM_VALUE/manifest.plist",
      "pageKey": "team/app/links/RANDOM_VALUE/index.html",
      "installUrlRedacted": "https://downloads.example.com/REDACTED"
    }
  },
  "lastFailureCode": "provider_outcome_unknown",
  "recoverable": true
}
```

Run status is one of `planned`, `running`, `recoverable`, `blocked`, or
`complete`. `stage` is one of the ordered execution-stage identifiers above.
A state transition is valid only when every prerequisite has exact durable
evidence. Unknown fields, inconsistent status/stage combinations, invalid
digests, and evidence outside the selected run root are rejected. Every member
of `artifacts` is optional until its stage has produced and checkpointed the
corresponding evidence.

`status` reads this state locally and performs no authentication, Git, Apple,
keychain, Xcode, storage, or HTTP operation. A valid running, recoverable, or
blocked run is
still a successful status query; automation branches on `status`, `stage`,
`lastFailureCode`, and `recoverable` rather than parsing prose.

Completion publishes an immutable create-only `receipt.json`. It binds
`runId`, `planHash`, app version/build, IPA digest and size, signing/profile
verification, object keys and digests, live-fetch result, redacted install URL,
profile expiry, and the paths and digests of its public and sensitive companion
artifacts. Its completion scope is exactly
`published_and_fetch_verified`. It never claims that a device installed or
launched the app. The exact install, manifest, and IPA URLs exist only in the
separate owner-private sensitive link artifact; normal output returns that
artifact's path, not its bearer values.

### Resume and live verification

`resume` continues only the exact confirmed plan. It never treats an earlier
process exit as proof that a durable effect completed. Before skipping a
successful stage it revalidates that stage's exact evidence:

- reconcile receipts and downloaded profile content against current account
  resources;
- local identity, profile, team, certificate, and expiry relationships;
- archive snapshot, IPA, and prepared-bundle digests;
- signing-session cleanup state; and
- published object, generated-document, receipt, link, and expiry evidence.

A failure definitely before a side effect is recorded as `recoverable` when
the same confirmed run can safely retry it, or `blocked` when a new plan is
required. A timeout, cancellation, crash, or lost response around a mutation
uses a closed `lastFailureCode` rather than adding an unbounded status. Resume
performs the exact read-only
reconciliation for that mutation before deciding whether it succeeded, may be
reused, or remains inconclusive. It never blindly repeats a device, Bundle ID,
profile, or object write. Input drift, a conflicting immutable resource, an
expired signing profile, and an expired private link stop as non-recoverable and
require a new plan; they are not silently repaired under the old confirmation.
Retention and deletion remain outside the run.

`verify` is read-only but live. It returns a typed result with
`publicationVerified`, the verified app/artifact identity, and an optional
redacted `deviceObservation`. It reopens the immutable receipt, prepared
bundle, and sensitive link artifact; reconstructs the manifest and install
page; verifies their exact relationships and expiry; then performs bounded,
no-redirect HTTPS reads of the page, manifest, and full IPA with content type,
size, and SHA-256 checks. Success means the planned release-testing artifact is
still fetchable through the recorded private publication. It does not claim an
install or launch result. Optional `--device` uses an explicit connected-device
selector to observe the exact bundle, version, and build; the selector is not
persisted or echoed, and this observation does not prove byte identity with the
published IPA.

### State and privacy boundary

The plan, run state, receipts, lock, and sensitive-link artifact live beneath
rooted handles anchored before execution. Path-based lower seams are bracketed
by a retained state/run inode lease, and any path drift is a terminal stop. Run directories are mode `0700` and
JSON state is mode `0600`. Files are bounded, owner-checked where supported,
opened without symlink traversal, and rejected when non-regular, hard-linked,
or group/world-accessible. Updates use unpredictable exclusive staging, file
and parent-directory synchronization, and no-replace publication. The final
receipt cannot be replaced. The permanent lock inode serializes cooperative
apply/resume owners of the unchanged run directory.

Raw UDIDs and device names, PKCS#12 or password bytes, secret-file contents,
App Store Connect credentials, S3 access keys or session tokens, credentialed
repository or endpoint URLs, exact presigned URLs, arbitrary child commands,
and unbounded provider bodies never appear in a plan, run document, normal
receipt, stdout, stderr, or error. Device names and UDIDs are reduced to a count
and deterministic set fingerprints. Diagnostics use closed error codes and
bounded redacted messages. Exact bearer URLs stay only in the sensitive link
artifact.

### Output and exit semantics

Data goes to stdout and progress or diagnostics to stderr. Explicit `--output`
wins; otherwise the ordinary TTY-aware ASC default applies. JSON remains one
parseable document even when an operational failure also returns a run
snapshot.

- Exit 0: the requested operation completed. `plan` may report `ready: false`,
  and `status` may report a running, recoverable, or blocked run, because both commands
  successfully answered their query.
- Exit 1: an operational, safety, provider, export, publication, live
  verification, or cancellation failure. Apply/resume includes the saved typed
  run state when it exists and a checkpoint was durably available.
- Exit 2: invalid syntax, an invalid flag or enum, a missing required flag, or a
  missing, malformed, or unequal `--confirm PLAN_HASH`.

The root process currently translates `SIGINT` into context cancellation; it
does not promise signal-number-preserving exit codes or `SIGTERM` handling.

`apply` exits 0 only after `fetch_verify` and immutable receipt publication.
`verify` exits 0 only when every requested live fetch check succeeds. Its
`--timeout` bounds the complete requested live check; the fetch verifier uses
the smaller of this value and the plan's configured `verifyTimeout`.

### Why this is agent-native

Imperative task runners commonly expose task-oriented vocabulary,
process-global environment, mutable keychain conventions, and a human log
stream. Those are not the contract ASC wants agents to reason over. ASC instead
exposes an immutable effect plan, exact hash authorization, closed
schemas and enums, durable evidence per stage, parseable recovery state, and
truthful verification scope. An agent can decide what will change,
request authorization for those exact effects, recover after a crash, and
prove what remains available without interpreting opaque steps or scraping
logs.

The existing generic `asc workflow` runner is also the wrong security boundary
for this default. It executes caller-authored shell commands, inherits broad
environment state, persists generic parameters and string outputs, and resumes
from process success rather than domain evidence. A workflow may call one
already-safe `asc distribute apply` or `resume` command as a higher-level
repository convention, but it must not own the identity, account mutation,
publication, secret, or recovery state.

### Acceptance evidence

On 2026-08-13, the lower-level stack was composed manually with a real SmolLens
archive, a local PKCS#12 identity, a reconciled ad hoc profile, and private
S3-compatible publication. The private OTA link was opened on a registered
physical iPhone, installation completed, and SmolLens launched successfully.
This establishes the real install-and-launch promotion gate for the composed
path. State-machine crash/recovery and private-object-store integration smoke
remain separate handoff evidence when matching signing inputs and storage
configuration are available.

The sanitized Xcode environment and process-group cleanup are isolation and
recovery controls, not a sandbox. Trusted project build scripts still execute
with the caller's UID and can access the caller's filesystem, keychains, and
network; a deliberately malicious script can create a new session. Private
identity and password files should therefore stay outside an untrusted project
tree.

No install URL, object-store bucket or prefix, device identifier, profile UUID,
certificate fingerprint, account identifier, or credential is retained in this
design note.

### PR 6 RED-GREEN and verification

RED begins at the public command boundary: registration and help; strict spec,
plan, run, and receipt decoding; exact plan-hash confirmation before auth;
read-only planning; hash coverage; single-main-app blocking; local-P12
selection and certificate binding; additive action inventory; protected state
paths; cooperative run locking plus pathname-drift detection; atomic persistence failures; crash points
before, during, and after each remote effect; unknown-outcome reconciliation;
successful-stage revalidation; secret-canary scanning of every stream and
artifact; sanitized Xcode environment; cleanup gating; immutable publication
reuse and conflict; local-only status; live verify; and exact exit codes.

Package tests use injected App Store Connect, Xcode, keychain, storage, HTTP,
clock, randomness, signal, and filesystem dependencies. CLI tests assert JSON,
table, and Markdown output plus stdout/stderr separation. The built binary is
then exercised against a disposable archive and local S3-compatible service,
followed by `make format`, `make check-docs`, `make lint`, and
`ASC_BYPASS_KEYCHAIN=1 make test`. Any later live mutation uses only the
disposable account and records cleanup separately from merge readiness.

## Placement and current behavior

`asc xcode export-options generate` currently always writes
`method=app-store-connect`. `asc xcode export` implicitly generates the same
options when `--export-options` is omitted. A caller can provide a custom plist,
but ASC cannot generate the non-App-Store export used for ad hoc delivery.

Xcode 26.6 and Xcode 27 call this method `release-testing`. Both versions still
accept `ad-hoc`, but mark it deprecated. ASC will use the current Xcode name and
will not introduce the deprecated spelling as a new public value.

## PR 1 public contract

The standalone generator adds:

```text
asc xcode export-options generate \
  --archive-path .asc/artifacts/App.xcarchive \
  [--method app-store-connect|release-testing] \
  [--destination export|upload] \
  [--signing-style automatic|manual]
```

`--method` defaults to `app-store-connect`, preserving every existing
invocation. `release-testing` requires `--destination export` because it creates
a local IPA rather than an App Store Connect upload. Its default output is
`.asc/export-options-release-testing.plist`; the existing App Store default
remains `.asc/export-options-app-store.plist`.

`asc xcode export` receives the same `--method` flag for its implicit generator:

```text
asc xcode export \
  --archive-path .asc/artifacts/App.xcarchive \
  --method release-testing \
  --signing-style manual \
  --ipa-path .asc/artifacts/App.ipa
```

An explicit `--export-options` file remains authoritative and cannot be combined
with `--method`, `--signing-style`, or `--team-id`. The default export method
remains `app-store-connect`. No existing output field changes; `method` reports
the actual generated value. Invalid values are usage errors with exit code 2.
Data remains on stdout and diagnostics remain on stderr.

## Implementation and compatibility

The repository-owned generator passes the selected method to the pinned Bitrise
typed models. App Store Connect continues to use the App Store model.
Release-testing uses the non-App-Store model. Manual signing resolution receives
the selected method so profile selection matches the requested export. The
pinned resolver still classifies installed ad hoc profiles with Xcode's legacy
`ad-hoc` enum, so ASC translates only at that internal resolver boundary and
continues to emit `release-testing` in the generated plist.

The change is additive. No deprecation or migration is required. The legacy
`ad-hoc` Xcode spelling is intentionally rejected with guidance to use
`release-testing`.

## RED-GREEN and verification

Coverage must establish:

- valid generator and implicit-export parsing for both methods;
- invalid and explicitly empty method values as usage errors;
- rejection of release-testing with `destination=upload` or `xcode export --wait`;
- conflict errors when `--method` accompanies an explicit plist;
- exact `method=release-testing` plist and JSON output;
- manual generator receipt of the selected method;
- portable and Darwin typed-model parity;
- unchanged app-store-connect defaults;
- generated command documentation, focused tests, built-binary stdout/stderr and
  exit codes, followed by the repository validation gate;
- real archive export with Xcode 26.6 and Xcode 27 before the distribution stack
  is declared complete.

## Handoff and verification gates

Each slice must be committed on its own feature branch and pushed at the exact
revision that passed its focused tests and repository validation gates. A
downstream slice must not be folded into the same commit merely because it uses
the preceding command. Review handoff must record the tested Xcode versions,
the exact commit, live verification performed, and any gate that remains.
When matching signing inputs or private storage are unavailable, the handoff
must explicitly record missing state-machine crash/recovery or object-store
integration smoke as an unresolved verification risk.

The `--method` workflow must be verified by exporting a real archive,
publishing a fetch-verified HTTPS manifest and IPA, and installing the
expected bundle and build on a registered device. Manual exports still depend
on a locally available distribution private key and provisioning profiles that
cover every embedded target and capability. Later slices must also settle the
security and retention contract for caller-provided storage, bearer install
URLs, device identifiers, and resumable state before the full `asc distribute`
workflow is complete.

## Alternatives

Accepting `ad-hoc` would mirror older automation tools but create a deprecated
surface on day one. Hiding the method only inside the future distribution
orchestrator would leave `asc xcode export` incomplete and make that orchestrator
depend on a private code path. Supporting every Xcode export method in this
change would widen the review without helping the first install-link workflow.
