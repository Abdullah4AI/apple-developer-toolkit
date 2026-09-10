# Deterministic Xcode signing settings plan/apply

## Placement and current behavior

This change adds a `signing` subgroup beneath the existing
`asc xcode` command. It is intentionally separate from `asc signing`, which
manages App Store Connect signing resources and protected local signing files.
The new command changes only the local Xcode project configuration that tells
Xcode how to consume those resources.

Current 4.11 behavior has no persistent project-signing command:

- `asc xcode export --signing-style ... --team-id ...` controls generated
  export options for one export and does not edit the project.
- `asc xcode build` and `asc xcode archive` pass build settings to one
  `xcodebuild` invocation.
- `asc xcode version` structurally edits version settings, but has no signing
  settings contract.
- `asc signing fetch`, `sync`, `run`, and `reconcile` operate on certificates,
  profiles, devices, or isolated commands rather than project settings.

Apple documents target/project build-setting precedence and the code-signing
settings used by this command in the [build settings reference][apple-build]
and [target build settings guide][apple-target].

## Command contract

The command is additive:

```text
asc xcode signing plan \
  --project PATH \
  --settings-file PATH \
  [--state-dir PATH] [--allow-external-xcconfig] [--overwrite] \
  [--output json|table|markdown]

asc xcode signing apply \
  --plan PATH [--allow-external-xcconfig] --confirm \
  [--output json|table|markdown]
```

`--project` is required and selects exactly one `.xcodeproj`. A workspace is
not accepted because the build settings are owned by the referenced project
file. The settings file is strict JSON with `schemaVersion: 1` and explicit
target/configuration entries. The supported settings are the allowlist below:

- `CODE_SIGN_STYLE`
- `DEVELOPMENT_TEAM`
- `CODE_SIGN_IDENTITY`
- `PROVISIONING_PROFILE_SPECIFIER`
- `PROVISIONING_PROFILE`
- `CODE_SIGN_ENTITLEMENTS`
- `PRODUCT_BUNDLE_IDENTIFIER`

`null` removes a direct assignment only for the optional identity, profile, or
entitlements settings. It is invalid for signing style, team, or bundle ID.
Values must be static and cannot contain NUL, newline, comment syntax, or
build-setting expressions. The command does not accept arbitrary build
settings, shell snippets, or paths from a remote response.

The default state directory is `.asc/xcode/signing`. Planning writes a
mode-0600 `plan.json`; applying writes a mode-0600 `receipt.json`. Planning
does not mutate the project or settings input. Applying requires `--confirm`.

## Resolution and mutation model

The implementation reuses the structured pbxproj and xcconfig model already
used by `internal/xcode/version_project.go` and `internal/xcode/version_xcconfig.go`.
It must not introduce a second project parser. For each explicit target and
configuration, it resolves settings in Xcode's effective order:

1. target-level pbxproj settings;
2. target-level xcconfig and its include graph;
3. project-level pbxproj settings; and
4. project-level xcconfig and its include graph.

An existing direct assignment is changed in place. An xcconfig assignment is
changed only when every consumer of that file is selected. If an xcconfig is
shared with an unselected consumer, the planner adds a narrow target/config
override instead of broadening the mutation. Missing settings are added at the
selected target/configuration level.

Build-setting references are expanded against the explicit pbxproj and
xcconfig layers first. Only when no layer assigns the referenced name does the
resolver fall back to the implicit context Xcode derives from the project's own
location, so an existing assignment always keeps Xcode's precedence. The
supported implicit variables are:

- `SRCROOT`, `SOURCE_ROOT`, and `PROJECT_DIR`: the directory containing the
  selected `.xcodeproj`;
- `PROJECT_FILE_PATH`: the absolute path of the selected `.xcodeproj`;
- `PROJECT_NAME`: that bundle's name without its extension; and
- `TARGET_NAME`: the target owning the configuration being resolved. It is
  undefined for a project-level configuration, which no single target owns.

