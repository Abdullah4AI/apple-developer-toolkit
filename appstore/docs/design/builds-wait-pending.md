# Resumable `builds wait` timeouts

## Problem

`asc builds wait` is the documented way to wait for a build, but callers that
run under a short command time limit cannot use it. When `--timeout` expires
the command prints one line, for example
`Error: builds wait: timed out resolving build selector after 50s`, writes
nothing to stdout, and exits 1. That line does not say whether App Store
Connect has received the upload, whether a build exists, what its processing
state is, or how to continue. Callers cannot tell "not done yet" from "broken",
so they abandon the command and poll `asc builds list` in a sleep loop instead.

Usage data from coding-agent sandboxes shows the pattern: agents in 261
projects issued 1,672 `sleep N; asc builds list` polls (median 4 and p90 15 per
project), mostly with 40-55 second sleeps, while 16 of the 41 `builds wait`
calls with captured output ended in the bare timeout above.

During discovery the build is often not visible in `/v1/builds` yet even
though App Store Connect has accepted the upload. Discovery never looks at
build uploads, so it cannot say "uploaded, not yet visible as a build".

## Contract

### Default behavior (unchanged exit code and streams)

Without the new flag a timeout still exits 1 with nothing on stdout, and the
error keeps its existing prefix (`timed out resolving build selector after …`
or `timed out waiting for build … after …`). The message now appends the last
known state and the command that resumes the wait:

```text
Error: builds wait: timed out resolving build selector after 50s; build upload "UPLOAD_ID" (version 1.0.12, build 78, IOS) is PROCESSING and not yet visible as a build; resume with: asc builds wait --app 123456789 --build-number 78 --version 1.0.12 --platform IOS --timeout 50s --poll-interval 10s; add --report-pending to print this state on stdout and exit 7
```

### `--report-pending`

`asc builds wait --report-pending` opts in to a resumable outcome. When the
wait's `--timeout` expires before the build reaches a terminal state, the
command:

- prints a pending result to stdout in the selected output format,
- prints `Build is still pending after <elapsed>; resume with: <command>` to
  stderr,
- exits with the new `ExitPending` code `7`.

A terminal outcome is unaffected by the flag: `VALID` (and `INVALID` without
`--fail-on-invalid`) still prints the existing wait result and exits 0, and
`FAILED`, `--fail-on-invalid`, API, auth, and transient-budget failures keep
their current errors and exit codes.

The JSON shape is an additive receipt type, `asc.BuildWaitPendingResult`:

```json
{
  "status": "pending",
  "phase": "discovery",
  "summary": "Build upload \"UPLOAD_ID\" (version 1.0.12, build 78, IOS) is PROCESSING and not yet visible as a build.",
  "appId": "123456789",
  "version": "1.0.12",
  "buildNumber": "78",
  "platform": "IOS",
  "upload": {
    "id": "UPLOAD_ID",
    "state": "PROCESSING",
    "version": "1.0.12",
    "buildNumber": "78",
    "platform": "IOS",
    "uploadedDate": "2026-09-30T10:00:00Z"
  },
  "elapsed": "50s",
  "timeout": "50s",
  "resumeCommand": "asc builds wait --app 123456789 --build-number 78 --version 1.0.12 --platform IOS --timeout 50s --poll-interval 10s --report-pending --output json"
}
```

- `status` is always `pending`.
- `phase` is `discovery` when no build matched the selector yet, or
  `processing` when the build exists and has not reached a terminal state.
  In the `processing` phase `buildId` and `processingState` (the last observed
  state) are set and `upload` is omitted.
- `upload` is the most recently uploaded build upload that matches the
  selector, or absent when none is visible. Its `state` is Apple's
  `BuildUploadState` (`AWAITING_UPLOAD`, `PROCESSING`, `COMPLETE`).
