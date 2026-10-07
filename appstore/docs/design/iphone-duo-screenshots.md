# iPhone Duo screenshot support

The existing `asc screenshots sizes --display-type IPHONE_DUO` and
`asc screenshots upload --device-type IPHONE_DUO` reject the new device before
any network request. Add the runtime-verified `APP_IPHONE_DUO` screenshot slot
to the shared screenshot catalog. Existing upload, metadata discovery, and
readiness validation then use the same dimensions and API value. Migration and
frame output dimension inference also recognize this slot.

## Verified API contract

On October 6, 2026, the public API accepted `POST /v1/appScreenshotSets` with
`data.type: appScreenshotSets`, `attributes.screenshotDisplayType:
APP_IPHONE_DUO`, and an `appStoreVersionLocalization` relationship belonging to
the disposable ASC Test app. It returned HTTP 201 and the identical display
type. The temporary set was deleted with HTTP 204. Apple's published OpenAPI
4.5 does not yet include this enum; keep the snapshot unchanged and explicitly
allow the verified runtime enum in the schema compatibility test.

The live asset reference data and App Store Connect UI list four supported PNG
or JPEG screenshot sizes: 1398×2034, 2034×1398, 2007×2853, and 2853×2007.
This value is passed unchanged to Apple, rather than aliased to an existing
phone slot. The iMessage enum and preview upload enum require independent
verification and are outside this change.

## CLI behavior and compatibility

- `asc screenshots sizes --display-type IPHONE_DUO --output json` lists all
  four dimensions. `--all` includes the device; default focused output remains
  unchanged.
- `asc screenshots upload --version-localization ID --path FILE
  --device-type IPHONE_DUO` validates dimensions and uses the existing upload
  workflow and receipt shape. Existing JSON, table, and markdown output
  contracts remain unchanged.
- Migration recognizes `iphone duo`, `iphone_duo`, and `iphoneduo` filename
  hints and the four dimensions.
- Frame output size inference recognizes Duo dimensions. No unverified device
  bezel or Koubou device definition is introduced.
- Existing usage errors and exit codes remain unchanged. Standard phone
  screenshot dimensions are rejected for the Duo slot.

## Verification

Establish RED for root CLI size listing and upload dry-run, dimension validation,
migration inference, and frame output inference. Reach GREEN with catalog and
inference changes. Existing published-enum coverage remains intact. Run adjacent
package tests and repository gates. Live set creation proved API recognition;
a complete simulator capture and upload is a separate verification layer.
