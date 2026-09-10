# Deep submission validation

## Decision

Extend the stable top-level App Store readiness command with an explicit,
additive deep mode:

```bash
asc validate --app "APP_ID" --version "1.0.0" --deep
asc validate --app "APP_ID" --version-id "VERSION_ID" --deep --apple-id "user@example.com"
```

Default `asc validate` remains public-API-only. `--deep` first builds that same
report, then uses an existing file-backed Apple web session to replace web-only
uncertainty with live evidence. It never starts an interactive login or opens
Keychain. When no usable cached session exists, the report remains available
but contains a warning-severity deep-validation finding. It includes an exact
`asc web auth login` recovery when the selected Apple Account is known.
`--strict` remains the explicit fail-closed policy.

`--apple-id` only selects a user-owned cached web session and is valid only with
`--deep`. Omitting it selects the last cached web session, matching the existing
`asc web` behavior. Deep validation is read-only and does not accept `--confirm`.

## Scope

The 4.11 slice verifies the web-only submission blockers for which the
repository already has read clients:

1. App Privacy publication state.
2. Pending Apple Developer Program agreements and App Store Connect contract
   messages.
3. First auto-renewable subscription attachment when no subscription is yet
   approved and at least one subscription is `READY_TO_SUBMIT`.

The private subscription field describes attachment to the app's next version,
not an arbitrary selected version. Deep mode evaluates it only when the selected
public version is a current review candidate, including `READY_FOR_REVIEW`. A
terminal historical version is `notApplicable`; a missing public version state
is `unverified`.

Paid Apps Agreement relevance includes an app's current upfront download price,
not only active IAPs and subscriptions. Deep mode reads the current manual app
price from the existing public price schedule. If that evidence is incomplete,
it reports paid-agreement relevance as unverified instead of assuming the app
is free or inventing a blocker.

The existing public readiness report remains authoritative for review-detail
completeness, initial app availability, pricing, metadata, screenshots, build
state, age rating, IAPs, and subscription metadata. Deep mode enriches those
existing actionable findings rather than fetching them twice.

Every actionable deep-mode finding receives an additive resolution object with
one repair channel (`api-fixable`, `web-fixable`, or `manual`), zero or more
exact long-form `asc` commands, and a relevant App Store Connect URL when one is
stable. API-fixable public findings carry the resolved resource ID and flag;
commands use an explicit placeholder only when the missing content still needs
an operator decision. The same fields flow into the ordered remediation plan
and appear as extra columns in table and Markdown output only when deep mode
supplies resolution data.

Deep mode also adds a structured `deep` section. It reports the cached-session
status and one deterministic result for privacy publication, subscription
attachment, agreements, availability, and review information. Status is one of
`passed`, `blocked`, `unverified`, or `notApplicable`; source is one of
`publicApi`, `webSession`, or `manual`. Availability and review information are
derived from the existing public checks, so their presence in this section does
not trigger duplicate requests or duplicate root findings.

## Endpoint and response contracts

No public App Store Connect OpenAPI endpoint or request schema changes.
Deep mode also reads the current manual price under the app's public price
schedule to distinguish free from upfront-paid apps when deciding whether the
Paid Apps Agreement is relevant.
Deep mode reuses the existing private web-session readers:

- `GET /apps/{id}/dataUsagePublishState`, decoded as
  `AppDataUsagesPublishState { id, published }`.
- `GET /apps/{id}/subscriptionGroups` with included subscription fields
  `state`, `submitWithNextAppStoreVersion`, and
  `isAppStoreReviewInProgress`.
- `GET /contractMessages` plus the read-only Developer Portal agreement-history
  request at `/services-account/QH65B2/account/getAgreementHistory`, decoded as
  `WebAgreementsStatusResult { pending, contractMessages, agreements }`.

Although agreement history uses Apple's POST-shaped private read endpoint, deep
validation sends no mutation request. App Privacy publication and subscription
attachment mutations remain separate commands that require explicit
confirmation where applicable.

