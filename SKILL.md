---
name: apple-developer-toolkit
description: "Apple platform skill for docs, WWDC lookup, App Store Connect work, and SwiftUI app generation. Use repo-local `node cli.js` for Apple docs and WWDC search, `appledev store` for App Store Connect workflows, and `appledev build` for app scaffolding or fix loops on macOS. USE WHEN: Apple APIs, WWDC sessions, TestFlight/App Store tasks, or building/fixing Apple-platform apps. DON'T USE WHEN: non-Apple platforms, generic backend work, or general web research. EDGE CASES: docs-only queries use `node cli.js` in this repo, not `appledev`; release workflows use `appledev store`; app scaffolding uses `appledev build`; rules-only requests can read `references/ios-rules/` or `references/swiftui-guides/` progressively without invoking binaries."
metadata:
  {
    "openclaw":
      {
        "emoji": "🍎",
        "requires":
          {
            "bins": ["node"],
            "anyBins": ["appledev"],
          },
        "install":
          [
            {
              "id": "appledev",
              "kind": "brew",
              "tap": "Abdullah4AI/tap",
              "formula": "appledev",
              "bins": ["appledev"],
              "label": "Apple Developer Toolkit - unified binary (Homebrew)",
            },
          ],
        "env":
          {
            "optional":
              [
                {
                  "name": "APPSTORE_KEY_ID",
                  "description": "App Store Connect API Key ID. Required only for App Store Connect features. Get from https://appstoreconnect.apple.com/access/integrations/api",
                },
                {
                  "name": "APPSTORE_ISSUER_ID",
                  "description": "App Store Connect API Issuer ID. Required only for App Store Connect features.",
                },
                {
                  "name": "APPSTORE_PRIVATE_KEY_PATH",
                  "description": "Path to App Store Connect API .p8 private key file. Required only for App Store Connect features. Alternative: use APPSTORE_PRIVATE_KEY or APPSTORE_PRIVATE_KEY_B64.",
                },
                {
                  "name": "LLM_API_KEY",
                  "description": "LLM API key for code generation. Required only for iOS App Builder. Supports multiple AI backends.",
                },
              ],
          },
      },
  }
---

# Apple Developer Toolkit

This skill has two execution surfaces plus a local Apple docs corpus. Each part works independently with different requirements.

## Architecture

Use the right entry point for the job:

```
node cli.js search ...    # Apple docs + WWDC lookup from this repo
appledev build ...        # SwiftShip app builder
appledev store ...        # App Store Connect CLI
```

Important: `appledev --help` currently exposes `build` and `store`, but not the docs search commands. Docs lookup is repo-local through `node cli.js`.

## Agent Setup

Before asking the user for defaults, read `config.json` in this skill directory.

- Keep non-secret setup defaults in `config.json`
- Keep durable agent-side notes or run logs in `${CLAUDE_PLUGIN_DATA}/apple-developer-toolkit/`
- Keep secrets out of repo files. Use env vars or Keychain-backed flows instead

## Agent Safety, Permissions, and Observability

Use this skill with strict agent-operating rules so Apple workflows do not poison future context or create unintended side effects.

### Memory Discipline

- Do not save task logs, raw Apple docs output, generated code snippets, App Store state, build errors, credentials, or temporary decisions to persistent memory
- Save only stable reusable facts: user preferences, environment conventions, tool quirks, or confirmed setup details that will matter in future sessions
- Prefer skill references, local artifacts, run summaries, and status files over memory for workflow state

### Tool Permission Design

- Read-only by default: `node cli.js search`, `symbols`, `doc`, `overview`, `samples`, `wwdc-*`, `appledev store ... list/status/validate/diff/docs/doctor`, and reading reference files
- Requires explicit user approval before side effects: App Store Connect creates/updates/deletes, TestFlight publish, App Store submit, metadata/localization updates, pricing/IAP/subscription changes, certificate/profile/device changes, webhooks, notarization, uploads, Xcode Cloud runs, `appledev build` project writes, simulator launches, or hook installation
- Never use publishing or external-posting tools outside their stated surface. In particular, this skill must not post to X/Twitter or invoke any X publishing command
- If a command can mutate App Store Connect or a local project, state the target app/project, expected change, and whether credentials will be used before running it
- Prefer dry-run, diff, status, validate, or docs commands before mutating commands when available

### Observability Requirements

For non-trivial runs, create or preserve enough artifacts to audit what happened:

- Logs: keep command output or tool output relevant to the decision
- Traces: note the command path used, for example `node cli.js` vs `appledev store` vs `appledev build`
- Artifacts: write generated plans, diffs, reports, metadata exports, or build summaries to project-local files when appropriate
- Status files: for long app-builder or release workflows, keep a small status/summary file with current step, target app/project, next safe action, and blockers
- Run summaries: finish with what was checked, what changed, what did not change, and remaining risks

