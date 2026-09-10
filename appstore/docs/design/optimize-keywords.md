# Keyword evaluation

## Goal

Add a read-only `asc optimize keywords` group that **evaluates**
App Store keyword candidates a caller already has. It does not invent keyword
candidates, and it does not mutate campaigns or App Store metadata.

The split of responsibility is deliberate. An agent is good at proposing a list
of plausible keywords and bad at knowing which of them are reachable. These
commands supply the missing half: Apple's own ranking, competition, and demand
evidence for a supplied list.

## Methodology adapted from

The keyword difficulty methodology in this document — the signal weights and
constants, the keyword match ladder, and the brand heuristic — is adapted from
**semihcihan's App Store Optimization CLI**, which is MIT licensed:

<https://github.com/semihcihan/App-Store-Optimization-CLI>

`internal/cli/optimize/keywords_difficulty.go` is an independent Go
implementation of that published formula and its constants, not ported code.
The rest of this document describes the formula on its own terms so a reviewer
can audit and re-derive every number without leaving this repository.

## Commands

```bash
asc optimize keywords rank     --app APP_ID --keywords LIST [--country us] [--platform IOS|TV_OS] [--workers N]
asc optimize keywords score    --keywords LIST [--country us] [--app APP_ID] [--genre GENRE] [--ad-account ID] [--ads-profile NAME] [--workers N]
asc optimize keywords discover --app APP_ID [--country us] [--genre GENRE] [--ad-account ID] [--ads-profile NAME] [--limit N]
```

`rank` needs no authentication at all. `score` needs no authentication for its
competition and rank sources, and needs Apple Ads credentials only for the
optional popularity source. `discover` needs Apple Ads credentials, because
Apple Ads is its only source.

The intended loop is `discover` to collect official candidates, `score` to
decide which are worth pursuing, and `rank` to track the ones that were
adopted. `discover` emits a `scoreKeywords` field that feeds straight into
`score --keywords`.

## Keyword hygiene

Every subcommand normalizes `--keywords` before any request is made:

1. split on commas,
2. trim, lowercase, and collapse internal whitespace,
3. deduplicate while preserving the caller's order.

A keyword must be 2-60 characters and at most 4 space-separated words, and one
invocation accepts at most 100 keywords. Each limit has its own usage error that
names the limit it hit. The bound exists so a single command stays a bounded,
reviewable request set rather than an open-ended crawl of Apple's public
endpoints.

## Sources and degradation

`score` composes three independent sources, and each one reports its own status
using the same vocabulary as the official Apple Ads source status already in
this tree: `available`, `empty`, or `unavailable`.

| Source | Endpoint | Requires | Contributes |
| --- | --- | --- | --- |
| `public_search` | public iTunes search, limit 200 | nothing | competitor ordering and `appCount` |
| `competitor_metadata` | public iTunes lookup, batched | nothing | competitor release dates |
| `search_term_popularity` | Apple Ads country-and-genre demand | `--genre`, Ads credentials | `popularity` |
| `app_rank` | derived from `public_search` | `--app` or `ASC_APP_ID` | this app's position |

An unavailable source is reported as unavailable. It is never replaced with a
zero, a default, or an estimate:

- Without `--genre`, or without working Apple Ads credentials,
  `popularity` is `null` and the source status names what was missing. Every
  other field still computes.
- If competitor metadata cannot be fetched, the two date-derived signals fall
  back to a documented one-year window (below) and the source is marked
  unavailable. The affected `rawSignals` entries carry empty date strings, so
  the degradation is visible in the output rather than implied. A partial
  lookup remains available when it returns some metadata, but its source error
  reports how many requested app IDs were omitted.
- A keyword whose own search fails becomes an `unavailable` row with its error.
  Its `difficultyScore`, `minDifficultyScore`, `isBrandKeyword`, and `appCount`
  serialize as explicit `null` rather than as zeros. A command fails outright
  only when *every* keyword failed, and then reports one stable representative
  error.

## Difficulty formula

Every value below is emitted in the JSON output next to the raw input it came
from, under `rows[].rawSignals[]`, so a score can be re-derived by hand.

### Per-app signals

All signals are clamped to `[0, 1]`.