The subscription web model exposes
`submitWithNextAppStoreVersionKnown` additively in JSON. A false presence bit
means Apple omitted the attachment attribute; consumers must not interpret the
companion boolean as definitively unattached.

## Output and exit contract

JSON remains the default for pipes and CI; table remains the terminal default.
The deep report is emitted additively as `deep: { sessionStatus, summary,
checks }`. Existing fields keep their names and meanings. Resolution fields use
exported camelCase structs.

Successful web checks stay in the deep section so callers can distinguish
"verified" from "not checked" without adding issue-shaped root checks. An
unpublished privacy state, pending agreement, or missing required first-of-type
subscription attachment is an error-severity root finding. An unverifiable
requested deep check is a warning, preserving the report while making
`--strict` the explicit fail-closed policy. The complete report is printed to
stdout before the command returns the existing validation reported-error exit
contract. Usage mistakes such as `--apple-id` without `--deep` return exit code
2 on stderr before authentication or HTTP.

Independent web-check failures do not erase successful public checks or other
deep results. They become explicit unverified findings with safe diagnostic
text; raw response bodies, cookies, passwords, provider identifiers, and signed
URLs never enter the report. The selected Apple Account email appears only in
account-scoped remediation commands so a later mutation cannot use a different
cached account.

## Compatibility and lifecycle

The change is additive and keeps all existing default invocations, JSON shapes,
exit behavior, and subcommands. `--deep` and `--apple-id` rely on private Apple
endpoints. Private endpoint failures are represented as unverified checks rather
than silently treated as success.

Top-level-only flags remain rejected before `validate testflight`, `validate
iap`, and `validate subscriptions`; no subcommand silently ignores `--deep` or
`--apple-id`.

## RED-GREEN verification

RED coverage precedes implementation for:

- `--deep` and `--apple-id` parsing, ordering, and top-level/subcommand scope.
- `--apple-id` without `--deep` as a usage error with empty stdout.
- cached-session missing and expired behavior without an interactive prompt.
- published and unpublished App Privacy states.
- active and pending agreement states.
- free, upfront-paid, and incomplete app-pricing evidence for agreement scope.
- no subscriptions, no ready subscriptions, attached ready subscriptions, and
  ready-but-unattached first-of-type subscriptions, including a selected
  `READY_FOR_REVIEW` version without offering an unsafe attachment mutation.
- one private endpoint failing while the other deep checks still complete.
- additive JSON resolution fields, API-fixable public commands, and conditional
  table/Markdown columns.
- stable blocking counts, ordered remediation, stdout/stderr, and exit codes.

Focused package tests, command-level tests, generated command docs, and a built
binary cover the changed surface. Full verification is `make build`, `make
format`, `make check-docs`, `make lint`, and `ASC_BYPASS_KEYCHAIN=1 make test`.
A live smoke test is read-only and runs only when both API credentials and a
cached web session are available; absence of either is reported, not hidden.

## Alternatives

1. A new `asc validate deep` subcommand would duplicate the canonical top-level
   validator and make flag placement harder to discover. The user-facing
   decision is an opt-in validation depth, so a flag is clearer.
2. Making web checks the default would add session requirements and private API
   fragility to existing CI. Explicit opt-in preserves compatibility.
3. `asc launch` or automatic repairs would be a larger orchestration and
   mutation project. Deep validation deliberately remains read-only for 4.11.
4. Reimplementing review details and availability through web endpoints would
   duplicate current public-API evidence. Enriching the existing findings is
   smaller and more trustworthy.

## Deferred work

- Automatic `--fix` or `--fix-web` mutations.
- Interactive login, 2FA, or provider selection inside validation.
- App creation, signing, build/upload, and full zero-to-review orchestration.
- Web checks without an established, tested read client.
- Treating later subscriptions as if they still require an app version. The
  4.11 check becomes not applicable once an auto-renewable subscription is
  approved. When the first-of-type submission has several ready candidates,
  the report returns the safe list command and leaves the choice to the
  operator. It emits a mutation command only when one candidate is unambiguous.
