# App Store Connect CLI Reference

Complete reference for managing App Store Connect via the `asc` CLI tool.

## Installation

```bash
brew install asc
```

## Authentication

```bash
# Register API key
asc auth login \
  --name "MyApp" \
  --key-id "ABC123" \
  --issuer-id "DEF456" \
  --private-key /path/to/AuthKey.p8

# With network validation
asc auth login --network --name "MyApp" --key-id "ABC123" --issuer-id "DEF456" --private-key /path/to/AuthKey.p8

# Skip validation (CI)
asc auth login --skip-validation --name "MyApp" --key-id "ABC123" --issuer-id "DEF456" --private-key /path/to/AuthKey.p8

# Switch profiles
asc auth switch --name "ClientApp"

# Use profile for single command
asc --profile "ClientApp" apps list

# Check auth status
asc auth status
asc auth status --verbose --validate

# Diagnose auth issues
asc auth doctor
asc auth doctor --fix --confirm

# Logout
asc auth logout
asc auth logout --all
asc auth logout --name "MyApp"

# Init config
asc auth init
asc auth init --local
asc auth init --open
```

### Environment Variables

| Variable | Purpose |
|----------|---------|
| `ASC_KEY_ID` | API key ID |
| `ASC_ISSUER_ID` | Issuer ID |
| `ASC_PRIVATE_KEY_PATH` | Path to .p8 key |
| `ASC_PRIVATE_KEY` | Raw key content |
| `ASC_PRIVATE_KEY_B64` | Base64 key content |
| `ASC_CONFIG_PATH` | Path to config.json |
| `ASC_PROFILE` | Default profile name |
| `ASC_APP_ID` | Default app ID |
| `ASC_VENDOR_NUMBER` | For sales/finance reports |
| `ASC_TIMEOUT` | Request timeout (e.g. 90s, 2m) |
| `ASC_DEFAULT_OUTPUT` | Default output format |
| `ASC_DEBUG` | Debug logging (1 or api) |

## Global Flags

| Flag | Purpose |
|------|---------|
| `--output table` | Human-readable table |
| `--output markdown` | Markdown format |
| `--paginate` | Fetch all pages |
| `--limit N` | Results per page |
| `--sort field` | Sort (prefix `-` for desc) |
| `--pretty` | Pretty-print JSON |
| `--profile "name"` | Use specific auth profile |
| `--debug` | Debug output |

---

## Apps

```bash
asc apps
asc apps --sort name
asc apps --paginate
```

## Builds

```bash
# List builds
asc builds list --app "APP_ID"
asc builds list --app "APP_ID" --sort -uploadedDate --paginate

# Build details
asc builds info --build "BUILD_ID"

# Latest build
asc builds latest --app "APP_ID"
asc builds latest --app "APP_ID" --version "1.0.0" --platform IOS

# Upload build
asc builds upload --app "APP_ID" --ipa "app.ipa"
asc builds upload --app "APP_ID" --pkg "app.pkg" --version "1.0.0" --build-number "123"
asc builds upload --app "APP_ID" --ipa "app.ipa" --concurrency 4 --checksum --wait
asc builds upload --app "APP_ID" --ipa "app.ipa" --test-notes "Test login" --locale "en-US" --wait
asc builds upload --app "APP_ID" --ipa "app.ipa" --dry-run

# Expire builds
asc builds expire --build "BUILD_ID" --confirm
asc builds expire-all --app "APP_ID" --older-than 90d --dry-run
asc builds expire-all --app "APP_ID" --older-than 90d --confirm

# Build groups
asc builds add-groups --build "BUILD_ID" --group "GROUP_ID"
asc builds remove-groups --build "BUILD_ID" --group "GROUP_ID" --confirm

# Build testers
asc builds individual-testers list --build "BUILD_ID"
asc builds individual-testers add --build "BUILD_ID" --tester "TESTER_ID"
asc builds individual-testers remove --build "BUILD_ID" --tester "TESTER_ID"

# Test notes
asc builds test-notes list --build "BUILD_ID"
asc builds test-notes create --build "BUILD_ID" --locale "en-US" --whats-new "Test the new login"
asc builds test-notes update --id "LOC_ID" --whats-new "Updated notes"
asc builds test-notes delete --id "LOC_ID" --confirm

# Build metrics
asc builds metrics beta-usages --build "BUILD_ID"

# Upload management
asc builds uploads list --app "APP_ID"
asc builds uploads get --id "UPLOAD_ID"
asc builds uploads delete --id "UPLOAD_ID" --confirm

# Build relationships
asc builds app get --build "BUILD_ID"
asc builds pre-release-version get --build "BUILD_ID"
asc builds icons list --build "BUILD_ID"
```

