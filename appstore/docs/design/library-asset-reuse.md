# Library asset reuse

The shipped `localizations placements create` accepts an image only for a default
product page header or search asset. CPP only has placement reads; PPO and events
have legacy media sets. Add shared placement list/create/swap/delete commands
under the groups below, plus reorder for version/CPP/PPO localizations:

- `asc localizations placements`
- `asc product-pages custom-pages localizations placements`
- `asc product-pages experiments treatments localizations placements`
- `asc app-events localizations placements`

Official OpenAPI 4.5.1, published on 2026-10-06, documents placement creation,
all four parent placement lists, single deletion, and ordering for version, CPP,
and PPO localizations. Ordinary creation supports image or video with each parent
relationship. The captured signed-in editor bundle `45adf3d8.1f27bafa.js` supplies
additional evidence for the live-verified `placementToSwapOut` extension, which
is absent from the official create relationship properties. Browser abstractions
alone do not establish public acceptance for every parent or operation.

Events expose list/create/swap/delete, but do not expose reorder. The official
`AppAssetLibraryPlacementOrderingRequestCreateRequest` has only version/CPP/PPO
parent branches. An owned public event ordering probe returned 409 with unknown
appEventLocalization and missing required fields, confirming that event ordering
must be rejected locally before HTTP.

## Requests

Creation is `POST /v1/appAssetLibraryPlacements`, JSON:API `data.type`
`appAssetLibraryPlacements`. Attributes are `placementType` and `placementGroup`.
Exactly one media relationship is `image` (`appAssetLibraryImages`) or `video`
(`appAssetLibraryVideos`). The localization relationship maps as follows:

| Resource type | Relationship |
| --- | --- |
| appStoreVersionLocalizations | appStoreVersionLocalization |
| appCustomProductPageLocalizations | appCustomProductPageLocalization |
| appStoreVersionExperimentTreatmentLocalizations | appStoreVersionExperimentTreatmentLocalization |
| appEventLocalizations | appEventLocalization |

Creative header/search and event card/details placements use `DEFAULT_PROFILE`.
Native screenshot/preview placements require an explicit `--placement-group`,
for example `IPHONE_DUO_PROFILE`; Apple validates device/media compatibility.
Screenshots require images; previews require videos. Events only accept card and
details types. CPP/PPO creation does not expose iMessage screenshots. CPP list
filters retain `IMESSAGE_APP_SCREENSHOT`, which official OpenAPI 4.5.1 permits
and a public GET accepted with HTTP 200 and an empty collection. Read filter
support does not establish creation support. Neither creation nor reordering
submits assets for review or promises publication.

Reordering is `POST /v1/appAssetLibraryPlacementOrderingRequests`, JSON:API
`data.type` of the same name, attributes `placementGroup`, relationships
`orderedPlacements` (placement IDs in caller order) and one version/CPP/PPO localization relationship.
The browser requests `include=orderedPlacements`; the CLI omits this optional
response expansion because it produces a mutation receipt from the returned
request ID and preserves the raw client mutation-query guard. `--placement-ids` requires unique, nonempty IDs;
Apple validates that they belong to the localization and group. Reordering never
deletes omitted placements locally. No inferred PATCH ordering operation is used.

Reads preserve Apple's complete resource envelope and use only observed filters:
`filter[placementType]`, `filter[placementGroup]`; include image/video, sort
`placementGroupPosition`, existing next/pagination behavior. The generic confirmed
placement delete is shared unchanged and preserves library media.

## Output and verification

Required flags, media exclusivity, native media kind, group syntax, supported
placement type, and duplicate/empty reorder IDs fail before auth/HTTP with usage
exit 2. Reads retain unknown envelope fields. Creation retains the existing
camelCase receipt and adds optional `videoId`; ordering uses a registered
camelCase receipt with request ID, localization, group, and ordered placement IDs.
Malformed or denied mutation responses cannot print a success receipt.

Focused RED/GREEN CLI HTTP tests cover four creation/swap/deletion bindings,
three supported ordering parents, local rejection of event reorder, video/native
payloads, order preservation, new input errors, and event/PPO read envelopes.
Existing default header/search image and generic deletion tests remain in force.
The coordinating agent owns broad gates, model reviews, built-binary and public
provider verification before publication.

## Atomic swaps and multiple removals

`placements swap --localization-id ID --placement-id OLD_ID --image-id IMAGE_ID
--placement-type TYPE --confirm` sends one POST to `/v1/appAssetLibraryPlacements`.
It reuses creation's media/context validation and links `placementToSwapOut` to
`appAssetLibraryPlacements`. The live-verified swap relationship is an undocumented extension to official
OpenAPI 4.5.1. The captured swap contract sends placement type but
omits placement group. There is no group flag and no separate deletion or fallback
removal. Apple validates existing group/context compatibility and requires the
replacement to have the same media type as the existing placement. Image and video
IDs are mutually exclusive; native screenshots require images, previews videos.
A registered camelCase swap receipt includes the new ID and previousPlacementId.
CPP image swaps returned 201 in the coordinating agent's public-provider probe.
An image-to-video swap returned 409 with that same-media-type requirement,
without removing the existing placement. Final built-CLI verification accepted a CPP video-to-video header swap, removed
the previous placement, and read back the new video linkage. Native Duo preview
and event-card/details video assignments also passed with their exact media IDs.
Independent processed HLS delivery and bounded decoding establish playable media,
while placement acceptance does not establish App Review or storefront publication.
After explicit removal of the owned CPP image placement, the encoded header
video was accepted by a new placement POST with HTTP 201 and state ACTIVE.
That establishes video assignment acceptance, not playback or publication.

The website sends POST `/v1/appAssetLibraryPlacementsDeletionRequests` with a
placements relationship, but the public API returned 404 PATH_ERROR for that
route. The CLI therefore retains single `delete --id` and adds exclusive
`delete --placement-ids ID1,ID2 --confirm` implemented as sequential public DELETE
requests. Each DELETE receives its own `ASC_TIMEOUT` budget; caller cancellation
and any caller deadline still bound the entire operation. It does not send the
private website batch route or promise atomicity.
IDs govern removal across contexts; no context/group is inferred or sent.
Library media remains available. Processing stops at the first failure and emits
an exported camelCase progress receipt with placementIds, deletedPlacementIds,
deleted=false, and failedPlacementId, then returns a nonzero error. Full success
sets deleted=true. Earlier removals are not rolled back or retried; inspect state
before retrying an uncertain failure.

Focused tests cover exact swap relationships without placementGroup, all four
context bindings, confirmation/exclusive media IDs, sequential deletion ordering,
partial progress and stop behavior, human renderers, and denial/malformed swap
responses without cleanup side effects. Earlier experimental bulk POST tests were
superseded by the live 404 and a RED/GREEN sequential-deletion regression.