### Workflow Orchestration

- Route tasks explicitly: docs lookup → `node cli.js`; App Store Connect → `appledev store`; app scaffolding/fix loops → `appledev build`; platform/design rules → relevant `references/` files
- Break multi-step release or build work into inspect → plan → dry-run/validate → execute with approval → verify → summarize
- Use specialized references progressively. Load only the relevant `references/apple-*.md`, `references/ios-rules/`, or `references/swiftui-guides/` material needed for the task
- Stop and ask when ambiguity changes side effects, such as which app ID, bundle ID, release version, TestFlight group, pricing, certificate, or project path to mutate

## Credential Requirements by Feature

| Feature | Credentials Needed | Works Without Setup |
|---------|-------------------|-------------------|
| Documentation Search (Part 1) | None | Yes |
| App Store Connect (Part 2) | App Store Connect API key (.p8) | No |
| iOS App Builder (Part 3) | LLM API key + Xcode | No |

## Setup

Check `config.json` first for local defaults and paths.

### Part 1: Documentation Search (no setup needed)

Works immediately with Node.js:

```bash
node cli.js search "NavigationStack"
```

### Part 2: App Store Connect CLI

Install via Homebrew:

```bash
brew install Abdullah4AI/tap/appledev
```

Authenticate with your App Store Connect API key:

```bash
appledev store auth login --name "MyApp" --key-id "KEY_ID" --issuer-id "ISSUER_ID" --private-key /path/to/AuthKey.p8
```

Or set environment variables:

```bash
export APPSTORE_KEY_ID="your-key-id"
export APPSTORE_ISSUER_ID="your-issuer-id"
export APPSTORE_PRIVATE_KEY_PATH="/path/to/AuthKey.p8"
```

API keys are created at https://appstoreconnect.apple.com/access/integrations/api

### Part 3: iOS App Builder

Prerequisites: Xcode (with iOS Simulator), XcodeGen, and an LLM API key for code generation.

```bash
appledev build setup    # Checks and installs prerequisites
```

### Build from source

```bash
bash scripts/setup.sh
```

The setup script builds `appledev`, copies it to `/opt/homebrew/bin/appledev`, and symlinks `swiftship` plus `appstore` to the same binary.

## Gotchas

- Apple docs and WWDC lookup are not exposed through `appledev --help` right now. Use `node cli.js ...` from this repo for those tasks
- Help text inside `cli.js` mentions `apple-docs ...`, but a separate `apple-docs` binary may not be installed. In this skill, `node cli.js ...` is the safe path
- `appledev store ...` needs App Store Connect credentials. If they are missing, authenticate with `appledev store auth login` or env vars before trying release actions
- `appledev build ...` is macOS-only in practice and depends on Xcode, Simulator support, and an LLM API key. Do not assume it will work in a generic sandbox
- Hook config lives outside the repo at `~/.appledev/hooks.yaml` and optionally inside projects at `.appledev/hooks.yaml`. Use `templates/hooks-*.yaml` as starting points instead of writing YAML from scratch
- Do not write secrets into `config.json`, templates, or repo files

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
| List apps | `appledev store apps` |
| Upload build | `appledev store builds upload --app "APP_ID" --ipa "app.ipa" --wait` |
| Find build by number | `appledev store builds find --app "APP_ID" --build-number "42"` |
| Wait for build processing | `appledev store builds wait --build "BUILD_ID"` |
| Publish TestFlight | `appledev store publish testflight --app "APP_ID" --ipa "app.ipa" --group "Beta" --wait` |
| Submit App Store | `appledev store publish appstore --app "APP_ID" --ipa "app.ipa" --submit --confirm --wait` |
| Pre-submission validation | `appledev store validate --app "APP_ID" --version-id "VERSION_ID"` |
| List certificates | `appledev store certificates list` |
| Reviews | `appledev store reviews --app "APP_ID" --output table` |
| Update localizations | `appledev store localizations update --app "APP_ID" --locale "en-US" --name "My App"` |
| Sales report | `appledev store analytics sales --vendor "VENDOR" --type SALES --subtype SUMMARY --frequency DAILY --date "2024-01-20"` |
| Xcode Cloud | `appledev store xcode-cloud run --app "APP_ID" --workflow "CI" --branch "main" --wait` |
| Notarize | `appledev store notarization submit --file ./MyApp.zip --wait` |
| Status dashboard | `appledev store status --app "APP_ID" --output table` |
| Weekly insights | `appledev store insights weekly --app "APP_ID" --source analytics` |
| Metadata pull | `appledev store metadata pull --app "APP_ID" --version "1.2.3" --dir ./metadata` |
| Release notes | `appledev store release-notes generate --since-tag "v1.2.2"` |
| Diff localizations | `appledev store diff localizations --app "APP_ID" --path ./metadata` |
| Nominations | `appledev store nominations create --app "APP_ID" --name "Launch"` |
| Price point filter | `appledev store pricing price-points --app "APP_ID" --price 0.99` |
| IAP (family sharable) | `appledev store iap create --app "APP_ID" --family-sharable` |
| Subscription (family sharable) | `appledev store subscriptions create --app "APP_ID" --family-sharable` |

