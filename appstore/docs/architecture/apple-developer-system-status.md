# Apple Developer system status

Status: Implemented

## Placement and command shape

`asc system-status` is a top-level utility command. It is separate from
`asc status`, which is an authenticated, app-scoped release dashboard, and
from `asc doctor`, which is an established alias for authentication
diagnostics.

Expected invocations:

```bash
asc system-status
asc system-status --service "App Store Connect API"
asc system-status --service "App Store Connect,TestFlight" --issues-only
asc system-status --watch --poll-interval 30s
```

The command accepts no positional arguments. `--service` is a comma-separated,
case-insensitive substring filter. `--issues-only` limits the service list while
retaining summary counts for all matched services. `--watch` emits the initial
snapshot and later snapshots only when the selected report changes.
`--max-polls` provides a bounded watch for automation and tests.

## Data source and response

This is not an App Store Connect OpenAPI operation. The command performs an
unauthenticated `GET` of Apple's public Developer System Status data feed:

```text
https://www.apple.com/support/systemstatus/data/developer/system_status_en_US.js
```

The feed currently wraps JSON in `jsonCallback(...)`. Its top-level payload has
`drMessage` and `services`; each service has `serviceName`, `redirectUrl`, and
`events`. Event fields include `messageId`, `statusType`, `message`,
`datePosted`, `startDate`, `endDate`, epoch timestamps, `usersAffected`, and
`eventStatus`.

The parser also accepts plain JSON so a harmless transport-format change does
not break the command. It rejects unknown wrappers, non-2xx responses, empty
service lists, unknown non-empty event statuses, and bodies larger than 2 MiB.
The source is a public web feed, not a documented API contract, so explicit
parse failures are preferable to silently reporting a healthy state when Apple
changes the schema.

## Output and exit behavior

The computed camelCase report contains:

- `source`, pointing to Apple's human-readable status page;
- a summary with overall status and service/incident counts;
- an optional global disaster-recovery message;
- selected services with computed `operational` or `issues` status and Apple's
  event details.

Output defaults to table in terminals and minified JSON for pipes or CI; an
explicit `--output` value takes precedence. JSON, table, and Markdown are
supported. A successfully fetched report exits zero even when Apple reports an
incident; the incident is data, not a command failure. Invalid flags and flag
combinations exit 2. Transport, HTTP, and parse failures exit 1. Standard data
goes to stdout and diagnostics go to stderr.

Watch mode reports failed polls to stderr and retries twice. It exits on the
third consecutive failure, while any successful poll resets the failure count.
JSON watch output is newline-delimited; `--pretty` is rejected in that mode so
each snapshot remains one complete JSON record.

## Compatibility and agent discovery

This is an additive command with no authentication requirement
and no changes to existing command behavior. The generated `ASC.md` agent
reference will instruct agents to query it after unexpected Apple API failures.
App Store Connect API 5xx errors reference the related system-status services;
generic timeout hints remain service-neutral. No request automatically performs
a hidden second network call.

## Tests and verification

RED-GREEN coverage includes:

- command registration and help;
- JSONP and plain-JSON decoding with a realistic incident;
- service filtering, issues-only summaries, and no-match errors;
- HTTP and malformed/oversized response failures;
- positional arguments and invalid watch flag combinations;
- changed-only bounded polling;
- JSON, table, and Markdown rendering;
- service-neutral timeout hints and App Store Connect API-only 5xx hints;
- a built-binary one-shot check against Apple's live feed.

The repository gate is `make format`, `make check-docs`, `make lint`, and
`ASC_BYPASS_KEYCHAIN=1 make test`. Because root help changes, generated command
documentation must also be refreshed before the gate.

## Handoff and residual risk

Implementation, validation, and review history are tracked in
[pull request #2062](https://github.com/rorkai/App-Store-Connect-CLI/pull/2062).
The remaining external risk is that the undocumented public feed can change
without notice; bounded parsing and explicit failures keep that visible.

## Alternatives considered

Adding flags to `asc doctor` would mix unauthenticated service health with a
stable authentication report and its JSON contract. Adding this to `asc status`
would require an app and credentials precisely when Apple may be unavailable.
Automatically querying Apple on every timeout or 5xx would add latency, noise,
and a second failure mode. A dedicated command plus a scoped App Store Connect
API 5xx hint remains explicit, scriptable, and reusable by both people and
agents.
