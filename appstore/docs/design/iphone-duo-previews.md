# iPhone Duo app previews

`asc video-previews upload --device-type IPHONE_DUO` currently rejects Apple's
new preview type before the request. Add `IPHONE_DUO` to the existing central
preview allowlist and document an upload example. All consumers of the shared
preview normalization, including metadata and custom page uploads, retain the
existing workflow and output contracts.

## API evidence

On October 6, 2026, authenticated public API `POST /v1/appPreviewSets` accepted
`data.type: appPreviewSets`, `attributes.previewType: IPHONE_DUO`, and the
existing `appStoreVersionLocalization` relationship on ASC Test. The response
returned HTTP 201 and echoed `IPHONE_DUO`; GET returned HTTP 200 with the same
value. The empty temporary set was deleted with HTTP 204. The request shape
already matches `AppPreviewSetCreateRequest` in the published OpenAPI snapshot;
its `PreviewType` enum has not yet caught up. Leave the official snapshot intact.

The App Store Connect reference and UI describe preview dimensions of 1920×886
or 886×1920, duration 15–30 seconds, frame rate 23–30 fps, and required audio.
This change permits the verified device value through the existing uploader;
it does not add new local video encoding or dimension checks. A successful set
creation proves API recognition, separately from video processing or review
acceptance. No iMessage preview value is introduced.

## Contract and validation

Existing `--device-type`, timeout, output, upload receipt, capacity, replacement,
and confirmation behavior remain unchanged. `APP_IPHONE_DUO` follows the
existing optional `APP_` prefix normalization. Unknown preview types continue to
fail before requests. No new flags or command groups are necessary.

The RED root CLI test demonstrates the unsupported value. GREEN verifies the
exact create-set request value and localization relationship, reuses the full
preview upload sequence, and checks the COMPLETE receipt. Run adjacent preview
command tests, regenerate help documentation, and complete all repository gates
and the required local review. Real provider video processing remains a separate
live verification step.