Every other implicit Xcode variable requires a build context and stays
unresolved. `CONFIGURATION`, `PLATFORM_NAME`, `SDKROOT`,
`EFFECTIVE_PLATFORM_NAME`, and `BUILT_PRODUCTS_DIR` are examples: this command
never invokes `xcodebuild`, so guessing them would misreport which file the
plan inventoried. A resolved implicit path is still subject to the same rooted,
no-follow containment and artifact-alias checks as a literal path, so it cannot
name a file outside the selected project root without being reported.

Conditional-only, divergent, unresolved, malformed, ambiguous, or missing
values are blockers. No unsupported expression is guessed or flattened.
External xcconfig writes are refused by default. `--allow-external-xcconfig`
must be supplied to both plan and apply, and the exact path and digest are
bound into the plan. Final symlinks remain forbidden.

Any unauthorized external xcconfig is a hard planning failure, not a
`ready: false` artifact. Its unread contents could define an entitlement value
or reference an entitlement path that cannot be inventoried without violating
the authorization boundary. The planner therefore does not publish either a
distinct or an overwrite-enabled plan artifact until the source is explicitly
authorized with `--allow-external-xcconfig`.

## Plan and apply artifacts

The plan contains versioned, deterministic JSON with:

- command, schema version, generation time, and a canonical content hash;
- project and settings-file paths;
- SHA-256 digests for every source file that may be written;
- explicit target/configuration/setting changes with set/remove operations;
- source provenance (`pbxproj` or `xcconfig`); and
- sorted blockers and warnings.

The hash excludes only `generatedAt` and the hash field itself. It includes
the desired settings, selected scope, old values and sources, source file
digests, output paths, and external-file authorization. A valid but blocked
plan is emitted with `ready: false`; malformed input is an exit-2 usage error.

Apply strictly decodes and verifies the plan, checks every input digest,
re-resolves the selected settings, stages all writes, reparses the staged
project/configuration files, and commits with rooted atomic no-follow writes.
It rejects stale or redirected plans before writing. A later write failure
restores earlier writes only while their current identity and bytes still match
this transaction, and reports rollback failure separately if recovery is not
complete. Apply revalidates every source both before and after receipt
publication and verifies the created receipt through its retained identity
before reporting success. Receipt removal uses the same rooted no-follow and
digest checks;
because the portable rootfs API has no compare-and-unlink primitive, a final
path replacement after the last identity check is still a residual concurrency
window. In that case the current file is preserved and the apply reports
rollback uncertainty rather than deleting an unverified replacement.

No command in this change calls App Store Connect, modifies certificates or
profiles, imports keychain material, signs binaries, or executes a shell.

## Compatibility and verification

Existing Xcode and signing commands and their JSON contracts remain unchanged.
Project/xcconfig planning remains cross-platform because it requires no Xcode
process. Apply additionally requires native identity-coupled replace and
remove support; it fails closed on Windows before mutation until the rooted
filesystem layer has handle-backed primitives there. macOS live validation
uses `xcodebuild -list`, `xcodebuild -showBuildSettings`, and a no-signing
archive syntax check.

Tests begin at the command boundary, cover strict input and output contracts,
then exercise direct/project/xcconfig precedence, shared-file safety, stale
plans, symlinks, atomic rollback, and no-op receipts. The implementation must
run the repository's complete build, format, docs, lint, and
`ASC_BYPASS_KEYCHAIN=1 make test` gates.

## Alternatives

Editing raw pbxproj lines would be shorter but would be formatting-sensitive,
would not understand target/project inheritance, and could silently modify a
shared configuration. Reusing the repository's existing structured project
parser and lossless xcconfig editor keeps the new command aligned with the
version editor while limiting its writable settings to a small audited
allowlist.

[apple-build]: https://developer.apple.com/documentation/xcode/build-settings-reference
[apple-target]: https://developer.apple.com/documentation/xcode/configuring-the-build-settings-of-a-target/
