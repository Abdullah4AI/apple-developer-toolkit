# Apple Developer Toolkit

All-in-one Apple developer skill: documentation search, WWDC videos, App Store Connect management, and autonomous iOS app builder. Built for AI agents and developers.

## Features

### Documentation & WWDC
- 1,267 WWDC sessions indexed locally (2014-2025)
- Direct integration with developer.apple.com
- Score-based search across all indexed sessions

### App Store Connect
- **TestFlight** - builds, beta groups, testers, feedback, crash reports
- **Builds** - upload IPAs/PKGs, expire old builds, test notes, metrics
- **App Store** - versions, localizations, screenshots, review submissions, phased releases
- **Signing** - certificates, provisioning profiles, bundle IDs, capabilities
- **Subscriptions & IAP** - create and manage subscriptions, in-app purchases, offer codes, pricing
- **Analytics & Sales** - download sales reports, analytics data, finance reports
- **Xcode Cloud** - trigger workflows, monitor build runs, download artifacts
- **Notarization** - submit, poll, and retrieve logs for macOS notarization
- **Game Center** - achievements, leaderboards, leaderboard sets
- **Webhooks, App Clips, Screenshots, Workflow automation**
- **Validate & Migrate** - pre-submission checks, Fastlane compatibility

### iOS App Builder
- Build complete iOS apps from natural language descriptions
- Multi-phase pipeline: analyze, plan, build, fix, run
- 38 iOS development rules (accessibility, dark mode, localization, gestures, etc.)
- 12 SwiftUI best practice guides (animations, liquid glass, state management, etc.)
- Auto-fix compilation errors
- Launch directly in iOS Simulator
- Interactive editing with slash commands

## Requirements

- **Node.js** v18+ (for documentation search)
- **Xcode** (for iOS app building)
- `brew tap Abdullah4AI/tap && brew install appstore swiftship`

## Installation

For AI agents:

```bash
npx skills add Abdullah4AI/apple-dev-docs
```

For ClawHub:

```bash
clawhub install apple-developer-toolkit
```

## Quick Start

### Documentation

```bash
node cli.js search "SwiftUI animation"
node cli.js wwdc-search "swift concurrency"
node cli.js wwdc-video 2024-10169
node cli.js overview "SwiftUI"
```

### App Store Connect

```bash
# Install
brew tap Abdullah4AI/tap
brew install appstore swiftship

# Authenticate
appstore auth login --name "MyApp" --key-id "KEY_ID" --issuer-id "ISSUER_ID" --private-key /path/to/AuthKey.p8
appstore apps
appstore publish testflight --app "APP_ID" --ipa "app.ipa" --group "Beta Testers" --wait
appstore reviews --app "APP_ID" --output table
```

### iOS App Builder

```bash
swiftship
# > A workout tracker that logs exercises and shows weekly progress with charts
# ✓ Build complete — ready to launch!
```

## Documentation Commands

| Command | Description |
|---------|-------------|
| `search "query"` | Search Apple Developer Documentation |
| `symbols "UIView"` | Search framework classes, structs, protocols |
| `doc "/path/to/doc"` | Get detailed documentation by path |
| `apis "UIViewController"` | Find related APIs |
| `overview "SwiftUI"` | Get technology overview guide |
| `samples "SwiftUI"` | Find sample code projects |
| `wwdc-search "async"` | Search WWDC sessions |
| `wwdc-year 2025` | List all videos for a year |

## App Store Connect Commands

| Task | Command |
|------|---------|
| List apps | `appstore apps` |
| Upload build | `appstore builds upload --app "APP_ID" --ipa "app.ipa" --wait` |
| Publish TestFlight | `appstore publish testflight --app "APP_ID" --ipa "app.ipa" --group "Beta" --wait` |
| Submit App Store | `appstore publish appstore --app "APP_ID" --ipa "app.ipa" --submit --confirm --wait` |
| Certificates | `appstore certificates list` |
| Reviews | `appstore reviews --app "APP_ID" --output table` |
| Sales report | `appstore analytics sales --vendor "VENDOR" --type SALES --subtype SUMMARY --frequency DAILY --date "2024-01-20"` |
| Xcode Cloud | `appstore xcode-cloud run --app "APP_ID" --workflow "CI" --branch "main" --wait` |
| Notarize | `appstore notarization submit --file ./MyApp.zip --wait` |
| Validate | `appstore validate --app "APP_ID" --version-id "VERSION_ID" --strict` |

Full reference: [references/app-store-connect.md](references/app-store-connect.md)

## iOS App Builder Commands

| Command | Description |
|---------|-------------|
| `swiftship` | Interactive mode - describe your app |
| `swiftship setup` | Install prerequisites |
| `swiftship fix` | Auto-fix build errors |
| `swiftship run` | Build and launch in simulator |
| `swiftship info` | Show project status |
| `swiftship usage` | Token usage and cost |

## References

- **38 iOS development rules**: accessibility, dark mode, localization, gestures, widgets, haptics, and more
- **12 SwiftUI guides**: animations, liquid glass, state management, navigation, layout, performance, and more
- **App builder prompts**: system prompts for app analysis, planning, and code generation

## How It Works

Documentation searches query developer.apple.com directly. WWDC data is indexed locally. App Store Connect operations use the built-in `appstore` CLI. iOS app building uses the `swiftship` CLI with Claude Code as the AI backend.

## License

MIT
