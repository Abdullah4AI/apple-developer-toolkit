# Official search optimization plan

## Goal

Add a read-only workflow that turns the official Apple Ads
Platform API v1 optimization surface into one evidence-based App Store search
plan. The workflow joins Apple Ads demand and paid-performance data with the
selected App Store Connect localization. It never calls Apple web-session or
private endpoints and never mutates campaigns or App Store metadata.

## Command placement and invocation

The cross-API workflow lives under a new top-level `optimize` group rather than
under either `ads` or `metadata`:

```bash
asc optimize search plan \
  --app "123456789" \
  --version "APP_VERSION" \
  --ad-account "987654321" \
  --country "US" \
  --genre "PRODUCTIVITY_UTILITIES" \
  --locale "en-US" \
  --window "30d" \
  --out-dir ".asc/optimization/APP_VERSION" \
  --output markdown
```

Required inputs are `--app`, `--version`, `--ad-account`, `--country`,
`--genre`, and `--locale`. `ASC_APP_ID`, `ASC_ADS_AD_ACCOUNT_ID`, and stored
Ads profiles remain valid resolution sources. `--platform` defaults to `IOS`,
and `--window` defaults to `30d` with an accepted range of 2 through 30 whole
days. `--out-dir` is optional; omitting it keeps the command stdout-only.
When an app has multiple App Info records, the command matches App Info to the
selected version state; `--app-info` provides an explicit override when that
match is ambiguous.

The command supports JSON, table, and Markdown. Data is written only to stdout,
diagnostics are written to stderr, and usage errors exit with code 2. The
command accepts no positional arguments.

## Official API contract

The workflow uses the existing typed App Store Connect client to resolve the
selected version and read its version and app-info localizations. Apple Ads
credentials remain independent from App Store Connect credentials.
The report records the exact resolved version and App Info resource IDs.

It composes these Platform API v1 operations:

| Purpose | Method and endpoint |
| --- | --- |
| App campaign scope | `POST /v1/campaigns/query` |
| Current targeting | `POST /v1/keywords/query` |
| Existing exclusions | `POST /v1/negative-keywords/query` |
| App keyword discovery | `POST /v1/suggestions/keywords/query` |
| App phrase discovery | `POST /v1/suggestions/phrases/query` |
| New-campaign target CPA | `POST /v1/suggestions/target-cpas/query` |
| Country/genre demand | `POST /v1/insights/apps/search-term-popularity/query` |
| Paid competitive reach | `POST /v1/insights/apps/impression-share/query` |
| Actual matched queries | `POST /v1/reports/apps/searchterms/query` |
| Placement eligibility | `POST /v1/eligibilities/apps/query` |
| Budget signals | `POST /v1/recommendations/daily-budgets/query` |
| Target-CPA signals | `POST /v1/recommendations/target-cpas/query` |

List and report bodies use body pagination and fetch every page. The selected
performance window ends yesterday. Search-term popularity uses the most recent
published Sunday-through-Saturday week because its time-range contract differs
from Ads performance reports. The workflow waits for Apple's Monday 07:00 UTC
publication time before requesting the immediately preceding week.

## Report semantics

The report contains one normalized row per search term. A row records source
provenance instead of presenting unavailable fields as zero:

- market popularity on Apple's 1–5 Campaign Management scale, its 1–100 scale,
  and genre rank come from Search Term Popularity;
- suggestion popularity is preserved separately because the suggestion
  response does not identify a source storefront;
- impression share and share rank are paid, app-specific metrics;
- the 1–5 popularity value carried by an Impression Share row is preserved
  separately so overlapping official snapshots can be compared;
- when Apple returns daily impression-share rows, the report selects the latest
  dated bucket deterministically and records that period;
- impressions, taps, installs, and spend come from actual Search Terms reports;
- CPA is computed as local spend divided by total installs;
- metadata coverage is an exact normalized phrase check across name, subtitle,
  and comma-separated keyword values in the requested locale;
- actions and confidence are deterministic client-side classifications.

The initial action vocabulary is:

- `promote_exact`: a broad-matched query has produced installs and is not an
  existing exact keyword;
- `negative_candidate`: at least 10 taps and no installs, excluding existing
  negative keywords;
- `metadata_candidate`: a converting term or app suggestion is absent from the
  requested metadata locale;
- `defend`: a converting term has paid impression share below 50 percent;
- `saturated`: Apple reports the greater-than-90-percent share bucket;
- `untested_candidate`: Apple suggests the term but the account has no
  performance row for it.

