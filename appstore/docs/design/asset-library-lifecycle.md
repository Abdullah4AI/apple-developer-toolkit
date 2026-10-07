# Asset Library lifecycle commands

The current help exposes image reads/upload and video collection reads. Extend
these existing media groups with rename, archive, unarchive and delete; videos
also gain view, placements and set-poster-frame. Existing invocations and raw
JSON read envelopes remain compatible.

Apple's OpenAPI 4.5.1 publishes PATCH and DELETE on
`/v1/appAssetLibraryImages/{id}` and `/v1/appAssetLibraryVideos/{id}`.
The update schemas accept referenceName and archived; the video update schema
also accepts previewFrameTimeCode. DELETE has no request body. These contracts
match the signed-in October 6 browser bundle 45adf3d8.1f27bafa.js; contextual
excerpts are recorded in maintainer lifecycle-contracts.json evidence.
The response state discriminator describes returned attributes, not mutation
eligibility. Public rename returned 200 with restored readback. Other new
mutations await independent public acceptance checks.

Examples:

    asc asset-library images rename --id IMAGE_ID --name "Header"
    asc asset-library images archive --id IMAGE_ID --confirm
    asc asset-library images unarchive --id IMAGE_ID
    asc asset-library images delete --id IMAGE_ID --confirm
    asc asset-library videos view --id VIDEO_ID
    asc asset-library videos placements --id VIDEO_ID --paginate
    asc asset-library videos set-poster-frame --id VIDEO_ID --time-code 00:00:01:00

All updates send only the observed attribute. Archive and delete require
--confirm. Delete eligibility and asset/submission state restrictions are
validated by Apple; placement removal is a separate operation. The commands do
not submit App Review or publish a release. Video poster updates affect every
use of the library video. Timecodes accept HH:MM:SS:FF or HH:MM:SS.mmm;
frame values are not interpreted as a particular playback frame rate.

Reads preserve provider envelopes. Mutations return registered camelCase
receipts (assetId, assetType, action, and the submitted attribute), with stdout
data and stderr errors. Invalid flags return usage errors before authentication.
Focused command tests establish RED for missing commands, exact PATCH/DELETE
requests, confirmation/validation, receipt output, raw video envelopes and
provider failures. No broad gates or live mutations are run by this worker;
these remain parent integration gates.

The owned public archive probe returned HTTP 409 STATE_ERROR.INVALID_ASSET_STATE
with "Only an approved asset can be archived" on a prepared image. This confirms
the request reaches provider eligibility validation; it does not prove a successful
archive/unarchive transition. The CLI preserves this provider refusal unchanged.

## Built CLI acceptance (2026-10-07)

On owned fixtures in ASC Test 6759231657, the exact built CLI renamed an image
and a placed video, read back both changes, and restored their original names.
It changed the placed video's poster timecode, verified the new value, and
restored the fresh baseline while retaining its existing CPP placement.
Prepared-image archive and unarchive both correctly returned Apple's state
refusals (only approved assets can be archived; only archived assets restored).
No successful archive-state transition is claimed without an approved fixture.
The CLI deleted the owned, unused FAILED video and the subsequent public GET
returned 404. This disproves the absolute help claim that only Prepare for
Submission assets are deletable; current help leaves eligibility to Apple.
No App Review submission occurred.
