---
name: apple-dev-docs
description: "Apple Developer Tools - documentation search, WWDC videos, and App Store Connect management (TestFlight, builds, submissions, signing, analytics, subscriptions, and more). USE WHEN: user asks about Apple APIs, needs documentation lookup, wants to manage App Store Connect (TestFlight, builds, certificates, profiles, analytics, subscriptions, IAP, screenshots), automate iOS/macOS app workflows, or search WWDC sessions. DON'T USE WHEN: user wants general coding help without Apple docs, is asking about non-Apple platforms, or wants to build an iOS app from scratch (use coding-agent). EDGE CASES: 'how do I use NavigationStack' -> this skill. 'upload my build to TestFlight' -> this skill. 'list my certificates' -> this skill. 'build me a SwiftUI app' -> coding-agent."
metadata: {"clawdbot":{"emoji":"🍎","requires":{"bins":["node","asc"]}}}
---

# Apple Developer Tools

Search Apple docs, frameworks, APIs, WWDC videos, and manage App Store Connect from CLI.

## Setup

```bash
# Install asc CLI (App Store Connect)
brew install asc

# Authenticate with App Store Connect API
asc auth login --name "MyApp" --key-id "KEY_ID" --issuer-id "ISSUER_ID" --private-key /path/to/AuthKey.p8
```

Generate API keys at: https://appstoreconnect.apple.com/access/integrations/api

## Part 1: Documentation Search

The docs CLI is at `cli.js`. All commands output formatted text.

### Search Documentation

```bash
node cli.js search "NavigationStack"
node cli.js search "SwiftUI animation" --limit 20
```

### Symbols & APIs

```bash
node cli.js symbols "UIView"
node cli.js apis "UIViewController"
node cli.js platform "UIScrollView"
node cli.js similar "UIPickerView"
```

### Get Documentation Details

```bash
node cli.js doc "/documentation/swiftui/navigationstack"
```

### Technology Browsing

```bash
node cli.js tech
node cli.js overview "SwiftUI"
node cli.js samples "SwiftUI"
node cli.js updates
```

### WWDC Videos

```bash
node cli.js wwdc-search "concurrency"
node cli.js wwdc-year 2025
node cli.js wwdc-topic "swiftui-ui-frameworks"
node cli.js wwdc-topics
node cli.js wwdc-video 2024-10169
node cli.js wwdc-years
```

Available topics: swiftui-ui-frameworks, swift, machine-learning-ai, developer-tools, privacy-security, visionos, accessibility, design, audio-video, networking, testing, graphics-games, health-fitness, maps-location, safari-web, system-services, app-store-distribution, extensions, augmented-reality, widgets-app-intents

## Part 2: App Store Connect

Full reference: [references/app-store-connect.md](references/app-store-connect.md)

All `asc` commands output JSON by default. Use `--output table` for human-readable output.

### Quick Reference

| Task | Command |
|------|---------|
| List apps | `asc apps` |
| List builds | `asc builds list --app "APP_ID"` |
| Upload build | `asc builds upload --app "APP_ID" --ipa "app.ipa" --wait` |
| Latest build | `asc builds latest --app "APP_ID"` |
| Expire old builds | `asc builds expire-all --app "APP_ID" --older-than 90d --confirm` |
| List TestFlight groups | `asc testflight beta-groups list --app "APP_ID"` |
| Add tester | `asc testflight beta-testers add --app "APP_ID" --email "t@test.com" --group "Beta"` |
| Publish to TestFlight | `asc publish testflight --app "APP_ID" --ipa "app.ipa" --group "Beta" --wait` |
| Submit to App Store | `asc publish appstore --app "APP_ID" --ipa "app.ipa" --submit --confirm --wait` |
| List certificates | `asc certificates list` |
| List profiles | `asc profiles list` |
| Create version | `asc versions create --app "APP_ID" --version "1.0.0"` |
| Check reviews | `asc reviews --app "APP_ID" --output table` |
| Sales report | `asc analytics sales --vendor "VENDOR" --type SALES --subtype SUMMARY --frequency DAILY --date "2024-01-20"` |
| Xcode Cloud trigger | `asc xcode-cloud run --app "APP_ID" --workflow "CI" --branch "main" --wait` |
| Notarize macOS app | `asc notarization submit --file ./MyApp.zip --wait` |
| Validate pre-submit | `asc validate --app "APP_ID" --version-id "VERSION_ID" --strict` |

### Categories

**TestFlight**: feedback, crashes, beta groups, beta testers, sync, review, metrics
**Builds**: list, upload, expire, test notes, groups, testers, metrics
**App Store**: reviews, ratings, versions, localizations, screenshots, submissions
**Signing**: certificates, profiles, bundle IDs, capabilities
**Subscriptions**: groups, pricing, availability, offers, localizations
**In-App Purchases**: create, pricing, localizations, availability
**Analytics**: sales reports, analytics requests, downloads
**Finance**: financial reports, regions
**Xcode Cloud**: workflows, build runs, artifacts, test results, SCM
**Notarization**: submit, status, logs
**Game Center**: achievements, leaderboards, leaderboard sets, localizations
**Webhooks**: create, deliveries, redeliver, ping
**App Clips**: experiences, header images, invocations, domain status
**Screenshots**: capture, frame, upload, sizes
**Workflow**: multi-step automation via .asc/workflow.json
**Publish**: end-to-end TestFlight and App Store workflows

### Auth Environment Variables

| Variable | Purpose |
|----------|---------|
| `ASC_KEY_ID` | API key ID |
| `ASC_ISSUER_ID` | Issuer ID |
| `ASC_PRIVATE_KEY_PATH` | Path to .p8 key |
| `ASC_APP_ID` | Default app ID |
| `ASC_VENDOR_NUMBER` | For sales/finance reports |
| `ASC_TIMEOUT` | Request timeout |
| `ASC_DEFAULT_OUTPUT` | Default output format |

### Output Formats

| Format | Flag | Use Case |
|--------|------|----------|
| JSON (minified) | default | Scripting, automation |
| Table | `--output table` | Terminal display |
| Markdown | `--output markdown` | Documentation |

### Global Flags

- `--paginate` - fetch all pages automatically
- `--limit N` - results per page
- `--sort field` - sort (prefix `-` for descending)
- `--pretty` - pretty-print JSON
- `--confirm` - required for destructive operations
- `--profile "name"` - use specific auth profile
- `--debug` - enable debug output
