# Xcode PKG validation

## Problem

`asc xcode validate` wraps Apple's server-side `altool --validate-app` check but
only accepts an IPA. Apple's current tool also validates macOS PKG archives, so
the command unnecessarily blocks the artifact produced by a macOS export.

## Contract

- Exactly one of `--ipa` or `--pkg` is required.
- `--ipa` continues to infer iOS, tvOS, or visionOS from IPA metadata.
- `--pkg` requires a `.pkg` file and invokes validation with the macOS type.
- JSON and rendered output report only the selected artifact path plus the
  existing `validated` result.
- Authentication flags and server-error classification are unchanged.

Invalid combinations and extensions remain usage errors. Validation is
read-only with respect to App Store Connect resources, but it sends the local
artifact to Apple's validation service.

## Verification

Focused tests cover command routing, output, exact-one validation, extensions,
and the `altool` arguments. Repository build, format, documentation, lint, full
tests, and branch review remain the release gates.
