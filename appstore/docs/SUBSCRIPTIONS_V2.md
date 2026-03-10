# Subscriptions V2 Taxonomy

This note captures the canonical V2 subscriptions command matrix after the
full removal of the legacy flat `asc subscriptions` surface.

## Canonical Families

- `asc subscriptions groups ...`
- `asc subscriptions list|get|create|update|delete ...`
- `asc subscriptions pricing summary ...`
- `asc subscriptions pricing prices ...`
- `asc subscriptions pricing price-points ...`
- `asc subscriptions pricing availability ...`
- `asc subscriptions offers introductory ...`
- `asc subscriptions offers promotional ...`
- `asc subscriptions offers offer-codes ...`
- `asc subscriptions offers win-back ...`
- `asc subscriptions review screenshots ...`
- `asc subscriptions review app-store-screenshot ...`
- `asc subscriptions review submit ...`
- `asc subscriptions review submit-group ...`
- `asc subscriptions promoted-purchases ...`
- `asc subscriptions localizations ...`
- `asc subscriptions images ...`
- `asc subscriptions grace-periods ...`

## Old To New Mapping

All legacy paths have been removed. Only the canonical V2 paths are supported.

| Old path | Canonical V2 path |
| --- | --- |
| `asc subscriptions pricing` (flat) | `asc subscriptions pricing summary` |
| `asc subscriptions prices ...` | `asc subscriptions pricing prices ...` |
| `asc subscriptions price-points ...` | `asc subscriptions pricing price-points ...` |
| `asc subscriptions availability ...` | `asc subscriptions pricing availability ...` |
| `asc subscriptions introductory-offers ...` | `asc subscriptions offers introductory ...` |
| `asc subscriptions promotional-offers ...` | `asc subscriptions offers promotional ...` |
| `asc subscriptions offer-codes ...` | `asc subscriptions offers offer-codes ...` |
| `asc subscriptions win-back-offers ...` | `asc subscriptions offers win-back ...` |
| `asc subscriptions review-screenshots ...` | `asc subscriptions review screenshots ...` |
| `asc subscriptions app-store-review-screenshot ...` | `asc subscriptions review app-store-screenshot ...` |
| `asc subscriptions submit ...` | `asc subscriptions review submit ...` |
| `asc subscriptions groups submit ...` | `asc subscriptions review submit-group ...` |
| `asc offer-codes ...` (root) | `asc subscriptions offers offer-codes ...` |
| `asc win-back-offers ...` (root) | `asc subscriptions offers win-back ...` |
| `asc promoted-purchases ...` (root) | `asc subscriptions promoted-purchases ...` or `asc iap promoted-purchases ...` |

## Canonical Flag Direction

Canonical V2 paths use typed selectors:

- `--group-id`
- `--reference-name`
- `--subscription-id`
- `--offer-code-id`
- `--price-point-id`
- `--screenshot-id`
- `--availability-id`
- `--territories` (comma-separated territory list)

Leaf commands under wrapped families (introductory, promotional, win-back)
still use `--id` for their own resource IDs. These are correct for the
underlying API resources but may be normalized to typed selectors in a
future pass.

## Compatibility Rules

- Legacy paths are **not** registered and will fail as unknown commands.
- There are no hidden deprecated shims or compatibility wrappers.
- The canonical V2 paths are the only supported invocation surface.
- Error prefixes in canonical wrappers reflect the V2 command path.