## TestFlight

```bash
# Feedback
asc feedback --app "APP_ID"
asc feedback --app "APP_ID" --device-model "iPhone15,3" --os-version "17.2"
asc feedback --app "APP_ID" --app-platform IOS --paginate

# Crashes
asc crashes --app "APP_ID" --output table
asc crashes --app "APP_ID" --sort -createdDate --limit 5 --paginate

# TestFlight apps
asc testflight apps list
asc testflight apps get --app "APP_ID"

# Sync config to YAML
asc testflight sync pull --app "APP_ID" --output "./testflight.yaml"
asc testflight sync pull --app "APP_ID" --output "./testflight.yaml" --include-builds --include-testers

# Review and submission
asc testflight review get --app "APP_ID"
asc testflight review submit --build "BUILD_ID" --confirm

# Beta details
asc testflight beta-details get --build "BUILD_ID"
asc testflight beta-details update --id "DETAIL_ID" --auto-notify

# Beta license agreements
asc testflight beta-license-agreements list --app "APP_ID"
asc testflight beta-license-agreements update --id "ID" --agreement-text "New terms"

# Beta notifications
asc testflight beta-notifications create --build "BUILD_ID"

# Metrics
asc testflight metrics public-link --group "GROUP_ID"
asc testflight metrics beta-tester-usages --app "APP_ID"
```

## Beta Groups

```bash
asc testflight beta-groups list --app "APP_ID"
asc testflight beta-groups list --app "APP_ID" --internal
asc testflight beta-groups list --app "APP_ID" --external --paginate
asc testflight beta-groups create --app "APP_ID" --name "Beta Testers"
asc testflight beta-groups create --app "APP_ID" --name "Internal" --internal
asc testflight beta-groups get --id "GROUP_ID"
asc testflight beta-groups update --id "GROUP_ID" --name "New Name"
asc testflight beta-groups update --id "GROUP_ID" --public-link-enabled --feedback-enabled
asc testflight beta-groups delete --id "GROUP_ID" --confirm
asc testflight beta-groups add-testers --group "GROUP_ID" --tester "TESTER_ID"
asc testflight beta-groups remove-testers --group "GROUP_ID" --tester "TESTER_ID"
asc testflight beta-groups app get --group-id "GROUP_ID"
```

## Beta Testers

```bash
asc testflight beta-testers list --app "APP_ID"
asc testflight beta-testers list --app "APP_ID" --build "BUILD_ID"
asc testflight beta-testers list --app "APP_ID" --group "Beta" --paginate
asc testflight beta-testers get --id "TESTER_ID"
asc testflight beta-testers add --app "APP_ID" --email "t@test.com" --group "Beta"
asc testflight beta-testers remove --app "APP_ID" --email "t@test.com"
asc testflight beta-testers invite --app "APP_ID" --email "t@test.com"
asc testflight beta-testers add-groups --id "TESTER_ID" --group "GROUP_ID"
asc testflight beta-testers remove-groups --id "TESTER_ID" --group "GROUP_ID"
asc testflight beta-testers add-builds --id "TESTER_ID" --build "BUILD_ID"
asc testflight beta-testers remove-builds --id "TESTER_ID" --build "BUILD_ID" --confirm
asc testflight beta-testers metrics --tester-id "TESTER_ID" --app "APP_ID"
```

## Devices

```bash
asc devices list
asc devices list --platform IOS --status ENABLED --udid "UDID1,UDID2"
asc devices list --name "My iPhone" --paginate
asc devices get --id "DEVICE_ID"
asc devices register --name "My iPhone" --udid "UDID" --platform IOS
asc devices register --name "My Mac" --udid-from-system --platform MAC_OS
asc devices update --id "DEVICE_ID" --name "New Name"
asc devices update --id "DEVICE_ID" --status DISABLED
asc devices local-udid
```

