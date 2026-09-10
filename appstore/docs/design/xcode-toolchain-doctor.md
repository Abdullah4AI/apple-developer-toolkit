# Read-only Xcode toolchain doctor

## Placement and command shape

Issue #2228 adds a leaf beneath the existing local `asc xcode`
group:

```text
asc xcode doctor [--developer-dir PATH] [--sdk SDK] [--output json|table|markdown] [--pretty]
```

`asc xcode --help` exposes `inject`, `build`, `archive`, `export`,
`export-options`, `validate`, `version`, and the `doctor` leaf.
The doctor command is local-only and does not require App Store Connect
credentials or an API request.

The effective developer directory is selected in this order:

1. explicit `--developer-dir`;
2. non-empty `DEVELOPER_DIR`;
3. `xcode-select --print-path`.

The flag is an inspection override. It must not call `xcode-select --switch`,
invoke `sudo`, change the parent process environment, or write host state.
Every child probe receives the resolved candidate as `DEVELOPER_DIR` so the
checks cannot silently use a different Xcode selection.

## Checks and output

The command checks the selected directory, resolves `xcodebuild` with
`xcrun --find` under that candidate, validates that the returned absolute
executable is inside the selected developer directory, and runs `-version` on
that exact path. This prevents a caller `PATH` shadow from supplying the
version/build while the report names a different toolchain. It optionally
resolves one SDK with `xcrun --sdk SDK --show-sdk-path`. Xcode application-root
normalization also uses the canonical target, so an extensionless symlink is
treated as the application it points to while the report keeps the
operator-selected spelling. Beta-looking paths
are classified from the canonical physical selected toolchain path, so a
neutral-named application symlink still produces an advisory warning for a
beta target. Command Line Tools detection also uses that canonical path, so a
neutral symlink to the Command Line Tools package cannot be mistaken for an
Xcode application. If canonicalization fails, beta remains unknown and the
doctor fails closed. Command Line Tools-only paths are a deterministic
failure even if mocked probes happen to return successful Xcode-shaped output.

The report uses the exported computed-output camelCase JSON contract. The
top-level status is `ok`, `warn`, or `fail`. Checks have stable names, status,
message, and optional path fields. The selected source is `flag`, `environment`,
or `xcode-select`. Beta is omitted from JSON when developer-directory
selection or normalization fails; human output shows that state as `unknown`.
JSON is emitted on stdout; bounded diagnostics remain on stderr. Table and
Markdown render the same stable data.

Without `--output`, the report defaults to `table` in an interactive terminal
and minified `json` for pipes or CI. An explicit `--output` value always takes
precedence.

Exit behavior is:

- `0` for `ok` and `warn` reports;
- `1` for an unavailable or unusable directory, Xcode tool, or requested SDK;
- `2` for invalid flags, empty explicit values, positional arguments, or an
  unsupported output format.

The command remains visible on every platform for consistent help and docs, but
execution is macOS-only and must retain the existing local Xcode unsupported
platform error contract.

## API and compatibility

This change has no App Store Connect API surface, OpenAPI operation, request
schema, or response resource. It is entirely a local macOS process and path
inspection feature.

Existing archive, export, validate, build, version, signing, profile, and
device behavior remains unchanged. In particular, this issue does not add a
developer-directory flag to those commands or select a toolchain for future
commands. A future change can consume this proven resolution contract without
recreating the diagnostics.

The command must not install or update Xcode, SDKs, simulator runtimes, or
first-launch components. It must not inspect or print credentials, complete
environment contents, or unbounded subprocess logs. Any telemetry added for
the new command records only aggregate outcome and whether selector flags were
present; path and SDK values remain local.

## Verification plan

RED coverage starts at the CLI boundary for command registration, help, flags,
output, and exit behavior, followed by core tests for selection precedence,
version parsing, path normalization, child environment propagation, exact
xcrun-resolved executable selection, PATH-shadow/mismatched-resolution
and symlink-escape failures, Command Line Tools failure, probe failures, beta
warnings, SDK checks, cancellation, and bounded diagnostics.
All subprocess and filesystem interactions use injectable seams, so tests do
not require a real Xcode installation.

The focused loop is:

```bash
go test ./internal/xcode ./internal/cli/xcode ./internal/cli/cmdtest
```

The completed behavior must also pass the required repository gates:

```bash
make build
make format
make check-docs
make lint
ASC_BYPASS_KEYCHAIN=1 make test
```

On macOS, manually verify the default selection, an explicit application or
developer directory, an SDK probe, a beta-path warning when available, and
that the `xcode-select --print-path` result is unchanged before and after each
invocation.

## Alternatives

Adding a package-manager-like installer or global Xcode switch would create
privileged, destructive host behavior and a large lifecycle surface. This
design keeps selection explicit and read-only while still making the effective
toolchain observable.

Adding a developer-directory flag to every existing local command first would
duplicate validation and expand the compatibility surface. A standalone doctor
establishes one reusable, testable contract before any command-specific
integration.
