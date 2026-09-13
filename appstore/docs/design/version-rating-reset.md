# Version rating reset revival

## Scope

Revive the unique rating-reset work from closed PR #2263 on current main. The
other four remote branches are already incorporated or superseded. This PR
remains a draft until an authenticated disposable-app contract check succeeds.

## Command contract

The current main binary rejects `asc versions rating-reset`. Add a group under
`versions`, implemented in the web command package to reuse authentication and
provider selection:

- `view --version-id VERSION_ID`: read the scheduled reset, preserving the entire
  Apple JSON envelope, including an explicit `data: null` when unscheduled.
- `create --version-id VERSION_ID --confirm`: schedule a reset and return an
  exported camelCase receipt with request ID, version ID, and `scheduled`.
- `delete --id RESET_REQUEST_ID --confirm`: cancel the request and return its ID
  and `cancelled`.

Support existing output formats, provider flags, and Apple Account session
selection. Required flags and confirmation fail with usage exit 2 before auth.
Data goes to stdout and diagnostics to stderr. Request timeouts start after
session resolution, using the current shared resolver contract. No existing
command is removed or changed.

## Proposed Apple contract and evidence limits

The offline OpenAPI snapshot has no rating-reset endpoint. The branch uses the
existing authenticated cookie transport to `https://appstoreconnect.apple.com/iris/v1`:

| Operation | Method and relative path | Expected data |
| --- | --- | --- |
| View | `GET /appStoreVersions/{versionId}/resetRatingsRequest` | JSON:API resource or explicit null relationship |
| Create | `POST /resetRatingsRequests` | `data.type=resetRatingsRequests`, relationship `appStoreVersion` with `type=appStoreVersions` and selected version ID |
| Cancel | `DELETE /resetRatingsRequests/{requestId}` | Successful response |

A resource contains `type`, `id`, and nullable `attributes.resetDate`. Preserve
unknown response fields in JSON output. Reject missing `data` instead of
mistaking malformed payloads for an unscheduled reset.

These paths and payloads remain a hypothesis supported by deterministic tests,
not a verified live contract. The old closure cited failed public/API-key
probes, which do not test the web-session transport used here. Apple documents
the UI behavior at
https://developer.apple.com/help/app-store-connect/monitor-ratings-and-reviews/reset-an-app-overview-rating.
That UI documentation does not establish an Iris API contract.

## Verification plan

Establish RED from root command integration tests on current main, then restore
the command, HTTP contracts, receipts, and discovery coverage. Reproduce the
old pre-auth request deadline and malformed-response handling before fixing
them. Run focused packages, a worktree-specific binary, repository gates, and
local reviews before opening the draft.

Before leaving draft, use an authorized disposable app and a valid authenticated
web session: select a non-releasing editable version; verify no reset is already
scheduled; create once; read the same request ID; cancel once; read an explicit
null relationship. Preserve the original state. After an uncertain write, read
state before any retry. Do not create a new version or release an app as part of
this verification. Record remaining resources if cleanup fails.

The first status check on September 8 reported the cached web session as
unauthenticated. No Apple mutation was performed during local preparation.

## Alternatives

A public API implementation is unsupported by the offline specification.
Retaining only the UI instructions is the fallback if live web-session evidence
rejects the proposed contract. Do not mark this draft ready based on mocked tests.
