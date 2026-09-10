# IPA re-signing design

## Placement and command shape

Add `resign` below the existing `signing` command group. The command is a
macOS-only local operation:

```text
asc signing resign --ipa PATH --output PATH --identity PATH --profiles-manifest PATH [--identity-password-file PATH] [--rebase-team-claims] [--format FORMAT]
```

The command-specific `--ipa`, `--output`, `--identity`,
`--identity-password-file`, and `--profiles-manifest` flags are supported;
`--ipa`, `--output`, `--identity`, and `--profiles-manifest` are required, while
the password-file path is optional. The opt-in `--rebase-team-claims` flag defaults to disabled.
The destination is create-only: an existing path is a hard conflict and there
is no overwrite flag in the first release. Positional arguments are rejected.
The output format uses the repository's standard table, JSON, and Markdown
renderers. The operation has no App Store Connect API endpoint or network
dependency.

`--rebase-team-claims` is an explicit opt-in for a narrow cross-team
transition. Without the flag, the fail-closed refusal behavior below is
unchanged. With the flag, the command first builds one immutable, whole-IPA
entitlement plan and validates every transformed value before writing any
generated entitlement document or replacement profile. A later target or
nested-code failure therefore cannot leave an earlier target partially
prepared.

## Current behavior and boundaries

`asc signing run` imports one identity and one profile into an isolated,
temporary keychain/profile environment for a child process. Archive signing
reconciliation discovers app-like targets and validates entitlements, while
distribution inspection can validate a main app and its nested code but
explicitly does not prepare embedded targets. None of those commands mutates
an existing IPA.

The new operation must reuse the signing identity parsing and temporary
keychain boundary, but must not install profiles globally. IPA parsing and
publication should use the bounded ZIP, `rootfs`, and `secureopen` patterns in
`internal/distribution` and `internal/xcode`.

## Local pipeline

1. Reject unsupported platform, positional arguments, missing flags, invalid
   format, and an existing output before opening private signing inputs.
2. Open the input through no-follow rooted access and snapshot it into private
   mode-0700 temporary storage. Validate archive entry names, duplicate paths,
   file/directory collisions, symlinks, encryption, declared expansion, and
   actual streamed expansion.
3. Require one `Payload/*.app` and discover only the supported app-like target
   locations: app extensions, watch applications/extensions, and App Clips.
   Validate each target's bounded `Info.plist`, bundle identifier, executable,
   platform, and Mach-O executable. Main-app, extension, and App Clip targets
   require iOS device metadata; discovered Watch targets require matching
   `watchos`/`WatchOS` metadata. Replacement profiles continue to use the
   profile parser's accepted iOS platform set for bundled Watch targets; no
   genuine signed Watch fixture was available for this local test lane.
   `CFBundleSupportedPlatforms` is decoded without lossy coercion: it must be
   a non-empty array of non-empty strings, with no control characters or
   case-insensitive duplicates. Scalar/object values, mixed member types,
   empty members, and malformed extras are operational archive failures (exit
   1), while a well-formed but unsupported canonical platform is a usage
   failure (exit 2).
   Reject unrecognized nested app bundles.
4. Strictly decode the manifest before creating any keychain. It maps each
   discovered target's exact bundle identifier to a relative, regular,
   no-follow profile file. Reject unknown fields, duplicate keys, duplicate
   mappings, traversal, wildcards, and extra/missing targets.
5. Parse and verify every profile's CMS integrity and Apple trust chain,
   classify development/ad-hoc/App Store profiles, reject enterprise/unknown
   classes, and require one team and one identity certificate binding. Parse
   the PKCS#12 identity and require one usable private key, matching leaf
   certificate, current validity, and a team/certificate match for every
   profile.
