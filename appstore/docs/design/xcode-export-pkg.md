# Xcode PKG export

## Placement and current behavior

`asc builds upload` already accepts `--pkg`, sends `com.apple.pkg`, and selects
the macOS platform. The missing layer is local export: `asc xcode export`
requires an IPA destination, searches only for `*.ipa`, and reports only
`ipa_path`. Its direct-upload mode also requires an IPA path even though
`xcodebuild` does not create a local artifact for `destination=upload`.

This change extends the existing local Xcode command. It does not add an App
Store Connect API operation or alter `asc xcode archive`; Xcode archives remain
`.xcarchive` bundles and become uploadable artifacts only during export.

## Public command shape

Local exports accept exactly one destination:

```text
asc xcode export --archive-path MacApp.xcarchive --pkg-path MacApp.pkg
asc xcode export --archive-path App.xcarchive --ipa-path App.ipa
```

`--pkg-path` must end in `.pkg`. It shares `--overwrite`, generated or explicit
ExportOptions.plist handling, timeouts, and xcodebuild argument passthrough with
IPA export. The generated `release-testing` method remains IPA-only and is
rejected when combined with `--pkg-path`. The pinned manual-signing generator
supports only iOS and tvOS archives, so generated manual options are also
rejected for PKG output with guidance to provide an explicit plist. Automatic
generation and explicit manual plists remain supported.

With an explicit plist containing `destination=upload`, or with `--wait`, both
artifact paths are optional because Xcode uploads directly. If a legacy caller
still supplies an artifact path in direct-upload mode, it is accepted but not
created, preflighted, or reported as a produced artifact.

No command prompts. Invalid flag combinations and extensions are usage errors
with exit code 2. Data stays on stdout and diagnostics stay on stderr.

## Export and output contract

Local PKG export runs `xcodebuild -exportArchive` in an owned temporary
directory, requires exactly one root-level `*.pkg`, verifies that it is a
non-empty regular file, and publishes it at the requested path without
silently replacing an existing destination. PKG metadata comes from the input
archive because an installer package does not expose the existing IPA metadata
layout.

Structured output adds optional `pkg_path`; existing `archive_path`,
`ipa_path`, bundle ID, version, and build number fields remain compatible. IPA
exports are unchanged. Direct upload continues to return archive metadata and
an empty `ipa_path`, without adding a local path that does not exist.

## RED-GREEN and verification

Focused coverage establishes:

- CLI acceptance and propagation of `--pkg-path`;
- mutual exclusion of IPA and PKG destinations;
- exact-path PKG publication and archive-derived metadata;
- rejection of missing, multiple, empty, or unsafe PKG output;
- direct upload without a placeholder artifact path or parent-directory write;
- preserved IPA behavior and help/output compatibility;
- built-binary usage exit codes and stdout/stderr separation.

Verification uses focused package tests, generated command docs, a built CLI,
and the repository build, format, docs, lint, full-test, and review gates. A
real signed archive export is optional because it depends on local signing
assets; no App Store Connect mutation is required for this change.

## Alternatives

Producing a PKG during `xcode archive` would misrepresent Xcode's archive
contract and conflate two stages. Requiring callers to find Xcode's temporary
PKG themselves would lose deterministic output and overwrite guarantees.
Keeping a dummy `--ipa-path` for direct upload preserves syntax but creates a
false local-artifact contract.