```text
nRatingCount = clamp(userRatingCount / 10000, 0, 1)
nAvgRating   = avgRating <= 3 ? 0
             : clamp((avgRating - 3) / 2, 0, 1) * min(userRatingCount, 20) / 20
nAge         = 1 - clamp(daysSinceLastRelease / 365, 0, 1)
rpd          = userRatingCount / daysSinceFirstRelease
nRPD         = rpd <= 0   ? 0
             : rpd <= 1   ? rpd * 0.25
             : rpd < 100  ? 0.25 + 0.75 * ((rpd - 1) / 99)
             :              1
```

`daysSinceFirstRelease` comes from `releaseDate` and `daysSinceLastRelease` from
`currentVersionReleaseDate`. Both are floored at one day. A missing,
unparseable, or non-finite date degrades to **365 days**, which scores the app
as neither recently updated nor freshly launched.

`nAvgRating` is deliberately damped by rating volume: a 5.0 average from three
ratings is not evidence of a strong competitor, so the rating signal reaches
full weight only at 20 ratings.

### Keyword match ladder

The keyword is compared against each competitor's title and subtitle. Both sides
are normalized first: NFKC normalization, every non-letter/number/mark rune
replaced with a space, lowercased, and whitespace collapsed. NFKC folds
compatibility forms and unifies different encodings of the same character, but
it does not strip marks, so accents remain significant (`cafe` does not match
`café`). Exact-phrase rungs require a contiguous sequence of complete tokens;
a short keyword such as `ai` does not match inside `mail`.

The ladder is evaluated in order and the first rung that holds wins:

| Rung | Condition | Score |
| --- | --- | --- |
| `titleExactPhrase` | the title contains the keyword phrase | 1.0 |
| `titleAllWords` | every keyword token appears in the title | 0.8 |
| `subtitleExactPhrase` | the subtitle contains the keyword phrase | 0.5 |
| `combinedPhrase` | `"title subtitle"` contains the keyword phrase | 0.4 |
| `subtitleAllWords` | every keyword token appears in the subtitle | 0.4 |
| `none` | otherwise | 0 |

A keyword's reported `keywordMatch` is the strongest match across the sampled
competitors.

### Per-app score

```text
appScore = max(0,
    0.2 * nRatingCount
  + 0.1 * nAvgRating
  + 0.1 * nAge
  + 0.3 * keywordScore
  + 0.3 * nRPD)
```

`appScore` is reported on a 0-1 scale in `rawSignals[].appScore`.

### Keyword difficulty

The top 5 results are sampled. `appCount` is the size of the result window
Apple returned, capped at the 200-result request limit.

```text
avgS          = mean(appScores)
minS          = min(appScores)
nAppCount     = appCount <= 10  ? 0
              : appCount >= 200 ? 1
              : (appCount - 10) / 190
difficulty    = clamp(100 * (0.5 * nAppCount + 2 * avgS + 4 * minS) / 6.5, 1, 100)
minDifficulty = 100 * minS
```

`minS` carries the heaviest weight because the weakest app in the top results
is the realistic entry point: displacing it is what a new entrant actually has
to do. `minDifficulty` is reported unclamped so the raw floor stays visible.

When fewer than 5 apps were returned, the sample is too thin to produce a final
difficulty, so `difficulty` and `minDifficulty` are both reported as 1 and the
row is flagged with `fallback: true`. The observed `averageAppScore`,
`minimumAppScore`, and `normalizedAppCount` remain populated so the raw evidence
stays internally consistent.

### Worked parity vectors

Both vectors are pinned as tests in
`internal/cli/optimize/keywords_difficulty_test.go`.

An app with `averageUserRating` 4.5, `userRatingCount` 1000, a current version
released 30 days ago, a first release 400 days ago, and a `titleAllWords` match:

| Value | Result |
| --- | --- |
| `nRatingCount` | 0.1 |
| `nAvgRating` | 0.75 |
| `nAge` | 0.9178 |
| `nRPD` | 0.26136 |
| `appScore` | 0.50519 |

A keyword whose sampled app scores are `[0.8, 0.7, 0.6, 0.5, 0.4]` with an
`appCount` of 120:

| Value | Result |
| --- | --- |
| `avgS` | 0.6 |
| `minS` | 0.4 |
| `nAppCount` | 0.57895 |
| `difficulty` | 47.5304 |
| `minDifficulty` | 40 |

## Brand heuristic

`isBrandKeyword` reports whether a keyword most likely names the publisher of
the leading result rather than describing a category.

1. Tokenize the keyword and the rank-1 app's publisher name with `[a-z0-9]+`.
   If any keyword token is absent from the publisher tokens, the answer is
   `false`.
