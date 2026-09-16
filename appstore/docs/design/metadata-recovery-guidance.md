# Metadata recovery guidance

## Decision

`asc metadata validate` remains a directory-based command that is offline by
default. The explicit `--check-urls` flag can opt into bounded URL
destination checks. The command does not accept `--app` or `--version`; those
unsupported flags produce a targeted recovery message that points to `--dir`
and shows `asc metadata pull` when local metadata must be fetched first.

`asc metadata pull --version` is optional. When it is omitted, the CLI first
selects the app's newest active editable App Store version, then a
developer-removed-from-sale version, and finally its newest live version. The
selected version and platform are reported on stderr. If the
chosen tier has candidates on several platforms, the command stops before any
file write and lists the `--platform` values that can disambiguate the request.
If no editable or live version exists, it also stops before writing files.
Passing `--version` preserves explicit selection.

## Compatibility

No flag is accepted and ignored. Both unsupported validation-flag failures
retain usage exit code 2, empty stdout, and their existing telemetry classes.
The pull preflight still runs before authentication or HTTP for locally invalid
inputs. Explicit help and successful metadata output remain unchanged, apart
from the selection note when the default is used.

## Verification

Command-level tests cover both unsupported validation flags, fixed recovery
commands, redaction of following flag values, default-version selection,
ambiguity and no-match no-write behavior, canonical telemetry, no full help
dump, and the remaining required-input cases.