6. Use the private mutable staging snapshot. For each app-like target, replace
   `embedded.mobileprovision`, derive provisioning-controlled entitlements
   from the existing signed entitlements, and reject any non-identity
   capability change that is not permitted by the replacement profile. A
   wildcard profile authorization is never emitted as a signed identity
   entitlement: an existing concrete value is retained only after it is
   authorized. Capability-group identity claims always keep the app's
   existing concrete subset; the profile value, wildcard or concrete, is an
   authorization boundary only. Optional identity capabilities absent from
   the existing signature are omitted rather than granted from the profile,
   while a required identity claim that cannot resolve to a concrete value
   fails closed. Claims the replacement profile does not authorize, such as
   old-team-prefixed keychain or ubiquity values, fail closed with a refusal
   that lists every blocked claim, its offending value, and a per-claim
   manual remediation; claims are never rewritten automatically. The refusal
   contains only bounded, escaped entitlement keys, values, and remediation
   text, so it is reported safely through the CLI instead of being reduced to a
   closed stage/code message. Profile-class-controlled claims
   (`aps-environment`, `com.apple.developer.devicecheck.appattest-environment`,
   `beta-reports-active`, and
   `com.apple.developer.icloud-container-environment`) take the replacement
   profile's class-authorized value when the existing signature already
   claimed them. `beta-reports-active` is omitted for development and ad-hoc
   replacements, and an App Store profile may derive it when the existing
   signature has no claim. A class-controlled environment claim is never
   granted if the existing signature did not already include it. Before
   writing the private entitlement documents, require existing concrete
   application-identifier claims to agree with one another and end in the
   exact target bundle identifier; the alternate
   `com.apple.application-identifier` synonym is optional. Validate an
   existing team-identifier claim syntactically, but do not infer that a
   legacy application-ID prefix must equal it. Replacement-profile identity
   claims are checked independently against the target and profile fields.
   `--rebase-team-claims` permits only these additional transformations, and
   each transformed value is checked against the replacement profile after
   transformation: concrete `keychain-access-groups` entries and the scalar
   `com.apple.developer.ubiquity-kvstore-identifier` may be rebased. KVS is
   planned once for the whole IPA: an exact existing value is preserved when
   authorized; otherwise one exact replacement-profile value with the same
   validated suffix is required. A wildcard can authorize that exact value,
   but never supplies a KVS destination. KVS prefixes are parsed from the KVS
   claim itself and may differ from application-ID prefixes. Changing the KVS
   value selects a different namespace and can make existing KVS data
   inaccessible. Keychain-group
   source prefixes are taken from the signed `application-identifier` claim,
   never from `TeamID`; values must be concrete, well-formed, and retain their
   original order, including duplicates. A repeated old KVS value or keychain
   group must resolve to exactly one planned value across the IPA, and
   distinct old values may not collapse into one destination. A concrete
   third-prefix value may remain unchanged only when its replacement profile
   authorizes it. No profile wildcard is emitted, and profile-only entries are
   never added. App Clip parent and associated-identifier relationship claims
   are rebased only through a proven unique reciprocal main-app/App-Clip pair;
   if pairing cannot be proven, each is preserved only when already authorized
   unchanged. Every other entitlement,
   including iCloud containers and migration/data claims, remains exact-only
   and is never generically rewritten. The JSON receipt reports one flattened
   entry per changed scalar or array element, with optional array indexes, in
   canonical target/key/index/value order under `entitlementRewrites`.
7. Create a dedicated temporary keychain using the existing recovery/journal
   and lock boundary, import the already validated identity, and sign leaf
   nested code before its enclosing `.framework`, `.bundle`, and `.xpc` code
   containers, then extensions, watch apps/App Clips, and the main app. Invoke `/usr/bin/codesign` directly with an
   explicit keychain and identity; never use `codesign --deep` for mutation.
