# Metadata URL intent checks

## Decision

`asc metadata validate` stays deterministic and offline by default. The new
`--check-urls` flag opts into bounded HTTP checks for version
localization `supportUrl` values and app-info localization
`privacyPolicyUrl` values.

The opt-in check follows redirects and reports warnings when:

- the request fails or the final response is not successful;
- the final host differs from the declared host; or
- a successful response ends at a bare site root (an empty path or `/` without
  a query or fragment route), which is unlikely to be a dedicated support or
  privacy page.

Warnings keep `valid: true` and exit status 0 when no ordinary metadata errors
exist. Existing syntax and length checks continue to run before network work.
Repeated URLs are fetched once, checks run with bounded concurrency and
timeouts, and direct or redirected private, loopback, link-local, and reserved
network targets are rejected.

## Command and output contract

```sh
asc metadata validate --dir "./metadata" --check-urls
```

The command still reads canonical metadata from `--dir`, performs no App Store
Connect API mutation, prints the existing `ValidateResult` JSON/table/Markdown
shape, and writes normal output to stdout. URL findings are additive
`ValidateIssue` warnings using the existing fields and renderer.

## Alternatives

Running HTTP checks by default would make the established offline command
network-dependent and non-deterministic. A separate command would duplicate
metadata discovery, validation output, and exit semantics. HEAD-only requests
were also rejected because many valid sites do not implement HEAD consistently;
the checker uses a bounded GET and closes the response body without reading it.

## Verification

RED-GREEN coverage includes flag parsing, redirect-to-root, final-host mismatch,
unsuccessful status, request failure, duplicate URL caching, offline-by-default
behavior, warning-only exit/output semantics, redirect limits, and rejection of
non-public targets. A built-binary smoke test uses read-only public URLs.
