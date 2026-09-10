# Opt-in cross-team entitlement claim rebasing

## Status and scope

This note defines the design issue [#2249](https://github.com/rorkai/App-Store-Connect-CLI/issues/2249). Issue [#2251](https://github.com/rorkai/App-Store-Connect-CLI/issues/2251) is the implementation follow-up that should consume this contract. It is a design for a future additive capability; it does not change the current signing implementation.

The implementation depends on [#2241](https://github.com/rorkai/App-Store-Connect-CLI/pull/2241), which introduces `asc signing resign`. Its first version deliberately refuses existing claims that are not authorized by the replacement profile. Rebasing is an explicit exception to that refusal, and must never become an implicit fallback.

The design has four boundaries:

- only a small, documented set of team-prefix claim grammars may be transformed;
- every transformed value must be authorized by the replacement profile;
- references between embedded bundles are checked as one graph before any signing mutation;
- the result reports every transformation without exposing signing secrets or arbitrary profile data.

## Placement and command contract

The feature belongs to the existing `signing resign` leaf. The command remains local and offline: it reads the IPA, identity, password file, and strict profiles manifest, and does not call App Store Connect.

The intended invocation is:

```text
asc signing resign \
  --ipa PATH \
  --output PATH \
  --identity PATH \
  --profiles-manifest PATH \
  [--identity-password-file PATH] \
  [--rebase-team-claims] \
  [--format FORMAT]
```

`--rebase-team-claims` is a command-specific boolean flag. It defaults to false. `--output` remains the artifact destination, and `--format` remains the result renderer; the new flag must not overload either name.

When the flag is absent, the command retains the existing #2241 refusal semantics and structured output shape: an existing unauthorized claim is refused, the diagnostic gives manual remediation, no automatic prefix derivation occurs, and `entitlementRewrites` is omitted. When the flag is present, only claims accepted by the rules below may be transformed. It is not an authorization bypass, a profile repair operation, or a way to grant a capability that was absent from the signed input.

The command reserves exit 2 for invocation and input-shape errors: flag parsing, missing or empty flag values, positional arguments, unsupported platforms, invalid output-format values, and invalid strict profiles-manifest JSON or mapping shape. Once valid paths and manifest shape are accepted, malformed IPA/archive bytes, malformed signed-entitlement plists, malformed CMS/mobileprovision bytes, profile-target or identity mismatches, unauthorized claims, signing, verification, and output failures are operational failures with an ordinary non-zero exit. A missing or unreadable file is also operational after its path flag has passed validation. Diagnostics go to stderr. Successful structured output goes to stdout unless the existing renderer contract directs it elsewhere.

## Existing behavior and dependency boundary

The #2241 pipeline inventories the main app and embedded application targets, reads their signed entitlements, validates the replacement profile for each target, generates exact entitlement documents, signs leaf-first, and verifies the packed IPA against those generated documents. It already preserves concrete subsets of existing identity-group claims and omits optional claims that were absent from the signed input.

The current refusal is important. For example, an existing `OLDPREFIX.com.example.shared` keychain group and a replacement profile containing `NEWPREFIX.*` cannot be silently changed merely because the suffix looks familiar. The new flag makes that transformation possible only after source-prefix, claim-grammar, profile-authorization, and whole-IPA checks succeed.

## Prefix model and derivation

The following identifiers are distinct and must remain distinct in code and documentation:

- `oldPrefix`: the concrete App ID prefix in the existing target's signed `application-identifier`, before the dot separating the prefix from the bundle identifier;
- `newPrefix`: the replacement profile's `ApplicationIdentifierPrefix`;
- `oldTeamID` and `newTeamID`: the signed and replacement profile team identifiers;
- `oldApplicationID` and `newApplicationID`: the complete target application identifiers, including the prefix and bundle identifier.

An App ID prefix is often, but is not required to be, the Team ID. The implementation must therefore never derive a prefix from a Team ID and must never use a broad text replacement. The replacement profile's authenticated application identifier is the source of the concrete planned application ID, and its `ApplicationIdentifierPrefix` is a consistency check, including when it differs from the replacement profile Team ID.

For every target that has a rebased claim:

1. Require a concrete existing `application-identifier` whose suffix exactly equals that target's bundle identifier.
2. Extract `oldPrefix` from that value and validate it with the existing identity validation rules.
3. Require the replacement profile's `application-identifier` to be the exact concrete `<newPrefix>.<bundle-id>` value already required by the current `signing resign` profile parser. Preserve that parser's exact-one `ApplicationIdentifierPrefix` contract: derive `newPrefix` from the application identifier and require it to equal the profile's sole validated prefix. Wildcards may authorize specific optional entitlement values below, but wildcard application identifiers remain unsupported and are never materialized by this feature.
4. Require the profile's application identifier, team identifier, and certificate identity to pass the normal #2241 checks.
5. Derive each generic prefix-only candidate transformed value only from the target's own `oldPrefix` and the replacement profile's `newPrefix`. The KVS and graph rules below use their own authenticated destination sources instead of this generic substitution.

No v1 flag accepts user-supplied old or new prefixes. An override would make it possible to rewrite an unrelated prefix and would need a separate strict-input design. If a future override is considered, it must match the target's observed prefix and the profile's authenticated prefix rather than replacing those observations.

## Allowlist and value grammars

The allowlist is intentionally narrow. A key is not eligible because its value happens to contain a dot or a Team ID-looking string. Each key needs a documented grammar, a typed transformer, profile fixtures, and an exact post-signing verification test.

### Initial rewrite policy

| Entitlement key | Shape | Initial policy | Rule |
| --- | --- | --- | --- |
| `keychain-access-groups` | array of strings | allow, prefix-only | Transform each concrete `<oldPrefix>.<suffix>` item to `<newPrefix>.<suffix>`; preserve order and authorize the resulting array items against the replacement profile. |
| `com.apple.developer.ubiquity-kvstore-identifier` | string | allow only through its transfer-aware rule | Preserve an already-authorized value. Otherwise derive the destination prefix from the replacement profile's KVS entitlement, never from the App ID prefix or Team ID; transform only when that profile value proves one unambiguous concrete destination prefix. |
| `com.apple.developer.parent-application-identifiers` | array of strings | allow, paired graph-only | Rewrite only as part of a proven unique main-app/App-Clip pair; otherwise preserve an exact profile-authorized value. |
| `com.apple.developer.ubiquity-container-identifiers` | array of strings | unsupported in v1 | Enable only after signed/profile fixtures prove the exact prefix grammar and replacement profile authorization behavior. |
| `com.apple.developer.icloud-container-identifiers` | array of strings | unsupported in v1 | Treat the container identifier as a shared resource reference, not as a string to rewrite, until its signed grammar and ownership rules are proven for this command. |
| `com.apple.developer.icloud-container-development-container-identifiers` | array of strings | unsupported in v1 | Same boundary as production iCloud container identifiers; no speculative rewrite. |
| `com.apple.developer.associated-appclip-app-identifiers` | array of strings | allow, paired graph-only | Rewrite only as part of a proven unique main-app/App-Clip pair; never rewrite an arbitrary sibling bundle identifier. |

The first implementation ships only the rows marked allow. Unsupported v1 keys remain unchanged when the replacement profile authorizes their exact existing values and are refused otherwise. Adding a key later is an additive allowlist change with its own fixtures and output tests; paired graph claims enter together or remain deferred together.

### Never-rewrite policy

These claims are never changed by prefix substitution:

- `application-identifier`;
- `com.apple.application-identifier`;
- `com.apple.developer.team-identifier`;
- `get-task-allow`;
- arbitrary unknown entitlement keys;
- `com.apple.developer.icloud-services` and other capability selectors whose values are not documented prefix grammars;
- application groups, associated domains, Apple Pay identifiers, push/environment claims, pass identifiers, Network Extension claims, and similar capability values;
- `previous-application-identifiers`, unless a separate update-continuity design proves its semantics and profile authorization.

The required identity values come from the replacement profile and the normal target checks. An existing optional identity claim remains absent when it was absent from the signed input, even if the replacement profile contains a concrete or wildcard value. Rebasing cannot add a keychain group, iCloud claim, parent relationship, or any other optional access claim.

### Prefix-only transformation

For a generic prefix-only string, accept exactly a non-empty concrete value with the form:

```text
oldPrefix + "." + non-empty-suffix
```

The suffix is copied as an opaque identifier after key-specific structural validation; it is not parsed by splitting on every dot. For keychain groups, require valid UTF-8 and reject an empty suffix, leading or trailing whitespace, any Unicode whitespace or control code point, `*`, `/`, `\\`, or a NUL byte. Arrays must contain only strings. These rules are an input boundary, not a generic dotted-string parser. The transformed value is:

```text
newPrefix + "." + same-suffix
```

The KVS entitlement uses its own transfer-aware scalar parser. Require valid UTF-8 and exactly one non-empty prefix segment before the first dot plus a non-empty suffix after it; reject leading or trailing whitespace, any Unicode whitespace or control code point, `*`, `/`, `\\`, NUL, and unsupported types. The existing value comes only from the target's signed-entitlement document inventoried by the normal pipeline, never from the manifest or a user-supplied prefix. Its prefix can intentionally differ from both the App ID prefix and Team ID, including after an app transfer.

If the replacement profile authorizes the existing full concrete KVS value, preserve it exactly so a transfer can retain its storage namespace. Otherwise v1 requires the replacement profile's KVS entitlement to be one exact concrete value with the same validated suffix. That exact profile value is authoritative: rewrite only the prefix, then run normal profile authorization. A wildcard may authorize a candidate but cannot select the destination KVS namespace, so an unauthorized old value plus only wildcard KVS authorization fails closed. Missing, conflicting, malformed, or suffix-incompatible profile values also fail closed. The opt-in flag help and command documentation must state that changing the KVS value selects a different namespace and can make existing data inaccessible; no additional interactive prompt is introduced.

Plan KVS across the complete target graph as a one-to-one source-to-destination mapping. Every occurrence of one old full KVS value must resolve to the same planned full value, and distinct old full values must not converge on one planned full value. If shared source values split or distinct source namespaces collapse, reject the whole IPA before mutation.

Apply the same one-to-one graph rule to keychain groups. Every occurrence of one old full keychain group must resolve to one planned full value across all targets, and distinct old groups must not converge on one destination. Different old groups remain independent only while their planned destinations remain distinct. Reject a shared source group that splits, or distinct source groups that collapse, before mutation rather than silently changing the archive's keychain-sharing relationships.

For an array, apply the same rule to every element. First preserve any concrete value that the replacement profile already authorizes exactly or by a valid entitlement wildcard. Only an unauthorized value with the exact `oldPrefix.` prefix is a rebase candidate. A remaining third-prefix or unprefixed value, empty suffix, wildcard source value, non-string element, or ambiguous grammar fails closed. There are no silent partial rewrites.

An already-new-prefix or other value may remain unchanged only when it is concrete and the replacement profile authorizes that exact value or a valid wildcard pattern authorizes it. A list may therefore contain source-old values that transform and already-authorized values that remain unchanged. Every unauthorized non-source-prefix value is a refusal. This mixed-set rule is per element and is not an invitation to accept arbitrary values.

Preserve array order, element type, length, and duplicates exactly. Deduplication or duplicate rejection would change an existing signed input without establishing a stronger authorization boundary. Report each changed element by its original index so duplicate transformations remain auditable.

### Profile authorization after transformation

The pipeline must first calculate the complete candidate entitlement document, then ask the existing profile authorization routine whether each candidate is permitted. Authorization is checked against the transformed value, not the old value. Derive `newPrefix` from the profile's concrete planned application identifier; entitlement wildcard entries are authorization patterns only. Permit only a literal terminal wildcard with a non-empty dotted prefix and no other `*`, and require the concrete candidate to contain at least one character after that dotted prefix. Apply this validation to a scalar profile value and recursively to every string in a profile array before matching; malformed types or patterns make the claim unusable rather than being skipped. Multiple valid patterns are not ambiguous: a concrete candidate is authorized when at least one pattern permits it. Reject malformed patterns or a concrete candidate with no permitting exact value or pattern. A wildcard must never be emitted in the signed document.

The following cases all fail closed:

- the replacement profile omits a claim that exists in the signed input;
- a transformed value is not authorized by the replacement profile;
- the profile application identifier cannot produce one concrete target prefix, an entitlement wildcard is malformed, or no exact value or valid pattern permits the candidate;
- a required identity claim remains wildcard-only or otherwise non-concrete;
- a source value can neither be classified as a valid old-prefix rebase candidate nor preserved as an already-authorized concrete value;
- a candidate is accepted by a capability presence check but not by its value-specific profile entitlement.

The rebasing planner returns a new entitlement map and a separate ordered list of rewrite records. It must not mutate the existing entitlement map, profile object, archive, or output tree while evaluating authorization.

## Cross-target entitlement graph

Rebasing is a whole-IPA operation. The archive inventory is the graph's node set; each node includes target kind, relative path, bundle identifier, existing concrete application identifier, replacement profile, and planned new application identifier. Bundle identifiers and relative paths must be unique, and target ordering must be stable.

All target entitlement plans are built before the first generated entitlement file, embedded profile, or signed binary is written. The graph validator then checks references using the planned values:

- references must resolve to a discovered target in the same IPA;
- the referenced target kind must be valid for the claim;
- the referenced target's planned application identifier must be concrete;
- the existing signed claim must pass its type and grammar checks, and the replacement profile must authorize the planned candidate;
- a failure in any node or edge that is required for rebasing rejects the complete operation without a partial output IPA; an unproven existing relationship may remain unchanged when its replacement profile authorizes it exactly.

### App Clip relationships

App Clip relationships are rebased only as a paired graph change. A unique main application and App Clip target must have reciprocal concrete claims, and both replacement profiles must authorize their respective planned values. If pairing cannot be proven, an existing concrete claim remains unchanged when its replacement profile authorizes it; an unauthorized claim refuses the IPA before mutation. A one-sided rewrite is never permitted.

The implementation discovers a unique main-app/App-Clip pair, maps both existing concrete references to that pair, plans both new application identifiers together, and requires each replacement profile to authorize its own planned claim. Missing or ambiguous targets, multiple parents, arbitrary sibling references, mismatched pairs, and any one-sided rewrite invalidate rebasing; the existing claims are preserved together when each is already authorized unchanged, otherwise the operation refuses before mutation.

Other cross-bundle or sibling references remain unchanged and must be authorized unchanged by the replacement profile. If their relationship cannot be proven, preserve it when authorized unchanged; otherwise fail closed rather than signing a partially rebased IPA.

## Pipeline and verification order

The implementation should make the following phases explicit:

1. Inventory the IPA and validate every existing target entitlement before side effects.
2. Resolve each target's source prefix and replacement profile identity.
3. Plan required profile-derived identity claims and allowlisted local rewrites without changing the archive.
4. Validate every candidate against the replacement profile after transformation.
5. Validate the complete cross-target graph using planned application identifiers.
6. Sort and persist generated entitlements and profiles only after all plans pass.
7. Sign leaf-first using the existing explicit target and nested-code rules; do not rebase arbitrary framework, bundle, or XPC entitlements.
8. Repack the signed tree to a temporary IPA without publishing it.
9. Re-open and verify that exact temporary IPA against the generated entitlement documents, replacement profiles, signing identity, archive limits, and target inventory.
10. Atomically publish the already-verified IPA with the existing no-overwrite artifact contract; publication must not repack or otherwise change the verified bytes. Only after publication succeeds may the command hand the success result to the selected stdout renderer.

The verification comparison must use exact generated documents, not profile-subset semantics. A profile wildcard authorizes a concrete value; it does not make a different signed value acceptable. Verification is read-only with respect to the signed tree, generated documents, temporary IPA, and published artifact. A verifier may materialize files only inside a fresh, size-bounded private workspace created for that verification pass; it must clean that workspace afterward and must never feed materialized bytes back into the artifact being verified or published.

Rewrite records are collected from the plan, not reconstructed from logs or from a second potentially different parse of the packed IPA. One canonical comparator orders the flattened records by target relative path, bundle identifier, allowlisted key rank, scalar-before-array kind, zero-based array element index, old value, and new value. The index is considered only for array records; a scalar does not receive a synthetic index. Build one sorted slice with this comparator and pass it unchanged to JSON, table, and Markdown renderers rather than sorting separately. This makes output independent of map iteration and keeps all formats, tests, and retries reproducible.

## Result and audit output

The current `signing resign` command emits a structured `SigningResignResult`; it does not write a separate receipt file. This feature should extend that result additively rather than introduce a second persistence format or an overwrite-prone receipt flag.

When `--rebase-team-claims` is enabled, add a top-level flattened `entitlementRewrites` array, present even when no values changed. Omit the field entirely when the flag is absent so existing structured output keeps its shape. Represent presence explicitly in Go with a pointer to a slice (or an equivalent custom marshaler): nil means the flag was absent and omits the field, while a non-nil pointer to an empty slice emits `[]`. Renderers use the same presence distinction. The array contains only automatic changes made by the flag; normal profile-derived values such as application identifiers, team identifiers, and `get-task-allow` are not rebase records. One record represents one scalar rewrite or one array element, so mixed values and ordering are unambiguous:

```json
{
  "targetRelativePath": "Payload/App.app/PlugIns/Clip.appex",
  "bundleId": "com.example.Clip",
  "key": "keychain-access-groups",
  "elementIndex": 0,
  "from": "OLDPREFIX.com.example.shared",
  "to": "NEWPREFIX.com.example.shared"
}
```

`elementIndex` is omitted for a scalar claim and is zero-based for an array claim. Exported Go fields use the repository's camelCase JSON convention. Keep the existing schema version and add the field under the output stability rules; do not remove or rename existing fields. Rows for table and Markdown output must use the same deterministic ordering and clearly identify target, key, index, old value, and new value.

Exact old and new values are appropriate here because they are limited to explicitly allowlisted identifiers being transformed locally and are required to audit what was signed. The result must never contain passwords, private keys, profile source paths, raw profile plists, temporary paths, subprocess diagnostics, or unchanged arbitrary entitlement values. A failure diagnostic may identify target, key, element index, and a value-safe reason, but must not echo operational secrets.

When the flag is enabled but no value changes, `entitlementRewrites` is an empty array. When the flag is absent, the field is omitted. A failure before artifact publication does not publish an IPA or a success result. A renderer failure after publication, including a broken stdout pipe, returns a non-zero operational error and leaves the verified IPA at its published destination; the command must not delete a successfully published artifact merely because result delivery failed. No partial durable rewrite receipt exists. If a future workflow needs a durable file receipt, it must define destination preflight, mode, no-overwrite behavior, atomic publication, and redaction separately; that is outside this feature.

## Failure, compatibility, and lifecycle

The flag is opt-in and additive:

- existing invocations, help behavior, structured output shape, refusal text, and exit mappings remain unchanged when it is omitted;
- existing profiles, signed entitlements, and optional-claim omission rules are not broadened by merely adding the flag;
- all planning, profile authorization, and graph validation occur before signing mutations; the mandatory post-sign pass then verifies the exact repacked IPA before its no-replace atomic publication;
- a profile authorization failure is never converted into a warning or a best-effort rewrite;
- a missing grammar, ambiguous target relationship, malformed value, or unsupported claim remains fail-closed;
- the operation remains macOS-only and has no App Store Connect network side effects.

The initial release should not migrate old invocations or change the default. Any future interface change requires explicit help, documentation, output, and regression review. A future allowlist addition is a separate compatibility decision and must not make an existing invocation rewrite more values merely because a new binary is installed unless the opt-in flag is present.

## Implementation plan

Implement the feature against the current `signing resign` contract in these areas:

1. `internal/cli/signing/signing_resign.go`: add `RebaseTeamClaims` to command options, bind the flag, and include the exact help text and plumbing.
2. `internal/cli/signing/signing_resign_entitlements.go`: add typed allowlist metadata, source-prefix parsing, scalar and list planners, duplicate-preserving mixed-prefix handling, and post-transform profile authorization. Return rewrite records as data, not side effects.
3. `internal/cli/signing/signing_resign_archive.go`: expose a stable target graph and old/new application-identifier lookup, or place the equivalent helper in the pipeline package without duplicating archive discovery.
4. `internal/cli/signing/signing_resign_pipeline.go`: plan every target before writes, validate graph edges, propagate rewrite records, retain exact generated-document verification, and keep nested non-target code outside the rebase scope.
5. `internal/asc/output_signing_resign.go`: add exported rewrite result types and deterministic table/Markdown rows while preserving the existing result fields.
6. `internal/asc/output_signing_resign_test.go`: assert JSON field shape, empty and non-empty arrays, table headers/rows, Markdown rows, and ordering.
7. `internal/cli/signing/signing_resign_test.go`: add unit, command-boundary, planning, graph, authorization, ordering, and no-mutation coverage.
8. `internal/cli/signing/signing_resign_privacy_test.go`: assert that success and failure output do not leak credentials, profile paths, raw profiles, or temporary paths, and that success results contain values only for allowlisted automatic rewrites. Existing refusal diagnostics may continue to identify the offending claim and value when the flag is absent.
9. `commands/signing.mdx` and `docs/design/signing-ipa-resign.md`: update only after the #2241 command surface exists on the target branch. Document the opt-in flag, allowlist, graph rules, output records, and refusal examples.

`internal/cli/signing/signing_resign_manifest.go`, `internal/cli/signing/signing_json.go`, and `internal/asc/output_registry_init.go` should remain unchanged unless implementation discovers a real schema or renderer registration requirement. The standalone design PR changes only this design document.

## RED-GREEN test matrix

Tests should begin with the smallest failing assertion at the command or planner boundary, then reach green with the narrow implementation. The existing wildcard test named `TestBuildSigningResignEntitlementsKeepsConcreteValuesForWildcardProfileClaims` uses already-new values and does not prove old-prefix rebasing; add an explicit old-prefix case rather than treating it as coverage.

### Command and compatibility

- `TestSigningResignCommandExposesRebaseTeamClaimsFlag`: help shows the long-form flag and its default-off meaning.
- `TestSigningResignCommandPassesRebaseTeamClaimsOption`: the flag reaches the execution options; no flag leaves the option false.
- `TestSigningResignCommandRejectsInvalidFlagShapes`: positional arguments, unsupported values, and platform-ineligible use remain usage errors with stderr diagnostics and exit 2.
- A no-flag regression test runs the current unauthorized old-team claim case and asserts the existing refusal/remediation contract is unchanged.

### Prefix and allowlist behavior

- `TestBuildSigningResignEntitlementsRequiresExplicitRebaseOptIn`: an old-prefix keychain claim and an exact-profile-destination KVS claim fail without the flag and transform with it.
- `TestRebaseSigningResignClaimUsesProfileApplicationIdentifierPrefix`: a legacy source prefix and a different replacement Team ID still use the profile App ID prefix for keychain claims.
- `TestRebaseSigningResignKVSUsesProfileKVSPrefix`: KVS preserves an authorized transfer prefix and otherwise uses only the unambiguous prefix authenticated by the replacement profile's KVS entitlement.
- A no-flag KVS regression case proves that an existing full value already authorized by the replacement profile remains unchanged; the opt-in rewrite case instead starts with an unauthorized source value and an exact concrete profile destination with the same suffix.
- A multi-target KVS regression gives two targets the same old full value but replacement profiles that preserve or propose different planned values, and asserts that the complete IPA fails before any signing-tree mutation.
- A KVS collision regression gives two distinct old full values that would resolve to one planned destination and asserts the same pre-mutation refusal.
- A multi-target keychain regression gives two targets the same old full group but profiles that preserve or produce different planned values, and asserts the same pre-mutation whole-IPA refusal.
- A keychain collision regression gives distinct old full groups that would resolve to one planned group and asserts that the planner cannot create a new sharing relationship.
- `TestRebaseSigningResignClaimRejectsUnauthorizedThirdPrefix`: an unauthorized third prefix fails closed, while an unchanged value already authorized by the replacement profile is preserved.
- `TestRebaseSigningResignClaimRejectsMalformedUnprefixedAndWildcardValues`: empty suffixes, unprefixed values, and wildcards fail closed.
- `TestRebaseSigningResignClaimPreservesListOrderAndShape`: old and already-new values remain in original order and retain array shape.
- `TestBuildSigningResignEntitlementsMixedOldNewTeamClaims`: old values transform, authorized new values remain, and unrelated values fail.
- A duplicate-element test asserts exact preservation of duplicates, order, array shape, and per-index rewrite records.
- `TestBuildSigningResignEntitlementsStillOmitsAbsentOptionalClaimsWithRebase`: the flag cannot add absent optional claims from the profile.

### Profile authorization and pipeline safety

- `TestRebaseSigningResignClaimRequiresReplacementProfileAuthorization`: wildcard profile authorization accepts a concrete rebased value; concrete profiles require exact membership.
- A missing profile key, non-authorized transformed value, wildcard-only required identity, malformed wildcard, and candidate matching no valid profile pattern each fail before signing.
- The wildcard regression fixture uses an old-prefix keychain value with `NEWPREFIX.*` profile authorization, proving no-flag refusal and opt-in concrete rewriting rather than reusing an already-new value. Its KVS fixture separately uses an exact concrete profile destination because a wildcard cannot select a KVS namespace.
- `TestPrepareSigningResignTreePlansAllRewritesBeforeMutation`: a later target or edge failure leaves no generated entitlement, embedded profile, or output-tree mutation.
- `TestRebaseSigningResignRewriteOrderingIsStable`: map iteration order cannot change records or generated documents.
- `TestRebaseSigningResignPostSignDocumentMatchesRewrittenEntitlements`: verification compares the exact concrete transformed document and rejects a different authorized subset.

### Cross-target graph

- App Clip relationship tests prove that authorized unpaired claims remain unchanged, unauthorized claims refuse before mutation, and paired claims rewrite together.
- Paired graph tests cover two-way mapping, one-sided-rewrite rejection, unique-pair enforcement, and arbitrary-sibling-reference rejection.

### Output and privacy

- `TestSigningResignResultReportsEveryEntitlementRewrite`: JSON contains target, bundle, key, optional index, exact old value, and exact new value for every rewrite.
- Renderer tests assert deterministic table and Markdown rows, an empty array when the flag is enabled without changes, and omission of the field when the flag is absent.
- Privacy tests inject a password, profile path, temporary path, and non-allowlisted entitlement. Credentials and paths never appear in success or refusal output; the non-allowlisted value never appears in a success result, while the no-flag refusal keeps the existing offending-claim diagnostic contract.
- Built-binary checks assert stdout/stderr separation, exit codes, and no duplicate error rendering.

Focused signing tests should run first with `ASC_BYPASS_KEYCHAIN=1`. Before implementation is opened for review, run the repository's required serialized gates: `make build`, `make format`, `make check-docs`, `make lint`, and `ASC_BYPASS_KEYCHAIN=1 make test`. A macOS fixture with genuinely signed nested targets should verify codesign and concrete entitlements; it must remain local and read-only with respect to App Store Connect.

## Alternatives and unresolved gates

Keeping the #2241 refusal forever is the safest option but leaves maintainers to rewrite every claim manually. It remains the default and is the fallback for every ambiguous case.

Accepting `--old-prefix` and `--new-prefix` would make the workflow appear flexible, but it delegates a security-sensitive identity decision to an unchecked string and can rewrite a different application's claims. Deriving from the signed target and authenticated replacement profile is narrower and auditable.

Rewriting every dotted string or copying the complete profile entitlement set would be shorter, but would grant or alter capabilities that were not present in the signed input. Typed allowlist planners preserve the least-privilege boundary.

Before coding, maintainers must resolve these gates with fixtures or retain the stated fail-closed defaults:

- whether iCloud and ubiquity container identifiers are genuinely prefix-scoped in the signed values used by this command;
- whether a later paired App Clip graph implementation has enough signed fixtures to enable both relationship claims together;
- #2249 remains the canonical design issue and #2251 is the implementation follow-up; implementation work should link both issues.

No gate may be resolved by making the flag broader or by treating a passing profile capability-presence check as value authorization.
