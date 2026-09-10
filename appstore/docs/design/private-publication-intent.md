# Crash-recoverable private publication intent

Status: implementation contract for the agent-native distribution
orchestrator. This does not change the public `asc distribute publish` command.

## Placement and invocation

The seam lives in `internal/distribution` with an adapter in
`internal/cli/distribute`. The `asc distribute apply` and `resume` executors call
it in process after preparing an immutable distribution bundle. There are no
new public flags, output formats, prompts, or App Store Connect API calls.
Failures return a non-zero exit status through the calling command. Normal
stdout and the redacted receipt never contain bearer URLs.

## State machine

Before the first remote `PUT`, preparation validates the bundle, private URL
lifetime, credential expiry, and signing-profile expiry. It chooses one random
link identifier, derives the content-addressed IPA key and stable manifest/page
keys, presigns all three downloads once, and generates the exact manifest and
install-page bytes. The adapter binds that intent to the normalized endpoint,
download endpoint, region, addressing style, bucket, prefix, bundle evidence,
and local artifact paths. It writes the complete intent create-only as a
bounded, owner-private mode-0600 JSON artifact, fsyncs the file, atomically
renames it without replacement, and fsyncs the parent directory where the
platform exposes directory syncing.

Execution reloads and validates that exact intent, then converges in the fixed
order IPA, manifest, page. Each object is accepted only when its key, digest,
size, and content type match. Existing S3 `Ensure` behavior resolves an
ambiguous `PUT` response with exact `HEAD` evidence. Execution never chooses a
new key or presigns a replacement URL. Only after all three objects and their
download URLs verify does it atomically publish the redacted receipt.

`resume` uses the same protected intent. A crash after intent persistence,
between any two objects, after an ambiguous response, or after the final object
but before the receipt therefore repeats idempotent `Ensure` operations against
the original destinations. Tampered state, a different bundle or destination,
an expired install link, insufficient saved credential lifetime, or a signing
profile that no longer covers the download deadline fails closed. Expired
intent is not refreshed automatically because doing so would create a second
publication identity.

## Compatibility and alternatives

The standalone publisher keeps its current one-shot public API and recovery
artifacts. Making all publication globally transactional would change its
existing on-disk contract. Persisting state after the uploads is insufficient:
a process can die after a successful remote write but before learning its
result. Deriving URLs again during recovery is also unsafe because credentials
and signatures can change. A separately versioned agent-only intent provides
the required durable boundary without migrating existing callers.

## Verification

Focused tests inject failures immediately after intent persistence, after each
successful `Ensure`, and after an ambiguous `Ensure`. Recovery must use the same
three keys, exact generated bodies, and saved URLs with no new random identifier
or presign call. Additional tests reject intent, destination, bundle, and expiry
tampering; assert mode-0600 create-only persistence; and verify bearer URLs are
absent from structured results and receipts.