## Reviews

```bash
asc reviews --app "APP_ID"
asc reviews --app "APP_ID" --stars 1 --output table
asc reviews --app "APP_ID" --territory US --sort -createdDate --paginate
asc reviews get --id "REVIEW_ID"
asc reviews ratings --app "APP_ID"
asc reviews summarizations --app "APP_ID" --platform IOS --territory USA
asc reviews respond --review-id "REVIEW_ID" --response "Thanks!"
asc reviews response get --id "RESPONSE_ID"
asc reviews response for-review --review-id "REVIEW_ID"
asc reviews response delete --id "RESPONSE_ID" --confirm
```

## App Tags

```bash
asc app-tags list --app "APP_ID"
asc app-tags list --app "APP_ID" --visible-in-app-store true --sort -name
asc app-tags get --app "APP_ID" --id "TAG_ID"
asc app-tags update --id "TAG_ID" --visible-in-app-store=false --confirm
asc app-tags territories --id "TAG_ID" --fields currency
asc app-tags list --app "APP_ID" --paginate
```

## App Events

```bash
asc app-events list --app "APP_ID"
asc app-events localizations list --event-id "EVENT_ID"
asc app-events localizations screenshots list --localization-id "LOC_ID"
asc app-events localizations video-clips list --localization-id "LOC_ID"
```

## Alternative Distribution

```bash
asc alternative-distribution domains list
asc alternative-distribution domains create --domain "example.com" --reference-name "Example"
asc alternative-distribution domains delete --domain-id "ID" --confirm
asc alternative-distribution keys list
asc alternative-distribution keys create --app "APP_ID" --public-key-path "./key.pem"
asc alternative-distribution keys app --app "APP_ID"
asc alternative-distribution packages create --app-store-version-id "VERSION_ID"
asc alternative-distribution packages get --package-id "PKG_ID"
asc alternative-distribution packages versions list --package-id "PKG_ID"
```

## Analytics & Sales

```bash
# Sales reports
asc analytics sales --vendor "VENDOR" --type SALES --subtype SUMMARY --frequency DAILY --date "2024-01-20"
asc analytics sales --vendor "VENDOR" --type SALES --subtype SUMMARY --frequency DAILY --date "2024-01-20" --decompress

# Analytics requests
asc analytics request --app "APP_ID" --access-type ONGOING
asc analytics requests --app "APP_ID" --paginate

# Get reports
asc analytics get --request-id "REQ_ID"
asc analytics get --request-id "REQ_ID" --date "2024-01-20"
asc analytics get --request-id "REQ_ID" --include-segments

# Download
asc analytics download --request-id "REQ_ID" --instance-id "INSTANCE_ID"
```

## Finance Reports

```bash
asc finance reports --vendor "VENDOR" --report-type FINANCIAL --region "ZZ" --date "2025-12"
asc finance reports --vendor "VENDOR" --report-type FINANCE_DETAIL --region "Z1" --date "2025-12" --decompress
asc finance regions --output table
```

## Sandbox Testers

```bash
asc sandbox list
asc sandbox list --email "tester@test.com" --territory "USA" --paginate
asc sandbox get --id "ID"
asc sandbox get --email "tester@test.com"
asc sandbox update --id "ID" --territory "USA"
asc sandbox update --email "tester@test.com" --interrupt-purchases
asc sandbox clear-history --id "ID" --confirm
```

## Xcode Cloud

