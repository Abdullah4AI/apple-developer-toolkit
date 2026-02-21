---
name: apple-developer-toolkit
description: "Complete Apple developer toolkit: documentation search, WWDC videos (2014-2025), App Store Connect management (TestFlight, builds, signing, analytics, subscriptions), and autonomous iOS app builder with 50 SwiftUI/iOS best practice guides. USE WHEN: user asks about Apple APIs, needs docs lookup, manages App Store Connect, builds iOS apps from description, or searches WWDC sessions. DON'T USE WHEN: non-Apple platforms or general coding without Apple context."
metadata: {"clawdbot":{"emoji":"🍎"}}
---

# Apple Developer Toolkit

Three tools in one skill: documentation search, App Store Connect management, and autonomous iOS app building.

## Setup

```bash
brew tap Abdullah4AI/tap
brew install appstore swiftship
```

For App Store Connect, authenticate:
```bash
appstore auth login --name "MyApp" --key-id "KEY_ID" --issuer-id "ISSUER_ID" --private-key /path/to/AuthKey.p8
```

## Part 1: Documentation Search

```bash
node cli.js search "NavigationStack"
node cli.js symbols "UIView"
node cli.js doc "/documentation/swiftui/navigationstack"
node cli.js overview "SwiftUI"
node cli.js samples "SwiftUI"
node cli.js wwdc-search "concurrency"
node cli.js wwdc-year 2025
node cli.js wwdc-topic "swiftui-ui-frameworks"
```

## Part 2: App Store Connect

Full reference: [references/app-store-connect.md](references/app-store-connect.md)

| Task | Command |
|------|---------|
| List apps | `appstore apps` |
| Upload build | `appstore builds upload --app "APP_ID" --ipa "app.ipa" --wait` |
| Publish TestFlight | `appstore publish testflight --app "APP_ID" --ipa "app.ipa" --group "Beta" --wait` |
| Submit App Store | `appstore publish appstore --app "APP_ID" --ipa "app.ipa" --submit --confirm --wait` |
| List certificates | `appstore certificates list` |
| Reviews | `appstore reviews --app "APP_ID" --output table` |
| Sales report | `appstore analytics sales --vendor "VENDOR" --type SALES --subtype SUMMARY --frequency DAILY --date "2024-01-20"` |
| Xcode Cloud | `appstore xcode-cloud run --app "APP_ID" --workflow "CI" --branch "main" --wait` |
| Notarize | `appstore notarization submit --file ./MyApp.zip --wait` |
| Validate | `appstore validate --app "APP_ID" --version-id "VERSION_ID" --strict` |

Covers: TestFlight, Builds, Signing, Subscriptions, IAP, Analytics, Finance, Xcode Cloud, Notarization, Game Center, Webhooks, App Clips, Screenshots, Workflow automation, Migrate (Fastlane).

## Part 3: iOS App Builder

Build complete iOS apps from natural language descriptions.

```bash
swiftship              # Interactive mode
swiftship setup        # Install prerequisites (Xcode, XcodeGen, Claude Code)
swiftship fix          # Auto-fix build errors
swiftship run          # Build and launch in simulator
swiftship info         # Show project status
swiftship usage        # Token usage and cost
```

### How it works

```
describe → analyze → plan → build → fix → run
```

1. **Analyze** - Extracts app name, features, core flow from description
2. **Plan** - Produces file-level build plan: data models, navigation, design
3. **Build** - Generates Swift source files, project.yml, asset catalog
4. **Fix** - Compiles and auto-repairs until build succeeds
5. **Run** - Boots iOS Simulator and launches the app

### Interactive commands

| Command | Description |
|---------|-------------|
| `/run` | Build and launch in simulator |
| `/fix` | Auto-fix compilation errors |
| `/open` | Open project in Xcode |
| `/model [name]` | Switch model (sonnet, opus, haiku) |
| `/info` | Show project info |
| `/usage` | Token usage and cost |

## References

| Reference | Content |
|-----------|---------|
| [references/app-store-connect.md](references/app-store-connect.md) | Complete App Store Connect CLI commands |
| [references/ios-rules/](references/ios-rules/) | 38 iOS development rules (accessibility, dark mode, localization, etc.) |
| [references/swiftui-guides/](references/swiftui-guides/) | 12 SwiftUI best practice guides (animations, liquid glass, state, etc.) |
| [references/ios-app-builder-prompts.md](references/ios-app-builder-prompts.md) | System prompts for app analysis, planning, and building |

### iOS Rules (38 files)

accessibility, app_clips, app_review, apple_translation, biometrics, camera, charts, color_contrast, components, dark_mode, design-system, feedback_states, file-structure, forbidden-patterns, foundation_models, gestures, haptics, healthkit, live_activities, localization, maps, mvvm-architecture, navigation-patterns, notification_service, notifications, safari_extension, share_extension, siri_intents, spacing_layout, speech, storage-patterns, swift-conventions, timers, typography, view-composition, view_complexity, website_links, widgets

### SwiftUI Guides (12 files)

animations, forms-and-input, layout, liquid-glass, list-patterns, media, modern-apis, navigation, performance, scroll-patterns, state-management, text-formatting