8. Verify every target and nested Mach-O object with bounded direct tool
   invocations, including resource seal, profile, entitlements, team,
   application identifier, and signer certificate binding. Repack into a new
   IPA, validate the generated archive, and publish with no-replace atomic
   rooted output. The input is never rewritten. Preserve each validated
   regular-file permission mode (defaulting only when the ZIP omitted mode
   metadata); unsafe group/world write modes are rejected. App-like target
   executables and nested Mach-O scheduled for signing, including `.xpc` and
   `.bundle` executables, must already have the owner-execute bit. DOS-created
   archive members default to `0644` and are rejected rather than silently
   gaining execute permission. The preserved `WatchKitSupport2/WK` binary has
   the same owner-execute requirement. Preserve
   the exact `SwiftSupport/iphoneos/*.dylib` layout byte-for-byte without
   treating those distribution-side runtime libraries as app code to re-sign.
   The `SwiftSupport` root may contain only the `iphoneos` directory, whose
   direct children must be regular, non-symlink `.dylib` files; nested,
   alternate-platform, root-file, and other entries are rejected. A
   `WatchKitSupport2` root, when present, may contain only the regular,
   non-symlink `WK` binary, preserved under the same provenance verification
   and inventory equality rules. Re-run the
   strict Apple generic-anchor `codesign` verification on the final packed
   tree before publication. The repacked entry count must remain within the
   validated archive limit, including materialized ancestor directories.
   Capture a private, sorted inventory of every
   direct runtime using its normalized relative path, bounded size (at most
   1 GiB per file), SHA-256 digest, and validated permission mode. Rebuild the
   inventory after repack and require exact path, size, digest, and mode
   equality in addition to the final provenance check; added, dropped,
   renamed, replaced, or mode-mutated runtimes fail before publication. The
   inventory is internal only and never appears in output or telemetry.
   Direct codesign invocations inherit a caller deadline or use a bounded,
   multi-minute phase fallback.
9. Remove temporary keychain, generated entitlements, staging, and journal on
   all paths. Cleanup errors are joined with the primary error and cannot be
   reported as success.

## Output and errors

JSON is a schema-versioned, registered `internal/asc` output type containing
input/output size and SHA-256 digests, public leaf-certificate digest/team,
target relative path and bundle identifier, profile class/UUID/digest, and an
all-target verification status. When claim rebasing is enabled, it also
contains the deterministic `entitlementRewrites` audit records described
above; the field is omitted when the flag is absent. Table and Markdown expose
the same safe fields. It never emits
passwords, PKCS#12/profile source paths, temporary keychain paths, raw profile
plists, device identifiers, or raw subprocess diagnostics.

Usage validation returns exit code 2. IPA, profile, identity, entitlement,
signing, verification, cleanup, and publication failures return a nonzero
execution error. Path-bearing operational failures use a closed internal
stage/code error whose public `Error()` is stable; the original filesystem,
tool, and cleanup causes remain available to internal `errors.Is`/`errors.As`
callers through `Unwrap` but are never rendered by the CLI. The output parent
is not created until all input, profile, identity, signing, and verification
checks pass. Exit 0 is possible only after output publication and
post-publication validation succeed. Any failure after the no-replace
publication creates the artifact but before its size, reopen, hash, or close
checks complete is reported as an ambiguous publication
(`ErrSigningResignPublicationAmbiguous`); the artifact is left in place for
inspection and must not be blindly retried.

## Compatibility and alternatives

This adds only a command and does not alter `signing run`,
archive reconciliation, distribution inspection, or stable output schemas.
It intentionally supports only iOS device IPAs and development, ad-hoc, and
App Store profiles; other platforms, enterprise profiles, wildcards, arbitrary
entitlement files, and overwrite behavior remain unsupported.

An alternative is to shell out to Xcode export. That cannot safely express a
complete existing-IPA target/profile mapping and would make output and cleanup
less deterministic. Another alternative is to add a general-purpose signing
library. That would duplicate Apple's signing tool behavior and enlarge the
review surface; direct, bounded `/usr/bin/codesign` calls keep the trust
boundary explicit.

## RED/GREEN validation

Begin with CLI tests for registration, help, required flags, positional args,
output formats, macOS gating, and exit behavior. Add unit tests for strict
manifest decoding, archive/target inventory, profile and identity binding,
entitlement rewriting, leaf-first order, no-deep mutation, output redaction,
no-replace publication, cancellation, and cleanup. Add macOS integration
coverage with a disposable signed nested-target fixture when signing assets
are available.

Required repository gates:

```bash
make build
make format
make check-docs
make lint
ASC_BYPASS_KEYCHAIN=1 make test
```
