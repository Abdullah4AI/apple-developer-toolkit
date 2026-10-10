# Portable iOS build and package workflow

Add `asc builds compile` and `asc builds package`, using explicitly installed xtool
and rcodesign. Preserve existing Xcode archive/export and fully verified
macOS re-signing contracts. xtool consumes a Swift package, not an Xcode scheme;
its converted product is not an Xcode archive.

## Tooling and license boundary

ASC invokes user-installed tools and does not bundle, download, or redistribute
Xcode, Apple SDKs, or Swift toolchains. It does not operate a hosted build service.
Users obtain their toolchain and signing materials and remain responsible for
the agreements applicable to their account and build environment. Document
[xtool's author's explanation](https://forums.swift.org/t/xtool-cross-platform-xcode-replacement-build-ios-apps-on-linux-and-more/79803/3)
of this boundary and the option to use macOS CI for a final distribution build.
Supplying a wrapper is separate from operating a user's build; it does not waive
Apple's SDK restrictions or establish permission, licensing compliance, or Apple
acceptance.

## Command contract

Compile accepts a prepared `--package-path`, `--product`, exact new `--app-path`,
`--platform device|simulator`, and `--configuration debug|release`. The installed
toolchain owns Swift/Apple SDK setup. Compiler output goes to stderr;
JSON/table/Markdown receipts go to stdout. Simulator metadata is corrected and
the app signed ad hoc.

Package accepts one built device app, an exact new IPA destination, and either
explicit `--ad-hoc` test signing or local PKCS#12 identity and Apple provisioning
profile. Real signing validates Apple CMS profile trust, expiry, exact bundle ID,
certificate/profile binding, and requested entitlement authorization using
existing portable validators. The validated identity and password reach rcodesign
through pipes, so ASC never writes them or a decrypted private key to disk. Reject
nested code until it has an explicit signing plan. Do not provision resources, discover credentials, or upload.

rcodesign's verification does not provide Apple's complete bundle verification.
The receipt reports that boundary; real Apple processing remains a separate
`asc builds upload --wait` step. Ad hoc IPAs are local pipeline tests and cannot
be published to Apple. Preserve create-only outputs, rooted reads/writes,
bounded copies, cancellation, and an allowlisted child environment.

## Upstream compile behavior

xtool's [build operation](https://github.com/joshuaswarren/xtool/blob/f0a1f90efdbb0dc023e276ff529da92618da7a87/Sources/XToolSupport/DevCommand.swift#L74-L95)
can apply an ad hoc signature for configured entitlements without `--sign`, so
device compile receipts report `signingType: unknown`.

xtool's [SDK readiness operation](https://github.com/joshuaswarren/xtool/blob/f0a1f90efdbb0dc023e276ff529da92618da7a87/Sources/XToolSupport/SDKCommand.swift#L142-L190)
can rebuild an outdated SDK from an existing local Xcode copy. Help and docs
describe that backend-owned maintenance and recommend a compatible pinned
xtool/SDK pair.

## Compiler output freshness contract

This backend requires xtool to publish a fresh product directory. The tested
xtool revision is `joshuaswarren/xtool` at
`f0a1f90efdbb0dc023e276ff529da92618da7a87`.
Its [Packer.swift](https://github.com/joshuaswarren/xtool/blob/f0a1f90efdbb0dc023e276ff529da92618da7a87/Sources/PackLib/Packer.swift#L123-L125)
removes the prior `xtool/<product>.app` and persists a separately staged bundle
at that path after packing. ASC pins the old directory handle so inode reuse is
not mistaken for new output, and rejects an unchanged directory: a successful
no-op or a build of another configured product must not publish an old app.
xtool versions that publish in place need a separately tested freshness
contract before support is claimed.

## Empty entitlement override

rcodesign imports entitlements from an existing signature when no explicit
entitlement setting is provided. Package always supplies a valid XML
dictionary, including an empty dictionary to clear previous claims.
