# API Canary & Capture Log

Dated live-canary transcripts, authenticated browser/source captures, disposable test fixture records, and historical verification notes extracted from [API_NOTES.md](API_NOTES.md). For active operational contracts and endpoint rules, see [API_NOTES.md](API_NOTES.md).

## Web App Privacy Data Usage Updates

- Live canary on 2026-09-04 using a disposable app and a redacted web session verified the private `PATCH /iris/v1/appDataUsages/{usageID}` contract for a same-category, same-purpose `DATA_LINKED_TO_YOU` to `DATA_NOT_LINKED_TO_YOU` identity flip. The request uses the existing `appDataUsages` JSON:API resource and its `dataProtection` relationship; Apple returned `200`, and a fresh GET showed the same usage ID with the new protection.
- A second live canary on 2026-09-05 using the same disposable app verified the reverse `DATA_NOT_LINKED_TO_YOU` to `DATA_LINKED_TO_YOU` transition. The temporary `EMAIL_ADDRESS`/`APP_FUNCTIONALITY` tuple returned `200`, and a fresh GET retained the same usage ID with the new protection. The exact pre-canary `DATA_NOT_COLLECTED` baseline was restored and matched on a fresh GET.
- The canary seeded the tuple with `POST` (`201`), confirmed it by GET, then restored the semantic baseline by deleting it (`204`) and creating the `DATA_NOT_COLLECTED` declaration (`201`). Direct restore attempts via `POST` or `PATCH` to `DATA_NOT_COLLECTED` returned `409 STATE_ERROR`.
- The direct mutations advanced the remote `lastPublished` value while `published` remained `true`, despite no publish endpoint being called.

## Analytics & Sales Reports

- Although Apple's current Sales Reports endpoint table lists `SUBSCRIPTION` `1_3`, live verification in PR #1842 proved `1_4` succeeds and is required by some accounts, so both are accepted and `1_4` remains the default.

## Tax Categories and Transaction Tax Reports

- Verified against the App Store Connect OpenAPI snapshot in `docs/openapi/` and the App Store Connect web-client source captured for issue #2299.
- For App Information tax categories (`asc web apps tax-category`), the request and response shapes are source-backed. A disposable-app canary verified explicit application tax configuration and readback. PATCH, condition changes, and account-specific errors remain unverified; selecting the legally correct classification remains the operator's responsibility.
- For In-App Purchase tax categories (`asc web iap tax-category`), a browser canary on 2026-09-06 against disposable app `6759231657` and IAP `6760268101` verified ADDON catalog discovery, create, update, explicit condition clearing, and delete/readback; the disposable IAP was restored and no resources were left behind. This is browser evidence only: final CLI live execution remains unverified.

## Sandbox Testers

- Historical public v1 create required password confirmation, a secret question/answer, and a birth date; that removed v1 contract does not establish that those fields are accepted by the current private web flow.
- The current App Store Connect web bundle's New Tester form exposes first name, last name, email, password, confirm password, and country. Its two `POST /sandbox/v2/account/validateFields` requests project the first three fields, then add `acAccountPassword`; `POST /sandbox/v2/account/create` sends exactly `firstName`, `lastName`, `acAccountName`, `acAccountPassword`, and `storeFront`. `confirmPassword` is UI-only, and the bundle contains no `secretQuestion`, `secretAnswer`, or `birthDate` fields. This is a source-backed request shape; it does not prove a successful live create or acceptance of extra portal fields. See issue #2294.
- The same bundle lists testers with `GET /sandbox/v2/provider/account/list?limit=50`; the only live list response captured was the empty `{"totalAccounts":0,"totalInactiveAccounts":0,"accounts":[]}` response. No continuation or pagination contract was captured. Account rows include `id` and `isInFamily`; the UI excludes family members from deletion.
- Delete status/body, post-delete disappearance, and account-wide mutation acceptance for `POST /sandbox/v2/account/delete` remain unverified because no sandbox tester rows were available during capture; do not infer them from the legacy public v1 surface.

## App Store Regulations & Permits declarations

