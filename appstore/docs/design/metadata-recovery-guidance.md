# Metadata recovery guidance

## Decision

`asc metadata validate` remains a directory-based command that is offline by
default. The explicit `--check-urls` flag can opt into bounded URL
destination checks. The command does not accept `--app` or `--version`; those
unsupported flags produce a targeted recovery message that points to `--dir`
and shows `asc metadata pull` when local metadata must be fetched first.

`asc metadata pull --version` remains required. The CLI does not select a
version implicitly because that could fetch or overwrite metadata for the wrong
App Store version. A missing value now returns a concise usage error and points
to `asc versions list --app "APP_ID" --paginate` for discovery, so versions on
later API pages are included.

## Compatibility

No flag is accepted and ignored. Both failures retain usage exit code 2, empty
stdout, and their existing telemetry classes. The pull preflight still runs
before authentication or HTTP. Explicit help and successful metadata output are
unchanged.

## Verification

Command-level tests cover both unsupported validation flags, fixed recovery
commands, redaction of following flag values, missing-version diagnostics,
canonical telemetry, no full help dump, and the existing required-input cases.