Multiple actions can apply. Confidence is `proven` when installs exist,
`observed` when paid traffic exists, and `suggested` for discovery-only rows.
No proprietary difficulty, organic attribution, or app-specific popularity is
invented.

Actions that depend on proving absence are source-aware. `promote_exact` is
suppressed when current targeting keywords are unavailable,
`negative_candidate` is suppressed when existing negatives are unavailable,
and `untested_candidate` is suppressed when search-term performance is
unavailable. Partial rows remain usable, but missing evidence is never treated
as an empty result.

Apple's complete daily-budget and target-CPA recommendation objects are
preserved in the report alongside their summary counts. The app-and-country
target-CPA suggestion for a new Maximize Conversions campaign is preserved as
a separate raw official signal. The workflow does not reinterpret or apply any
of them.

Every Apple Ads source records `available`, `empty`, or `unavailable`. A source
error produces a report notice and does not erase data from successful sources.
Table and Markdown output render the source matrix, notices, and term plan; JSON
retains the complete structured objects.

The default TTY table is the compact human view. Its summary names the app and
groups the version, platform, and market context; the term plan follows it so
the primary result fits in one terminal capture. Source status and shortened
one-line notices follow the plan, and the term table combines the two popularity
scales in one `Popularity` column. Row-level source arrays and complete provider
errors remain available in Markdown and JSON. This keeps the default output
readable in a normal terminal without weakening the report used by scripts or
audits.

Compatibility is limited to presentation: no flag, exit code, report schema, or
artifact changes. Keeping the original ten-column table was rejected because
ordinary terms, actions, and source names push it well beyond a screenshot-sized
terminal. Truncating report data was also rejected; only the TTY rendering is
condensed.
The command fails if App Store Connect metadata cannot be resolved or if every
Apple Ads intelligence source is unavailable. Privacy-suppressed and genuinely
empty official responses remain successful empty sources.

## Artifacts

When `--out-dir` is present, the operator-selected directory is the trusted
`internal/rootfs` anchor. The command writes:

- `report.json`: the complete canonical report;
- `metadata-candidates.csv`: an import-compatible `locale,keywords` proposal
  that preserves existing keywords and adds fitting candidates without
  exceeding the 100-character field limit;
- `exact-keywords.json`: a `KeywordCreateBulkRequest` for `promote_exact`
  actions with known ad-group context;
- `negative-keywords.json`: a `NegativeKeywordCreateBulkRequest` for
  `negative_candidate` actions with known campaign/ad-group context.

Artifacts are plans only. Operators apply them through the existing confirmed
commands (`asc metadata keywords import/plan/apply`, `asc ads
targeting-keywords create-bulk`, and `asc ads negative-keywords create-bulk`).
`asc release stage` and `asc publish appstore --metadata-dir` remain the only
release integration; this change does not add an implicit publish mutation.

## Compatibility and lifecycle

`optimize` is a new root group, so there is no compatibility or
deprecation impact. Existing raw Apple Ads and metadata commands do not change.
The JSON report and artifacts include a schema version so future additions can
remain explicit.

## Tests and verification

RED-GREEN coverage includes:

- root registration, help, required flags, invalid country/locale/window, and
  positional-argument rejection;
- one realistic joined result across suggestions, popularity, impression
  share, search-term performance, existing targeting, and metadata;
- deterministic action precedence, provenance, missing data, zero installs,
  saturated share, existing keyword/negative suppression, and dependency-aware
  suppression when absence cannot be proven;
- Apple Ads method, path, context header, body filters, body pagination, and
  API-error partial-result behavior, including schema-specific pagination;
- Apple's Campaign Management-style 1–5 popularity, 1–100 popularity, genre
  rank, and the app/country target-CPA suggestion;
- JSON, table, and Markdown output;
- artifact content, import compatibility, 100-character budgeting, and rooted
  write failures;
- built-binary stdout, stderr, and exit-code checks.

Credentialed verification must remain read-only and cover at least two apps,
including an app with multiple App Info records and an app with campaign data.
Verify source fill rates, country scoping, report latency, artifact parsing,
pagination, and score agreement for overlapping sources. Treat provider errors
as explicit unavailable-source risk; do not infer successful coverage from a
partial report or apply generated artifacts during verification.

## Alternatives

Adding convenience flags to each raw endpoint would reduce JSON boilerplate
but would still leave users to join incompatible response shapes themselves.
Putting the workflow under `ads` would hide that App Store Connect metadata is
an equal input. A cross-API `optimize` group makes the orchestration explicit
without presenting the result as an Apple-provided recommendation.