```bash
# Workflows
asc xcode-cloud workflows --app "APP_ID" --paginate
asc xcode-cloud build-runs --workflow-id "WORKFLOW_ID" --paginate

# Trigger builds
asc xcode-cloud run --app "APP_ID" --workflow "CI Build" --branch "main"
asc xcode-cloud run --workflow-id "WORKFLOW_ID" --git-reference-id "REF_ID"
asc xcode-cloud run --app "APP_ID" --workflow "Deploy" --branch "release/1.0" --wait
asc xcode-cloud run --app "APP_ID" --workflow "CI" --branch "main" --wait --poll-interval 30s --timeout 1h

# Status
asc xcode-cloud status --run-id "BUILD_RUN_ID" --output table
asc xcode-cloud status --run-id "BUILD_RUN_ID" --wait

# Products
asc xcode-cloud products --app "APP_ID"

# SCM
asc xcode-cloud scm providers list
asc xcode-cloud scm repositories list
asc xcode-cloud scm repositories git-references --repo-id "REPO_ID"

# Artifacts
asc xcode-cloud actions --run-id "BUILD_RUN_ID"
asc xcode-cloud artifacts list --action-id "ACTION_ID"
asc xcode-cloud artifacts download --id "ARTIFACT_ID" --path "./artifact.zip"

# Test results
asc xcode-cloud test-results list --action-id "ACTION_ID"
asc xcode-cloud issues list --action-id "ACTION_ID"

# Versions
asc xcode-cloud macos-versions
asc xcode-cloud xcode-versions
```

## Notarization

```bash
asc notarization submit --file ./MyApp.zip
asc notarization submit --file ./MyApp.zip --wait
asc notarization submit --file ./MyApp.zip --wait --poll-interval 30s --timeout 1h
asc notarization status --id "SUBMISSION_ID"
asc notarization log --id "SUBMISSION_ID"
asc notarization list --output table
```

## Game Center

```bash
# Achievements
asc game-center achievements list --app "APP_ID"
asc game-center achievements create --app "APP_ID" --reference-name "First Win" --vendor-id "com.example.firstwin" --points 10
asc game-center achievements update --id "ID" --points 20
asc game-center achievements delete --id "ID" --confirm
asc game-center achievements localizations list --achievement-id "ID"
asc game-center achievements localizations create --achievement-id "ID" --locale en-US --name "First Win" --before-earned-description "Win" --after-earned-description "Won!"
asc game-center achievements images upload --localization-id "LOC_ID" --file "image.png"
asc game-center achievements releases create --app "APP_ID" --achievement-id "ID"

# Leaderboards
asc game-center leaderboards list --app "APP_ID"
asc game-center leaderboards create --app "APP_ID" --reference-name "High Score" --vendor-id "com.example.highscore" --formatter INTEGER --sort DESC --submission-type BEST_SCORE
asc game-center leaderboards localizations create --leaderboard-id "ID" --locale en-US --name "High Score"
asc game-center leaderboards images upload --localization-id "LOC_ID" --file "image.png"

# Leaderboard Sets
asc game-center leaderboard-sets list --app "APP_ID"
asc game-center leaderboard-sets create --app "APP_ID" --reference-name "Season 1" --vendor-id "com.example.season1"
asc game-center leaderboard-sets members set --set-id "SET_ID" --leaderboard-ids "id1,id2,id3"
```

## Signing

```bash
asc signing fetch --bundle-id "com.example.app" --profile-type IOS_APP_STORE --output "./signing"
asc signing fetch --bundle-id "com.example.app" --profile-type IOS_APP_DEVELOPMENT --device "DEVICE_ID" --create-missing
```

## Certificates

```bash
asc certificates list
asc certificates list --certificate-type "IOS_DISTRIBUTION,IOS_DEVELOPMENT" --paginate
asc certificates get --id "CERT_ID"
asc certificates create --certificate-type "IOS_DISTRIBUTION" --csr "./CertificateSigningRequest.certSigningRequest"
asc certificates revoke --id "CERT_ID" --confirm
```

## Profiles

```bash
asc profiles list
asc profiles list --profile-type "IOS_APP_STORE,IOS_APP_DEVELOPMENT" --paginate
asc profiles get --id "PROFILE_ID" --include "bundleId,certificates,devices"
asc profiles create --name "My Profile" --profile-type IOS_APP_STORE --bundle "BUNDLE_ID" --certificate "CERT_ID"
asc profiles create --name "Dev Profile" --profile-type IOS_APP_DEVELOPMENT --bundle "BUNDLE_ID" --certificate "CERT_ID" --device "DEVICE_ID1,DEVICE_ID2"
asc profiles download --id "PROFILE_ID" --output "./profile.mobileprovision"
asc profiles delete --id "PROFILE_ID" --confirm
```

## Bundle IDs