- A sanitized live CLI canary on 2026-09-05 used disposable app `6759231657` and a locally built candidate CLI. The `overall-yes`, `regional-gbr-false`, and `overall-no` CLI steps all exited 0 with exact readbacks; the regional `GBR` No path was exercised while the overall declaration was Yes. One exact raw-form restore PUT returned 200 and its readback matched the saved form. The final baseline, session identity, temporary-file cleanup, and no-uncertain-retry checks all passed. This verifies the current CLI's negative regional path and restoration on that disposable app; it does not prove arbitrary affirmative legal/contact values.
- The captured Business frontend identifies `personalServiceDeclaration`, `personalServiceApps`, and `nonPersonalServiceApps` in dynamic DAC7/MRDP/ITA/SERR compliance forms. The current account has no qualifying live requirement/form fixture to verify the applicable form identity, constraints, and write postcondition.

## Web-session API keys

- The captured App Store Connect individual-keys frontend bundle confirms the actor-filtered v2 list (`GET /iris/v2/apiKeys?include=createdByActor,revokedByActor&filter[createdByActor]=USER:{actorID}`), the empty `POST /iris/v2/apiKeys` create request, browser P-256 key generation with PKCS#8 private and SPKI public export, public-key registration by v2 `PATCH`, and v2 inactive-state revocation. Before the detail/generation flow it checks `GET /iris/v1/apiAccesses` for an `APPROVED` access resource and obtains the actor ID from initialized page state. This capture contains no `/iris/v1/users/` or `AuthSession.UserEmail` request.
- No individual key was created or revoked during capture or validation, so provider acceptance of a newly created individual key and the individual revoke path remain unverified.
- A sanitized 2026-09-05 live canary created and revoked one newly created temporary team key through an authenticated web session. The pre-existing keys were unchanged, the new key was verified inactive, and temporary files were removed. This verifies the team request path for that account and session.

## App price schedules (`POST /v1/appPriceSchedules`)

