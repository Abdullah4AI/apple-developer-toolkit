# Fail fast when no web session is available

## Problem

`asc web` commands need a signed-in Apple Account session. Without one and
without a terminal, they failed as usage errors (exit `2`) and printed the
command's whole usage page.

- With nothing cached, the error was `--apple-id is required when no cached web
  session is available; run 'asc web auth login --apple-id EMAIL'`.
- With an account selected but no usable session, it was `password is required:
  run in a terminal for an interactive prompt or set ASC_WEB_PASSWORD`.

In non-interactive environments such as coding-agent sandboxes and CI, the
first message sends callers to `asc web auth login`. That command then stops on
the second message, and callers retry other web commands. Agents in 68 projects
hit this error 136 times. When stderr is merged into stdout, the usage page also
breaks JSON parsing: `DESCRIPTION` was the most common first line of
`2>&1 --output json` output.

## Decision

- Every web command except `asc web auth login` returns
  `shared.MissingWebSessionError` (matching `shared.ErrMissingWebSession`)
  when it has no usable session and cannot sign in. This covers:
  - the empty-cache case;
  - the selected-account case with no password from `ASC_WEB_PASSWORD`, the
    saved-password store, or a terminal;
  - `web apps create` without a terminal.
- The error does not wrap `flag.ErrHelp` and is not reported by the command,
  so no usage page is printed. The root renderer prints the message and a
  `Hint:` line. The hint says that sign-in needs a terminal and names three
  ways forward: `asc web auth login`, `asc web auth import`, and the
  unattended sign-in variables.
- Where the public App Store Connect API answers the same question, the hint
  names that command:
  - `web review list` points to `asc review submissions-list`.
  - `web review show` and `web review threads` point to `asc review status`.
  - The other web commands have no public API equivalent and add nothing.
- `asc web auth login` exists to create the session, so it keeps its usage
  errors and usage page for a missing account or password. An ambiguous cache
  stays an ordinary usage error everywhere, because the fix is to pass
  `--apple-id`.

## Exit code and telemetry

Exit codes are a stable contract, so the failure keeps usage exit code `2`:
`cmd.ExitCodeFromError` maps `ErrMissingWebSession` to `ExitUsage`. Telemetry
keeps the classification the replaced usage errors had: a validation-stage
`usage_error` with error kind `missing_required`
(`MissingWebSessionError.UsageErrorKind`). The empty-cache case keeps the
`required_input_missing` diagnostic on `--apple-id`. The selected-account case
carries no diagnostic of its own, as before: the password it lacks is missing
input, and App Group commands, which derive a diagnostic from the usage
classification, keep reporting `required_input_missing` for it exactly as they
did for the replaced `password is required` usage error. No new diagnostic code
is added, because the telemetry collector validates codes against a fixed list.

The failure is an authentication problem rather than a usage problem, and the
authentication exit code (`3`) would describe it better. Moving it to `3` is
planned for the next major release, with migration guidance, because scripts
may rely on `2` today.

## Tests

- `internal/cli/cmdtest/web_session_required_test.go` runs the root command and
  asserts the exit code, the exact stderr, and the absence of the usage page.
  It covers:
  - privacy pull, agreements status, and review list, show, and threads;
  - the named-account case;
  - `web apps create`;
  - that `web auth login` keeps both usage errors.
- Unit tests cover:
  - the resolver for both cases, the sign-in context, and the diagnostic;
  - the public API alternative;
  - the usage exit-code mapping and telemetry classification;
  - the `errfmt` hint;
  - the `web auth capabilities` pass-through;
  - the App Group diagnostic.