```bash
asc bundle-ids list --paginate
asc bundle-ids get --id "BUNDLE_ID"
asc bundle-ids create --identifier "com.example.app" --name "My App" --platform IOS
asc bundle-ids update --id "BUNDLE_ID" --name "New Name"
asc bundle-ids delete --id "BUNDLE_ID" --confirm
asc bundle-ids capabilities list --bundle "BUNDLE_ID"
asc bundle-ids capabilities add --bundle "BUNDLE_ID" --capability IN_APP_PURCHASE
```

## Subscriptions

```bash
# Groups
asc subscriptions groups list --app "APP_ID"
asc subscriptions groups create --app "APP_ID" --reference-name "Premium"
asc subscriptions groups update --id "GROUP_ID" --reference-name "Premium+"
asc subscriptions groups delete --id "GROUP_ID" --confirm
asc subscriptions groups submit --group-id "GROUP_ID" --confirm

# Group localizations
asc subscriptions groups localizations list --group-id "GROUP_ID"
asc subscriptions groups localizations create --group-id "GROUP_ID" --locale en-US --name "Premium"

# Subscriptions
asc subscriptions list --group "GROUP_ID"
asc subscriptions create --group "GROUP_ID" --ref-name "Monthly" --product-id "com.example.monthly" --subscription-period "ONE_MONTH"
asc subscriptions get --id "SUB_ID"
asc subscriptions update --id "SUB_ID" --ref-name "Monthly Premium"
asc subscriptions delete --id "SUB_ID" --confirm
asc subscriptions submit --subscription-id "SUB_ID" --confirm

# Pricing
asc subscriptions pricing --app "APP_ID"
asc subscriptions pricing --subscription-id "SUB_ID" --territory "USA"
asc subscriptions prices list --id "SUB_ID"

# Availability
asc subscriptions availability get --subscription-id "SUB_ID"
asc subscriptions availability set --id "SUB_ID" --territory "USA,GBR,JPN"

# Offers
asc subscriptions introductory-offers list --subscription-id "SUB_ID"
asc subscriptions introductory-offers create --subscription-id "SUB_ID" --offer-duration "ONE_MONTH" --offer-mode "FREE_TRIAL" --number-of-periods 1
asc subscriptions promotional-offers list --subscription-id "SUB_ID"
asc subscriptions promotional-offers create --subscription-id "SUB_ID" --offer-code "PROMO1" --name "Holiday" --offer-duration "ONE_MONTH" --offer-mode "PAY_AS_YOU_GO" --number-of-periods 3

# Price points
asc subscriptions price-points list --subscription-id "SUB_ID"
```

## In-App Purchases

```bash
asc iap list --app "APP_ID" --paginate
asc iap list --app "APP_ID" --legacy
asc iap create --app "APP_ID" --type CONSUMABLE --ref-name "100 Coins" --product-id "com.example.coins100"
asc iap update --id "IAP_ID" --ref-name "200 Coins"
asc iap delete --id "IAP_ID" --confirm
asc iap prices --app "APP_ID"
asc iap submit --iap-id "IAP_ID" --confirm
asc iap localizations list --iap-id "IAP_ID"
asc iap localizations create --iap-id "IAP_ID" --locale en-US --name "100 Coins" --description "Buy 100 coins"
asc iap availability get --iap-id "IAP_ID"
asc iap availability set --iap-id "IAP_ID" --territories "USA,GBR,JPN"
asc iap price-points list --iap-id "IAP_ID"
asc iap price-schedules get --iap-id "IAP_ID"
```

## Offer Codes

```bash
asc offer-codes list --offer-code "OFFER_CODE_ID" --paginate
asc offer-codes get --offer-code-id "ID"
asc offer-codes create --subscription-id "SUB_ID" --name "Holiday" --customer-eligibilities "NEW,EXISTING" --offer-eligibility "ONCE" --duration "ONE_MONTH" --offer-mode "PAY_AS_YOU_GO" --number-of-periods 3 --prices "USA:PRICE_POINT_ID"
asc offer-codes update --offer-code-id "ID" --active false
asc offer-codes generate --offer-code "ID" --quantity 10 --expiration-date "2026-02-01"
asc offer-codes values --id "ID" --output "./offer-codes.txt"
asc offer-codes custom-codes list --offer-code-id "ID"
asc offer-codes custom-codes create --offer-code-id "ID" --custom-code "HOLIDAY2026"
```

