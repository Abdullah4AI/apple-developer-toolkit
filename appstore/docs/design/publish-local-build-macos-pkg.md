# macOS PKG support for local-build publish

## Problem

`asc publish testflight` and `asc publish appstore` already accept
`--platform MAC_OS` in local-build mode and archive with the macOS destination.
The export and upload stages still force an `.ipa` destination and reserve the
upload as `com.apple.ipa`, so the advertised macOS path cannot complete.

## Contract

- Local builds for `MAC_OS` export a `.pkg`; other platforms continue to export
  an `.ipa`.
- `--pkg-path` is a local-build-only destination override for `MAC_OS`.
- `--ipa-path` and `--pkg-path` are mutually exclusive. `--ipa-path` is rejected
  for `MAC_OS`, and `--pkg-path` is rejected for other platforms.
- The default macOS artifact is
  `.asc/artifacts/<scheme>-MAC_OS-<version>-<build>.pkg`.
- The exported PKG is validated as a non-empty regular, non-symlink file before
  creating an upload reservation with UTI `com.apple.pkg`.
- JSON and rendered stage output report `pkgPath` / `pkg_path` for macOS and do
  not claim an IPA was produced.

An explicit ExportOptions plist remains supported. Generated manual signing for
macOS remains unavailable because Xcode signing-asset synthesis currently only
supports iOS and tvOS; local-build publish rejects that combination before the
archive side effect and directs callers to an explicit plist.

## Compatibility

The iOS, tvOS, and visionOS IPA paths are unchanged. The previous
`--platform MAC_OS --ipa-path` combination could not produce a valid publishable
artifact, so it now returns a usage error with the PKG migration path.