- Live runs against disposable app `6759231657` on 2026-09-26 and 2026-09-27 (each one snapshot, attempts, restore, and verification in a single serialized session) returned HTTP 409 for past start dates, single future start dates, non-base-territory manual prices, and unknown price point IDs. The raw bodies are checked in under `internal/cli/cmdtest/testdata/pricing_schedule_create_409/`, byte for byte including Apple's per-response error `id`.
- At 23:22 UTC on 2026-09-26, `startDate` `2026-09-26` returned 201. At 00:03 UTC on 2026-09-27, `startDate` `2026-09-27` (the UTC date, and the CLI's default at the time) returned the future-start 409 (`Entire timeline must be covered for USA. The first interval has a start date 2026-10-26T00:00 in the future`), while `2026-09-26` (the US Pacific date) returned 201. Omitting `startDate` also returned 201 and restored the original `startDate: null` / `endDate: null` interval.
- Apple's base64 price resource IDs encode interval boundaries as Unix seconds (`sd`, `ed`); a captured in-app purchase schedule encodes `ed` of one interval and `sd` of the next as the same value, `1771228800` (`2026-02-16 00:00 PST`, `08:00 UTC`).
- No subscription-specific live capture exists for `subscriptionPrices` start-date timezone boundaries; the CLI applies the same US Pacific day as app and in-app purchase schedules.

## Developer Portal iCloud containers

- fastlane Spaceship still uses the older form endpoint `POST /services-account/QH65B2/account/cloudContainer/addCloudContainer.action` with `name`, `identifier`, and `teamId` ([portal_client.rb#L483-L492](https://github.com/fastlane/fastlane/blob/5faa062de92fd21e5674efcfde1a8359de7dd861/spaceship/lib/spaceship/portal/portal_client.rb#L483-L492)) and sends the full `iCloud.`-prefixed identifier unchanged ([cloud_container.rb#L59-L62](https://github.com/fastlane/fastlane/blob/5faa062de92fd21e5674efcfde1a8359de7dd861/spaceship/lib/spaceship/portal/cloud_container.rb#L59-L62), [commands_generator.rb#L263-L270](https://github.com/fastlane/fastlane/blob/5faa062de92fd21e5674efcfde1a8359de7dd861/produce/lib/produce/commands_generator.rb#L263-L270)). The captured list read shows the current portal reads iCloud containers from `/services-account/v1/cloudContainers`, not fastlane's `listCloudContainers.action`.
- Live capture (2026-09-29, CLI built from `414167918`, one maintainer-approved permanent container): the sequence was `listTeams.action` 200, the visible and hidden preflight reads 200, `POST https://developer.apple.com/services-account/v1/cloudContainers` **201**, then the visible and hidden read-backs 200. The receipt was `containerId` `7PU5FAD3YR`, identifier `iCloud.com.rorkai.asc.capture.t1790620906`, name `asc capture container`, prefix `YQZQG7N4WG` (the team prefix), and `hidden:false`. A separate `asc web icloud-containers list` showed the same ID, identifier, and name. This container is permanent and cannot be removed. It stays in the team as the capture artifact; do not create another one to re-verify.
- The create response body was not recorded. `internal/web/testdata/icloud_container_create_capture.json` holds the captured request body and status, plus the read-back resource. Its `canEdit`, `canDelete`, and `responseId` values are inferred. The replay test accepts both a JSON:API resource body and an empty 201 body, because the read-back is the authority either way.

## Web-session Resolution Center

- No disposable live thread/draft fixture was available during implementation of `asc web review reply` and `asc web review drafts create|update|delete`, so Apple provider acceptance of these writes remains unverified.

## Web-session app create conflicts (`POST /iris/v1/apps`)

- Captured live on 2026-09-29 against disposable app `6759231657`: re-running `asc web apps create --name "ASC Test 20260216074703" --bundle-id com.rudrank.asc.throwaway20260216074703 --sku asc-test-20260216074703 --auto-rename=false` returned HTTP 409 with three `errors[]` entries whose codes are, in order, `ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE`, `ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE`, and `ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE.SAME_ACCOUNT`. The CLI exited 5 with `web apps create failed: web api error (status 409) ... codes=[...]`.
- Only the status and the codes were captured. The `detail`, `title`, and `source` of each entry were not printed, so which attribute (name, SKU, or bundle ID) each entry refers to is unknown. The captured-shape test fixture carries codes only. A second fixture adds an app-name `detail` and `source` pointers to exercise the `--auto-rename` interaction; those fields are **synthetic** (inferred, not captured) and are labeled as such in `internal/cli/web/web_apps_create_if_exists_test.go`.

## Web-session app distribution method

- The current Apple frontend's pricing action supplies an inner `apps` resource to its request action. The shared Iris transport serializer (`module 60285`, helper `Oe`) converts that resource to `JSON.stringify({data: resource})` before `fetch` (`it`), so the wire request is JSON:API.
- `asc web apps distribution set` writes are based on the captured frontend constructor and local HTTP tests; no live Apple mutation or live eligibility/success proof was performed.

## Web-session custom-distribution Apple Account recipients

- A redacted authenticated browser canary on 2026-09-06 exercised the workflow in CUSTOM mode, including `POST /iris/v1/customAppUsers` with HTTP 201, an app-scoped list with HTTP 200, and `DELETE /iris/v1/customAppUsers/{id}` with HTTP 204. The disposable app was restored to its original public distribution and education settings; fresh app, user, and organization reads showed the baseline and zero remaining rows. This is browser contract evidence, not final CLI live verification, and no organization write was performed.
- The live canary does not establish role behavior, duplicate rejection, organization recipient rows, or a second collection page.

## App transfer status (web session)

- On 2026-09-06, an authenticated read of disposable app `6759231657` (`GET /iris/v1/apps/{appId}?include=appTransferRequest`) returned HTTP 200, the expected `apps` resource ID, `relationships.appTransferRequest.data: null`, and an empty `included` array. The related URL `/iris/v1/apps/{appId}/appTransferRequest` returned HTTP 409 with `ENTITY_ERROR.RELATIONSHIP.INVALID` for that fixture. No pending-transfer response was observed live.

## Last-compatible version settings (`downloadable`)

- A web-session read (`asc web apps last-compatible-version view`) briefly existed for this screen and was retired before it reached a release. It mirrored App Store Connect's own iris request (`GET /iris/v1/apps/{id}?include=appStoreVersions&fields[appStoreVersions]=...,downloadable,...&limit[appStoreVersions]=2000`), which required a web session and offered no write. The public API path supersedes it in both directions.

## App Store version creation conflicts (`POST /v1/appStoreVersions`)

- Captured live on 2026-09-15: Apple answers HTTP 409 with `ENTITY_ERROR.RELATIONSHIP.INVALID` ("You cannot create a new version of the App in the current state.") both when the platform already has an unreleased version and, followed by `ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE`, when the version string is already used.

## Web-session review submissions (iris)

- Verified live on 2026-09-03: `GET /iris/v1/reviewSubmissions/{id}/items` rejects `include=appStoreVersionExperimentV2` with HTTP 400 `PARAMETER_ERROR.INVALID` even though the public OpenAPI snapshot lists that relationship.

## TestFlight text validation (What to Test)

- A live probe on 2026-09-25 against `betaAppLocalizations` `description` on a disposable app (the app had no builds, so `betaBuildLocalizations` `whatsNew` could not be written directly) rejected the write with `An attribute value has invalid text.: Text contains invalid characters/formats.` for: any `<` (including `a<b` and `x < y`); decomposed `Cafe` + U+0301 that NFC would compose; U+0301 after `q` or at the start of the text; U+1AB0; U+20D7; and every tested emoji (`❤`, `❤️`, U+FE0E, keycap, ZWJ family, skin tone, `😀`).
- The same probe accepted `>`, `&`, the literal text `&lt;`, newlines, precomposed Vietnamese, and script-specific combining marks: Devanagari (`हिंदी में परीक्षण करें`, including a leading U+0902), Thai tone marks, Hebrew niqqud, Arabic harakat, and the Cyrillic titlo U+0483.
- A live probe on 2026-09-26 against `betaBuildLocalizations` `whatsNew` (PATCH on an existing localization, disposable app) answered 409 `An attribute value has invalid text.: Text for whatsNew contains invalid characters:'[…]'` and listed each offending character.
  - Rejected: every supplementary-plane character (U+1F600, ZWJ family, skin tone U+1F3FD, regional indicators U+1F1FA/U+1F1F8, U+1F004, U+1F130, U+1D400, U+1D11E, U+10330, U+20000); Variation Selectors U+FE00, U+FE0E, U+FE0F (even after a plain `a`); enclosing marks U+20DD and keycap U+20E3; Control Pictures U+2400 and OCR U+2440; Box Drawing U+2500/U+257F and Block Elements U+2588/U+2591; Miscellaneous Symbols U+2600–U+26FF (`☀☂☺♠♥⚠★☆`, U+26FF); Dingbats U+2701–U+27BF (`✓✔✖✗✂✨✅❤`, U+2794, U+27BF); Miscellaneous Mathematical Symbols-A U+27C2 and -B U+2980; Braille U+2801; Supplemental Mathematical Operators U+2A00; Miscellaneous Symbols and Arrows U+2B00–U+2BFF (`⬆⭐⭕⬛`); Private Use U+E000 and U+F8FF; U+FFFD.
  - Accepted: Arrows (U+2192, U+2194, U+21FF), Supplemental Arrows-A U+27F6 and -B U+2934, Letterlike (`™`, `ℹ`, `№`), `©`, `®`, Miscellaneous Technical (U+2300, `⌘⌚⌛`, U+23E9, U+23F0, U+23FF), Enclosed Alphanumerics (U+2460, `Ⓜ`, U+24FF), Geometric Shapes (U+25A0, U+25AA, `▶`, U+25CF, U+25FB, U+25FF), Mathematical Operators, General Punctuation (`•…—‼⁉`), `€`, U+3030, U+303D, U+3297, U+3299, newlines, CRLF, tabs, NBSP, soft hyphen, ZWSP, ZWJ/ZWNJ inside Devanagari and Persian, LRM/RLO, U+FEFF, and Latin, Greek, Cyrillic, IPA, CJK, Hangul, Thai, Hebrew, Arabic, Devanagari, and Tamil text.
  - Whole scripts and presentation forms rejected in the same sample (left to App Store Connect rather than refused locally because neighboring characters such as IPA and U+F900 were accepted): Ethiopic, Cherokee, Khmer, Georgian, Tibetan, Mongolian, Yi, Vai, Glagolitic, U+02C6 (Spacing Modifier Letters), Alphabetic Presentation Forms U+FB01, Arabic Presentation Forms U+FDFD, Vertical Forms U+FE10, Small Form Variants U+FE50, and CJK Compatibility U+3300.
- A live check on 2026-09-25 showed App Store Connect matches `filter[locale]` on `betaAppLocalizations` case-insensitively (`fr-fr` and `FR-FR` both returned `fr-FR`).

## App Store review details (`appStoreReviewDetails`)

- Field length, contact requirement, and phone validation rules were derived from App Store Connect responses observed in agent runs between 2026-09-21 and 2026-09-30:
  - `notes`: Rejected over 4000 characters with `An attribute value is too long.: Review Notes cannot be longer than 4000 characters.` (agents trimmed and retried up to 7 times, often measuring bytes with `wc -c`).
  - Contact fields on create: `POST /v1/appStoreReviewDetails` sending only `contactEmail` failed with `You must provide a value for the attribute 'contactFirstName' with this request`; sending first name, last name, and email failed on `'contactPhone'`. An update sending only `contactEmail` to a detail without a first name also failed on `contactFirstName`.
  - `contactPhone`: Rejected 7 digits after `+503`, and 7- and 11-digit national numbers after `+1`, with `The phone number must be in a valid format. Preface the phone number with '+' followed by the country code (for example, +44 844 209 0611)` (`betaAppReviewDetails` answered `The format of the phone number is invalid`). Numbers without a plus sign (10- and 11-digit North American numbers) were accepted by the API.

## Authentication & Rate Limiting

- The public storefront retry path is validated with deterministic `httptest` coverage for status boundaries, Retry-After parsing/capping, response-body draining, request replay, concurrency, and cancellation; it does not perform live mutations.
- The bulk-mutating concurrency ceiling of 16 (`asc.WithBulkMutatingRequestLimit`) is covered by request-gating tests with a fake transport; its behavior against App Store Connect rate limits has not been measured live yet (#2786).

## Builds

- Read-only checks on 2026-09-30 found 62–74 iPad `deviceModel` rows in `GET /v1/buildBundles/{id}/buildBundleFileSizes` on every one of five builds of a live iPhone-only app, matching universal apps.
- fastlane encountered the same intermittent `GET /v1/apps/{id}/buildUploads` 404 flake (`There is no resource of type 'apps' with id '…'`) after moving build-number lookups to that endpoint (https://github.com/fastlane/fastlane/issues/29908).

## Xcode Cloud workflows

- On 2026-09-06, an authenticated browser session returned HTTP 200 for `GET /ci/api/teams/{teamID}/scm-providers-v2` (six providers) and `GET /ci/api/teams/{teamID}/scm-providers/{providerID}/connection-v2` (one `{"status":"success"}` response) with zero mutations. This establishes the provider contract, not final CLI live execution.
- Custom Xcode Cloud version aliases (`version-aliases-v3`) request and response details are source-captured from the current frontend; no alias mutation was performed against Apple's service during implementation.
- A live read on 2026-09-04 confirmed that private `workflows-v15` payloads expose TestFlight post-actions under `post_actions[].deployment_config.testflight_deployment_ids`. Two distinct `beta_group_ids` observed across two post-actions matched the same app's authenticated IRIS `betaGroups` IDs and the public command output from `asc testflight groups list --app APP_ID --paginate --output json` (three groups returned). The scan covered 14 products/workflows; all observed `beta_tester_ids` arrays were empty, so tester-ID equivalence remains unverified.

## Developer Portal session (web session)

- The public schema supports `fields[bundleIds]=bundleIdCapabilities` and `limit[bundleIdCapabilities]=50`; their acceptance by the private `/services-account/v1` proxy remains live-unverified.

## Developer Portal Bundle ID reads (web session)

- Portal-only attributes observed in the captured `POST /services-account/v1/bundleIds` (`X-HTTP-Method-Override: GET`) collection include `identifier`, `dateModified`, `entitlementGroupName`, `bundleType`, `platform`, `wildcard`, `dateCreated`, `bundleIdCapabilitiesSettingOption`, `seedId`, `name`, `platformName`, `deploymentDataNotice`, and `responseId`.

## Developer Portal Services IDs (web session)

- A 2026-09-26 browser capture accepted `PATCH /services-account/v1/bundleIds/{id}` for `asc web service-ids domains set` with an ID-less `bundleIdCapabilities` child linked to capability `APPLE_ID_AUTH` and its existing `appConsentBundleId`. The captured detail response reported `paging.total=0` with `limit=2147483647` despite populated capability linkage.
- Live CLI verification on 2026-09-27 confirmed a domain update, unchanged repeat, and two-domain/two-return-URL replacement with the existing primary App ID preserved. The disposable Services ID and parent App ID were deleted afterward.
- A 2026-09-28 live capture on the disposable team (`asc web service-ids create --identifier com.rorkai.asc.capture.services.t1790609684 --name "asc capture temp" --confirm`, then `asc web service-ids view --service-id ID --output json`, then `asc web service-ids delete --service-id ID --confirm`; a follow-up view returned 404 and the list had no match) showed that a Services ID with zero capabilities returns `{"meta":{"paging":{"total":0,"limit":2147483647}},"data":[],"links":{"self":...,"related":...}}` with no `next` link. The sanitized relationship is checked in as `internal/web/testdata/service_id_empty_capabilities_relationship.json`, with the captured resource ID in its links replaced by `service-1`.

## Developer Portal Website Push IDs (web session)

- The live `listWebsitePushIds.action` read returned HTTP 200 with `resultCode=0`, `pageNumber=1`, `pageSize=1000`, and an empty root-level `websitePushIdList` in the captured account state.
- The captured frontend (`captures/apple-developer-main.js`) maps each legacy item to `id=item.websitePushId` for this resource (offset 849,700), while its generic identifier table reads `name` and `identifier` (offset 866,718); the topic picker also reads `websitePushId`/`id` and `identifier` (offsets 1,806,200–1,808,480).
- A final built-CLI canary using binary SHA-256 `fd940938b7477aa27b0f5c793618af0c5c1e30d050f85b6bacf1738378ce31f9` exercised the complete happy path for one uniquely generated owned disposable identifier: baseline `list`, `create`, `view`, `delete`, and final `list` all exited successfully. The create and delete receipts and detail readback matched the canary identity, the capability relationship and included graph were empty, the baseline list remained unchanged, and final absence plus temporary-evidence cleanup were verified. The CLI view exposed no certificate fields; `certificateAbsenceIndependentlyVerified` is false because this command slice has no certificate endpoint, so the canary does not claim independent certificate absence.

## Sparse Fieldsets Combined with Includes

- Observed on 2026-09-02 against a live App Store Connect team:
  - `GET /v1/profiles` with `fields[profiles]=name&include=devices` returned HTTP 200 and still put related devices in `included`, while omitting `relationships` on each profile unless `devices` was also in `fields[profiles]`. `fields[devices]=name,udid` sparse-filtered the included devices.
  - `GET /v1/certificates` with `fields[certificates]=displayName&include=passTypeId` returned HTTP 200. The team had no `PASS_TYPE_ID` certificates, so `included` was absent both with that sparse fieldset and with `include=passTypeId` alone. Non-pass certificates exposed `relationships.passTypeId.data=null` only when the relationship was in the fieldset (or when no certificate fieldset was sent).

## Developer Portal App Groups (web session)

- `deleteApplicationGroup.action` was inferred from sibling `listApplicationGroups.action` / `addApplicationGroup.action` actions and third-party portal client contracts; it was not live-verified when added because no Developer Portal web session was available.
- Both single-resource `PATCH /services-account/v1/bundleIds/{id}` capability updates and the pre-delete collection read (`POST /services-account/v1/bundleIds` with `X-HTTP-Method-Override: GET`, `fields[bundleIds]=bundleIdCapabilities`, and `limit[bundleIdCapabilities]=50`) remain unverified against a live private proxy.

## Offline signature verification: Apple leaf certificate types

- Built on 2026-09-28 from public certificates on a macOS 27 host:
  - Login keychain certificates (`security find-certificate -a -p`), `DeveloperCertificates` in local provisioning profiles (22 Apple Development, 11 Apple Distribution, 8 legacy iPhone Developer), leaves extracted with `codesign -d --extract-certificates` from signed apps and IPAs, and leaves read from `X509Certificate` elements of flat-package tables of contents (`xar --dump-toc`), including Xcode's bundled `CoreTypes`, `MobileDevice`, `MobileDeviceDevelopment`, and `XcodeSystemResources` pkgs.
  - Apple's device-management article "Allowing and denying apps and binaries" confirmed `6.1.9` (App Store), `6.1.13` (Developer ID), `6.1.4`, and `6.1.12` (development). Apple's WWDR and Developer ID CPS pages were not machine-readable at the time.

## Offline signature verification: xar member payload checksums

- Checked on 2026-09-29 against `pkgutil --check-signature` on macOS 27:
  - Flipping one payload byte of Xcode's `MobileDeviceDevelopment.pkg` made `pkgutil` report "package is invalid (checksum did not verify)", and `pkg-info` reported `invalid` naming `Payload`.
  - All 12 packages bundled in three installed Xcode releases and a notarized Developer ID Installer pkg verified as `valid`.
  - An older Developer ID Installer pkg whose certificate expired in 2024 recomputed all 13 of its file checksums and still reported `expired`.
  - `pkgutil --check-signature` still accepted the older Developer ID Installer pkg after one of its extended-attribute (`<ea>`) heap bytes was flipped, confirming that `<ea>` checksums should not be enforced.
