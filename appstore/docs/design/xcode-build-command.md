# First-class local Xcode build command

## Placement and current behavior

`asc xcode build` is a leaf beneath the existing local `asc xcode`
group. It wraps the ordinary `xcodebuild ... build` action; it does not archive,
export, upload, or call App Store Connect.

Before this change, `asc xcode --help` exposed `inject`, `archive`, `export`,
`export-options`, `validate`, and `version`. Simulator and device compile checks
therefore had to invoke `xcodebuild` directly or misuse `asc xcode archive`.

The supported invocation shape is:

```bash
asc xcode build \
  --project App.xcodeproj \
  --scheme App \
  --configuration Debug \
  --destination 'platform=iOS Simulator,name=iPhone 17 Pro Max,OS=27.0' \
  --no-code-signing \
  --output json
```

Replace the example destination with a simulator installed on the current host.

Exactly one of `--project` or `--workspace` is required, and `--scheme` is
required. `--configuration`, `--destination`, `--derived-data-path`,
`--result-bundle-path`, `--clean`, and `--no-code-signing` are explicit typed
options. Repeatable
`--xcodebuild-flag` values are appended as individual process arguments, using
the same no-shell passthrough model as archive and export. Passthrough cannot
override asc-managed selector, destination, derived-data, result-bundle,
`CODE_SIGNING_ALLOWED`, or build-action arguments; those must use the typed
flags so the reported result stays truthful.

## Behavior and output

When `--derived-data-path` is omitted, asc derives a stable cache path outside
the source checkout from the absolute project/workspace path and the selected
scheme, configuration, and destination. An explicit path always wins. An
optional result-bundle path is resolved to an absolute path and must not already
exist; asc never deletes or overwrites it. The derived-data path and requested
result-bundle path are reported. The derived-data `Build/Products` directory is
reported only when the current invocation creates that previously absent
directory; asc does not guess individual product paths or parse human-readable
build logs.

`--no-code-signing` appends `CODE_SIGNING_ALLOWED=NO`. It never turns signing
off merely because a destination looks like a simulator, so device builds keep
Xcode's normal signing behavior unless the operator opts out explicitly.

Successful JSON includes the requested project/workspace, scheme, explicit
configuration and destination when supplied, selected derived-data path,
requested result-bundle path when supplied, clean choice, requested
`no_code_signing` override, `success: true`, and `duration_ms`.
`no_code_signing` does not claim to resolve the project's effective code-signing
build setting. Table and Markdown render the same stable fields.
After xcodebuild completes, asc includes `exit_status`: zero for success or the
subprocess exit status for an ordinary process failure. Preflight and
cancellation failures omit the field because no meaningful process exit status
exists. Failed builds return a non-zero command error. Xcode logs and diagnostics
remain on stderr so machine-readable stdout stays parseable.

The command is discoverable through root/Xcode help, live command search,
generated command docs, and README/workflow examples. Execution remains
macOS-only with the existing Xcode availability errors on other hosts.

## Compatibility and failure modes

This is additive behavior. Existing archive/export invocations, error
text, error chains, and output schemas do not change. Command-shape and output
format validation happen before filesystem or subprocess side effects. Missing
Xcode, unsupported hosts, nonexistent or mis-typed project/workspace paths,
cache-path resolution failures, build failures, and context cancellation are
returned explicitly. Process arguments are passed as an argv slice, never
through a shell.

## Verification plan

- CLI RED/GREEN coverage for required flags, project/workspace exclusion,
  positional arguments, every typed flag, repeatable raw flags, and JSON/table/
  Markdown rendering.
- Core RED/GREEN coverage for normalization, deterministic default paths,
  argument order and space preservation, clean/action placement, explicit
  signing behavior, project/workspace path validation, unsupported hosts,
  subprocess failures and exit status, cancellation, and product-directory
  reporting.
- Built-binary checks for help, search, output streams, and usage exit codes.
- Repository format, docs, lint, test, race, and build gates.
- Read-only source validation against Zenther and a second local Xcode project
  or workspace, with derived data outside each checkout and clean-source checks
  before and after.

## Alternatives considered

A documented raw `--xcodebuild-flag=build` alias would remain hard to discover
and would not provide validation or structured output. Reusing archive would
perform materially different work. Automatically disabling signing for
simulator-looking destinations would be convenient but would make destination
parsing policy-sensitive and could silently change behavior; the explicit
composable flag is safer.