### Environment Variables

All environment variables are optional. They override flags when set.

| Variable | Description |
|----------|-------------|
| `APPSTORE_KEY_ID` | API Key ID |
| `APPSTORE_ISSUER_ID` | API Issuer ID |
| `APPSTORE_PRIVATE_KEY_PATH` | Path to .p8 key file |
| `APPSTORE_PRIVATE_KEY` | Raw private key string |
| `APPSTORE_PRIVATE_KEY_B64` | Base64-encoded private key |
| `APPSTORE_APP_ID` | Default app ID |
| `APPSTORE_PROFILE` | Default auth profile |
| `APPSTORE_DEBUG` | Enable debug output |
| `APPSTORE_TIMEOUT` | Request timeout |
| `APPSTORE_BYPASS_KEYCHAIN` | Skip system keychain |

## Part 3: Multi-Platform App Builder

Supports iOS, watchOS, tvOS, and iPad. Generates complete Swift/SwiftUI apps from natural language with AI-powered code generation.

```bash
appledev build                     # Interactive mode
appledev build setup               # Install prerequisites (Xcode, XcodeGen, AI backend)
appledev build fix                 # Auto-fix build errors
appledev build run                 # Build and launch in simulator
appledev build open                # Open project in Xcode
appledev build chat                # Interactive chat mode (edit/ask questions)
appledev build info                # Show project status
appledev build usage               # Token usage and cost
```

### Supported Platforms

| Platform | Status |
|----------|--------|
| iOS | Full support |
| iPad | Full support |
| macOS | Supported |
| watchOS | Supported |
| tvOS | Supported |
| visionOS | Supported |

### How it works

```
describe > analyze > plan > build > fix > run
```

1. **Analyze** - Extracts app name, features, core flow, target platform from description
2. **Plan** - Produces file-level build plan: data models, navigation, design
3. **Build** - Generates Swift source files, project.yml, asset catalog
4. **Fix** - Compiles and auto-repairs until build succeeds
5. **Run** - Boots Simulator and launches the app

### Interactive commands

| Command | Description |
|---------|-------------|
| `/run` | Build and launch in simulator |
| `/fix` | Auto-fix compilation errors |
| `/open` | Open project in Xcode |
| `/ask [question]` | Ask a question about the project |
| `/model [name]` | Switch model (sonnet, opus, haiku) |
| `/info` | Show project info |
| `/usage` | Token usage and cost |

## References

| Reference | Content |
|-----------|---------|
| [references/app-store-connect.md](references/app-store-connect.md) | Complete App Store Connect CLI commands |
| [references/ios-rules/](references/ios-rules/) | 38 iOS development rules |
| [references/swiftui-guides/](references/swiftui-guides/) | 12 SwiftUI best practice guides |
| [references/ios-app-builder-prompts.md](references/ios-app-builder-prompts.md) | System prompts for app building |

### iOS Rules (38 files)

accessibility, app_clips, app_review, apple_translation, biometrics, camera, charts, color_contrast, components, dark_mode, design-system, feedback_states, file-structure, forbidden-patterns, foundation_models, gestures, haptics, healthkit, live_activities, localization, maps, mvvm-architecture, navigation-patterns, notification_service, notifications, safari_extension, share_extension, siri_intents, spacing_layout, speech, storage-patterns, swift-conventions, timers, typography, view-composition, view_complexity, website_links, widgets

### SwiftUI Guides (12 files)

animations, forms-and-input, layout, liquid-glass, list-patterns, media, modern-apis, navigation, performance, scroll-patterns, state-management, text-formatting

## Consolidated Apple/Swiftship references

This umbrella absorbed the former one-feature/one-phase Apple skills. Load the targeted reference instead of relying on narrow active sibling skills:

- `references/apple-features-skills.md` — App Store Connect, framework features, monetization, assets, and app services.
- `references/apple-ui-skills.md` — accessibility, layout, typography, animations, glass, gestures, and view composition.
- `references/apple-extensions-skills.md` — widgets, App Clips, Live Activities, Safari/share/notification extensions.
- `references/apple-phases-skills.md` — Swiftship analyzer/planner/builder/editor/fixer/recovery routing.
- `references/apple-always-skills.md` and platform variants — shared SwiftUI, design-system, layout, components, navigation, review guidance.