2. Otherwise, if the leader has at least 1000 ratings, the answer is `true`.
3. Otherwise, take the apps ranked 2-5 whose publisher differs from the
   leader's. If there are none, the answer is `false`. If the median of their
   rating counts is at least 10000, the answer is `true`; otherwise `false`.

Step 3 exists because a small app can rank first for its own brand name in a
crowded market; large independent neighbours make it more likely that the
leader's position is explained by the brand than by the keyword.

## Limitations

These are properties of the data, not defects to be papered over.

- **Subtitle is not available from the public endpoints.** Apple's public
  lookup response has no subtitle field, so in practice matches resolve on the
  title alone, and the subtitle rungs of the ladder stay unreachable through
  this data source. `rawSignals[].subtitle` is emitted as an empty string so
  the gap is visible rather than assumed. The ladder still implements those
  rungs and is tested, so a future subtitle source drops straight in.
- **Popularity has no storefront dimension of its own.** The Apple Ads demand
  source is scoped to a country and a genre and is strongly US-centric in
  coverage. Popularity is reported with the country, genre, and publication
  week it came from; it is not rescaled or interpolated to other storefronts.
- **Popularity requires a genre.** A missing genre is reported as an
  unavailable source naming the required flag rather than silently omitted.
- **Public endpoints are volatile.** Result ordering, window size, and
  metadata freshness are not contractual. A score is a snapshot of one
  observation, which is why `generatedAt` and every raw input travel with it.
- **`appCount` saturates at 200.** The public search request caps at 200
  results, and the difficulty normalization saturates at the same point, so
  keywords with more than 200 competing apps are indistinguishable on that
  signal.
- **Discovery is suggestion-scoped.** `discover` calls only the documented
  keyword and phrase suggestion endpoints. Each request asks Apple to order
  results by popularity descending and uses the requested `--limit` as its
  bounded page size (subject to Apple's platform page-size cap), stopping once
  that bounded prefix is collected. Campaign,
  eligibility, reporting, and recommendation data are not fetched, and
  `--genre` remains an optional report label that does not affect suggestions.
- **Release dates are read directly.** The shared public client type in
  `internal/itunes` does not expose `releaseDate` or
  `currentVersionReleaseDate`, so `internal/cli/optimize/keywords_metadata.go`
  reads those two fields from the same public lookup endpoint using that
  client's base URL and HTTP client. Folding them into the shared type is the
  natural follow-up.

## Keyword discovery

`discover` is the one command in this group that produces candidates rather
than evaluating them, and it does so only by reporting what Apple itself
suggests. It reads Apple's two official Ads suggestion endpoints:

| Purpose | Method and endpoint |
| --- | --- |
| App keyword discovery | `POST /v1/suggestions/keywords/query` |
| App phrase discovery | `POST /v1/suggestions/phrases/query` |

Neither requires an existing campaign. Terms from both endpoints are lowercased
and whitespace-collapsed, deduplicated across the two sources by retaining the
highest popularity occurrence (ties retain the first occurrence), then sorted
globally by popularity descending with missing popularity last. Each term is
reported with the endpoint it came from and any popularity Apple attached.
`--limit` caps the returned list and sets `truncated` when Apple offered more.

The report also carries `scoreKeywords`: a comma-separated list of the
suggestions that satisfy the keyword hygiene rules above, ready to pass
straight to `score --keywords`. Suggestions that are too short, too long, or
too many words, or contain the comma delimiter stay listed under `keywords`
but are excluded from that field, so nothing is silently dropped and nothing
that would be split into different keywords is handed onward.

Apple Ads is the only source for `discover`, so it is the only command in this
group that fails rather than degrades when its source is unavailable. The
failure names the credential flags and `asc ads auth login` so the fix is
actionable.

## Out of scope

Undocumented Apple endpoints are out of scope for this group. In particular,
the iTunes search **autocomplete** endpoint is not used for keyword discovery,
even though it would return more candidates: it is undocumented, unversioned,
and carries no compatibility commitment, which is exactly the kind of source
this tree's design position rules out. Only Apple's official, documented Ads
suggestion endpoints are used.

## Output contract

`score` and `discover` emit registered output contracts from `internal/asc`,
with table and Markdown renderers alongside JSON. `rank` remains a computed
camelCase result from `internal/cli/optimize`. These are computed results, not
App Store Connect envelopes and not mutation receipts. Added fields are
additive; removals follow the stability ladder.