## Performance

```bash
asc performance metrics list --app "APP_ID"
asc performance metrics list --app "APP_ID" --metric-type "LAUNCH,HANG" --platform IOS
asc performance metrics get --build "BUILD_ID"
asc performance diagnostics list --build "BUILD_ID" --diagnostic-type "DISK_WRITES,HANGS"
asc performance diagnostics get --id "SIGNATURE_ID"
asc performance download --app "APP_ID" --output "./metrics.json"
```

## Webhooks

```bash
asc webhooks list --app "APP_ID"
asc webhooks get --webhook-id "ID"
asc webhooks create --app "APP_ID" --name "Build Notifications" --url "https://example.com/webhook" --secret "secret" --events "BUILD_CREATED,BUILD_UPDATED" --enabled true
asc webhooks update --webhook-id "ID" --enabled false
asc webhooks delete --webhook-id "ID" --confirm
asc webhooks deliveries --webhook-id "ID" --created-after "2025-01-01"
asc webhooks deliveries redeliver --delivery-id "ID"
asc webhooks ping --webhook-id "ID"
```

## Publish (End-to-End)

```bash
# TestFlight publish
asc publish testflight --app "APP_ID" --ipa "app.ipa" --group "Beta Testers"
asc publish testflight --app "APP_ID" --ipa "app.ipa" --group "Internal,External" --notify --wait
asc publish testflight --app "APP_ID" --ipa "app.ipa" --group "Beta" --test-notes "Test login" --locale "en-US" --wait

# App Store publish
asc publish appstore --app "APP_ID" --ipa "app.ipa" --submit --confirm --wait
```

## Versions

```bash
asc versions list --app "APP_ID" --paginate
asc versions get --version-id "VERSION_ID"
asc versions create --app "APP_ID" --version "1.0.0"
asc versions create --app "APP_ID" --version "2.0.0" --platform IOS --release-type MANUAL
asc versions update --version-id "VERSION_ID" --version "1.0.1"
asc versions delete --version-id "VERSION_ID" --confirm
asc versions attach-build --version-id "VERSION_ID" --build "BUILD_ID"
asc versions release --version-id "VERSION_ID" --confirm
asc versions phased-release get --version-id "VERSION_ID"
asc versions phased-release create --version-id "VERSION_ID"
asc versions phased-release update --id "ID" --state PAUSED
```

## App Info

```bash
asc app-info get --app "APP_ID"
asc app-info get --app "APP_ID" --version "1.2.3" --platform IOS
asc app-info get --app "APP_ID" --include "ageRatingDeclaration,territoryAgeRatings"
asc app-info set --app "APP_ID" --locale "en-US" --whats-new "Bug fixes"
asc app-info set --app "APP_ID" --locale "en-US" --description "My app" --keywords "app,tool" --support-url "https://example.com"
```

## App Setup

```bash
asc app-setup info set --app "APP_ID" --primary-locale "en-US" --bundle-id "com.example.app"
asc app-setup info set --app "APP_ID" --locale "en-US" --name "My App" --subtitle "Great app"
asc app-setup categories set --app "APP_ID" --primary GAMES --secondary ENTERTAINMENT
asc app-setup availability set --app "APP_ID" --territory "USA,GBR" --available true
asc app-setup pricing set --app "APP_ID" --price-point "PRICE_POINT_ID" --base-territory "USA"
asc app-setup localizations upload --version "VERSION_ID" --path "./localizations"
```

## Localizations

```bash
asc localizations list --version "VERSION_ID" --paginate
asc localizations list --app "APP_ID" --type app-info
asc localizations download --version "VERSION_ID" --path "./localizations"
asc localizations upload --version "VERSION_ID" --path "./localizations"
```

## Build Localizations

```bash
asc build-localizations list --build "BUILD_ID" --paginate
asc build-localizations create --build "BUILD_ID" --locale "en-US" --whats-new "Bug fixes"
asc build-localizations update --id "LOC_ID" --whats-new "New features"
asc build-localizations delete --id "LOC_ID" --confirm
```

## Pre-Release Versions

```bash
asc pre-release-versions list --app "APP_ID" --paginate
asc pre-release-versions list --app "APP_ID" --platform IOS --version "1.0.0"
asc pre-release-versions get --id "PRERELEASE_ID"
```

