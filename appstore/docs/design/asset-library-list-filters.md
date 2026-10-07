# Asset Library list filters

Current image/video list help provides library selection, limit, next and
pagination. Add --category, --state, --spec-id, --reference-name, --id and --sort
only to the existing images list and videos list commands. No other read accepts these flags.

Apple's published OpenAPI 4.5.1 documents GET
/v1/appAssetLibraries/{id}/images and /videos with all six filters/sort inputs.
Array query parameters use style=form, explode=false (CSV). The authoritative
4.5.1 snapshot is maintained separately; this feature uses the public related
resource endpoints, not the private library media endpoint. filter[archived]
was rejected by the provider and is absent from the specification; archived
assets can instead be selected using --state ARCHIVED.

The earlier October 6 live probes independently verified image category/state/
spec filters and referenceName/-lastModifiedDate sorting, plus video state
filtering. Official support of the additional combinations does not imply a
new independent live verification.

    asc asset-library images list --library-id LIBRARY_ID --category CREATIVE_ASSETS --state APPROVED --sort referenceName
    asc asset-library videos list --library-id LIBRARY_ID --state FAILED --paginate

CSV tokens are trimmed; empty members and malformed tokens return usage errors
before authentication or HTTP. Categories accept the two observed categories:
CREATIVE_ASSETS and APP_SCREENSHOTS_AND_PREVIEWS. States use structural token
validation instead of a closed enum so Apple can accept future states. spec-id
accepts resource-ID tokens. Reference names accept arbitrary nonempty CSV
members, preserving interior spaces and punctuation. Asset IDs use the existing resource-ID token syntax.
Sort accepts referenceName, createdDate and lastModifiedDate,
with an optional leading minus for descending order. Providers govern support
and state semantics. No client-side filtering or silent value omission occurs.

Filters encode as filter[category], filter[state], filter[specId],
filter[referenceName] and filter[id]; sorting uses sort. Every selector conflicts with --next, even if explicitly empty, preserving
provider-owned next links. --paginate follows those links unchanged. Default
JSON remains Apple's envelope including unknown fields; table/Markdown behavior
is unchanged. Flags are additive and existing commands retain behavior.

Focused CLI tests cover exact encoded requests for both media lists including
reference-name escaping and createdDate ordering, self-link library selection, future states, malformed input with zero HTTP, specific next
conflicts and nonempty filtered pagination. Full gates, generated help, model
reviews and live built-command smoke are parent integration responsibilities.
