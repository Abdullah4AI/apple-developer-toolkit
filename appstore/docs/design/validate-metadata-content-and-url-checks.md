# Validate metadata content and URL destinations

Status: proposed for 4.12.0

## Placement and current command contract

This change extends the existing top-level `asc validate` readiness report. It
does not add a command or an App Store Connect API operation. On the 4.11.0
baseline, `asc validate --help` accepts `--app`, `--version`, `--version-id`,
`--platform`, `--strict`, `--deep`, `--apple-id`, `--output`, and `--pretty`.
The new `--check-urls` flag is top-level:

```text
asc validate --app APP_ID --version-id VERSION_ID --check-urls
```

It can be combined with `--strict` and `--deep`; it is rejected on
`validate testflight`, `validate iap`, and `validate subscriptions`.

## Current source and API boundary

`internal/validation/report.go` builds the canonical `validation.Report` from
already-fetched version and app-info localizations. The report already checks
URL syntax and narrow, high-confidence placeholder markers. The existing
`internal/cli/metadata/url_checks.go` checker performs bounded public-URL
requests for the offline metadata validator. This feature extracts that
request/security implementation into a shared internal helper and maps its
results into the canonical report. No new App Store Connect endpoint,
OpenAPI operation, request schema, or response schema is needed.

The static checks reuse the existing validation data types and keyword audit
normalization. They do not run the user-supplied blocked-term list or expose
keyword values in the canonical report.

## Behavior and output

Static checks run offline with the existing validator. They add warning-only
findings for an app name shorter than two Unicode runes and the existing
deterministic keyword hygiene cases: empty segments, non-canonical separators,
locale duplicates, name overlap, and subtitle overlap. Existing length,
required-field, and narrow placeholder checks remain authoritative and are not
duplicated.

With `--check-urls`, populated syntax-valid `supportUrl`, `marketingUrl`,
`privacyPolicyUrl`, and `privacyChoicesUrl` fields are checked. Identical
trimmed URLs are requested once; findings are projected back to every affected
field and localization. Requests use a fixed timeout and bounded concurrency,
follow at most ten HTTP(S) redirects, disable proxies, and reject private,
loopback, link-local, reserved, or unspecified addresses at every dial and
redirect. Response bodies are closed without being read.

Findings are appended to the existing `Report.Checks` array with stable IDs and
the existing `CheckResult` fields. No new top-level output shape is introduced.
The command recomputes `Summary` and `Remediation` with the existing helpers.
Output remains TTY-aware (`table` for terminals and minified `json` for pipes),
and explicit `--output` continues to win. Finding order is deterministic.

New URL IDs are:

- `legal.url.request_failed`
- `legal.url.http_status`
- `legal.url.redirected_host`
- `legal.url.site_root`
- `legal.url.unsafe_target`

All new findings are warnings. They do not change non-strict exit behavior;
`--strict` makes them blocking through the existing summary and validation
error path. Ordinary URL failures do not need a new diagnostic code. Existing
invalid-input diagnostics and exit code 2 remain unchanged, and invalid flag
usage must not make a request.

Messages are generic. They must not contain the raw URL, query, fragment,
hostname, IP address, response body, headers, or transport/DNS details. The
network operation is opt-in and read-only; no App Store Connect credentials,
cookies, or mutation requests are used.

## Compatibility and lifecycle

`asc metadata validate --check-urls` keeps its current behavior and output. The
shared checker may be extracted, but the existing metadata validator remains
backward compatible. Without `--check-urls`, `asc validate` makes no additional
network request and keeps its existing result shape apart from the new offline
warning checks. Future interface changes require compatibility review.

The implementation must not scrape or interpret HTML, assess page semantics,
verify contact information, classify editorial copy, add a generic description
minimum, or make remote failures default errors.

## RED-GREEN and verification plan

Start with failing unit tests for Unicode app-name length, keyword hygiene,
privacy-safe URL finding projection, deduplication, redirect/status/unsafe
target outcomes, and unchanged placeholder behavior. Add command-level tests
for flag parsing and rejection, warning versus strict exits, deterministic
JSON/table/Markdown rendering, no requests without `--check-urls`, and generic
redaction. Use `httptest` and an injected checker; no test calls a real site.

Run focused package tests after each implementation step, then:

```bash
make build
make format
make check-docs
make lint
ASC_BYPASS_KEYCHAIN=1 make test
```

Because help changes, regenerate `docs/COMMANDS.md`. If credentials are
available, run one read-only built-binary smoke test against an existing app
and version with `ASC_BYPASS_KEYCHAIN=1`; do not mutate App Store Connect data
and redact all identifiers.

## Unresolved risks

1. The new offline warnings raise the warning count for existing apps. Pipelines
   that already run `--strict` can begin to fail without a metadata change.
2. `--check-urls` depends on destination availability, so repeated runs can
   report different results for unchanged metadata.
3. Live App Store Connect smoke verification remains environment-dependent and
   is not a substitute for the deterministic API fixtures and URL-server tests
   above.

## Alternatives considered

1. Add only a separate metadata subcommand. This preserves the current split
   and leaves the canonical readiness report incomplete, so it is rejected.
2. Run network checks by default. This would make an ordinary readiness check
   non-deterministic, leak operator-supplied destinations to network services,
   and introduce avoidable SSRF risk, so explicit opt-in is preferred.
3. Scrape destination pages for contact or policy content. Page semantics are
   provider- and localization-dependent and cannot be proven safely by a
   bounded CLI check; status, redirect, and address evidence are the narrower
   supported contract.