## Screenshots & Video Previews

```bash
# Screenshots
asc screenshots list --version-localization "LOC_ID"
asc screenshots sizes
asc screenshots sizes --display-type "APP_IPHONE_65"
asc screenshots upload --version-localization "LOC_ID" --path "./screenshots/" --device-type IPHONE_65
asc screenshots delete --id "SCREENSHOT_ID" --confirm

# Local capture (experimental)
asc screenshots capture --bundle-id "com.example.app" --name home
asc screenshots frame --input "./screenshots/raw/home.png" --device iphone-air

# Video previews
asc video-previews list --version-localization "LOC_ID"
asc video-previews upload --version-localization "LOC_ID" --path "./previews/" --device-type IPHONE_65
asc video-previews delete --id "PREVIEW_ID" --confirm
```

## App Clips

```bash
asc app-clips list --app "APP_ID"
asc app-clips get --id "APP_CLIP_ID"
asc app-clips default-experiences list --app-clip-id "ID"
asc app-clips default-experiences create --app-clip-id "ID" --action OPEN
asc app-clips advanced-experiences list --app-clip-id "ID"
asc app-clips advanced-experiences create --app-clip-id "ID" --link "https://example.com/clip" --default-language en
asc app-clips header-images create --localization-id "LOC_ID" --file "header.png"
asc app-clips invocations list --build-bundle-id "BUNDLE_ID"
asc app-clips domain-status cache --build-bundle-id "BUNDLE_ID"
```

## Encryption

```bash
asc encryption declarations list --app "APP_ID"
asc encryption declarations get --id "DECLARATION_ID"
asc encryption declarations create --app "APP_ID" --app-description "Uses HTTPS only" --contains-proprietary-cryptography false --contains-third-party-cryptography false
asc encryption declarations assign-builds --id "DECLARATION_ID" --build "BUILD_ID1,BUILD_ID2"
asc encryption documents upload --declaration "DECLARATION_ID" --file "./encryption-doc.pdf"
```

## Background Assets

```bash
asc background-assets list --app "APP_ID"
asc background-assets create --app "APP_ID" --asset-pack-identifier "com.example.assets.pack1"
asc background-assets update --id "ASSET_ID" --archived true
asc background-assets versions list --background-asset-id "ASSET_ID"
asc background-assets upload-files create --version-id "VERSION_ID" --file "./asset.bin" --asset-type ASSET
```

## Routing Coverage

```bash
asc routing-coverage get --version-id "VERSION_ID"
asc routing-coverage create --version-id "VERSION_ID" --file "./routing.geojson"
asc routing-coverage delete --id "COVERAGE_ID" --confirm
```

## Workflow

```bash
asc workflow validate
asc workflow list
asc workflow run --dry-run beta
asc workflow run beta BUILD_ID:123456789 GROUP_ID:abcdef
```

## Categories

```bash
asc categories list --output table
asc categories set --app "APP_ID" --primary GAMES --secondary ENTERTAINMENT
```

## Validate (Pre-Submission)

```bash
asc validate --app "APP_ID" --version-id "VERSION_ID"
asc validate --app "APP_ID" --version-id "VERSION_ID" --platform IOS --output table
asc validate --app "APP_ID" --version-id "VERSION_ID" --strict
```

## Submit

```bash
asc submit create --app "APP_ID" --version "1.0.0" --build "BUILD_ID" --confirm
asc submit status --id "SUBMISSION_ID"
asc submit status --version-id "VERSION_ID"
asc submit cancel --id "SUBMISSION_ID" --confirm
```

## Migrate (Fastlane)

```bash
asc migrate validate --fastlane-dir ./fastlane
asc migrate import --app "APP_ID" --version-id "VERSION_ID" --fastlane-dir ./fastlane
asc migrate import --dry-run
asc migrate export --app "APP_ID" --version-id "VERSION_ID" --output-dir ./exported-metadata
```

## Notify

```bash
asc notify slack --webhook "WEBHOOK_URL" --message "Build deployed!"
asc notify slack --webhook "WEBHOOK_URL" --message "v1.0.0 live" --channel "#releases"
```