- `resumeCommand` re-runs the same wait. In the `processing` phase it switches
  to `--build-id` so a newer upload cannot be picked up instead. It keeps the
  explicitly set root flags (`--profile`, `--strict-auth`, `--report`,
  `--report-file`) and the explicitly set `--timeout`, `--poll-interval`,
  `--fail-on-invalid`, `--report-pending`, `--output`, and `--pretty` flags.
  Values are rendered with `shared.ShellQuote`; when one cannot be rendered as
  a copyable argument for the platform, the field is omitted rather than
  approximated.

Table and Markdown output render one row with the status, phase, build,
processing state, upload state, elapsed time, and resume command.

### Exit code 7

`ExitPending` (7) joins the well-known 0-9 range. None of the existing codes
fit: 1 is an unclassified failure, which is exactly what callers need to tell
apart from "not finished", and 3-6 are auth, not-found, conflict, and
read-only outcomes. The code is only produced when the caller opted in, so no
existing invocation changes its exit status.

### Failed upload at timeout

If the build was never found and the matching build upload is `FAILED`, the
build will never appear. The timeout then reports the upload failure (with
Apple's error details and recovery guidance) as an ordinary error with exit 1,
also under `--report-pending`, instead of claiming the wait is pending.

## Build upload lookup during discovery

Each discovery poll that finds no build also reads
`GET /v1/apps/{id}/buildUploads` with:

- `filter[cfBundleVersion]` from `--build-number`,
- `filter[cfBundleShortVersionString]` from `--version`, including the
  equivalent `1.2`/`1.2.0` spelling that build discovery already accepts,
- `filter[platform]` from `--platform`,
- `sort=-uploadedDate` and `limit=20`.

The newest match by `uploadedDate` (or `createdDate` before the upload
finishes) is kept, and `--since` excludes uploads older than the cutoff. The
lookup is best effort: an error, including Apple's intermittent app-scoped
404, only means this poll reports no upload, and it never fails the wait. It
adds one read per discovery poll and none while waiting for processing.

## Compatibility

- Default stdout, exit codes, and the existing error prefixes are unchanged;
  only the error text gains a suffix.
- The success JSON (`asc.BuildWaitResult`) is unchanged.
- The pending receipt is a new type with a registered renderer, so its fields
  follow the additive output contract.
- The default timeout keeps its telemetry classification: it does not wrap
  `context.DeadlineExceeded`. The opted-in pending outcome (exit 7) is
  recorded as `expected_negative` with the `state_not_ready` diagnostic.

## Alternatives considered

- **Exit 7 by default.** Clearer, but changes the exit status and stdout of
  every existing timed-out invocation, which the stability contract forbids
  without a deprecation cycle.
- **Exit 0 with `status: "pending"`.** Lets `asc builds wait && next-step`
  continue with an unprocessed build, so a pending wait must stay non-zero.
- **Failing fast on a `FAILED` upload during discovery.** Useful, but it
  changes how long existing waits run; it is left for a follow-up.

## Tests

- `internal/cli/cmdtest/builds_wait_pending_test.go` runs `cmd.Run` against
  recorded App Store Connect responses with a fake clock (`builds.SetWaitClockForTesting`)
  and asserts stdout, stderr, and exit codes for: discovery-phase pending with
  a visible upload (including the upload query filters), without one, and with
  `--since` filtering uploads; processing-phase pending resuming by
  `--build-id`; the default timeout message for both phases; a `FAILED`
  upload at timeout; and an upload lookup that keeps failing.
- `cmd/exit_codes_test.go` and `internal/cli/cmdtest/exit_codes_test.go`:
  `ExitPending` value and mapping, including a wrapped pending error.
- `internal/asc/output_builds_test.go`: pending row rendering.
- Existing discovery tests answer the added build-upload read.
- A built binary is checked for help text and flag parsing. The CLI has no
  base-URL override, so the timed-out paths are covered through `cmd.Run`
  instead of a built binary.
