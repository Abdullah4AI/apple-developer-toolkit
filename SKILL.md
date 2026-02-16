---
name: apple-docs
description: "Search and browse Apple Developer Documentation, frameworks, APIs, WWDC videos, and sample code. USE WHEN: user asks about Apple APIs (SwiftUI, UIKit, Foundation, etc.), needs to look up documentation for iOS/macOS/watchOS/tvOS/visionOS development, wants WWDC session transcripts or code examples, needs to check API platform compatibility, or wants to explore Apple frameworks and sample projects. DON'T USE WHEN: user wants general coding help without needing Apple docs lookup, is asking about non-Apple platforms, or already has the information they need. EDGE CASES: 'how do I use NavigationStack' → this skill (API lookup). 'write me a SwiftUI view' → just write it, use this skill only if you need to verify an API. 'what's new in iOS 26' → this skill (documentation updates). 'find WWDC talks about concurrency' → this skill (WWDC search)."
---

# Apple Developer Documentation

Search Apple docs, frameworks, APIs, WWDC videos, and sample code via CLI.

## Setup

First run only:
```bash
bash scripts/setup.sh
```

## CLI Reference

The CLI is at `scripts/apple-docs.mjs`. All commands output formatted text.

### Core Commands

**Search docs:**
```bash
scripts/apple-docs.mjs search "SwiftUI List"
scripts/apple-docs.mjs search "Core Data migration" --type documentation
```

**Get doc page content:**
```bash
scripts/apple-docs.mjs doc https://developer.apple.com/documentation/swiftui/navigationstack
scripts/apple-docs.mjs doc https://developer.apple.com/documentation/uikit/uiviewcontroller --related --platform
```

**Browse frameworks and symbols:**
```bash
scripts/apple-docs.mjs technologies --category "App frameworks"
scripts/apple-docs.mjs symbols SwiftUI --type struct --pattern "*View"
```

**Check platform compatibility:**
```bash
scripts/apple-docs.mjs compatibility https://developer.apple.com/documentation/swiftui/list
```

**Find alternatives/similar APIs:**
```bash
scripts/apple-docs.mjs similar https://developer.apple.com/documentation/uikit/uialertview
```

### WWDC Commands

```bash
scripts/apple-docs.mjs wwdc search "async await" --year 2024
scripts/apple-docs.mjs wwdc video 2024 10101
scripts/apple-docs.mjs wwdc code --framework SwiftUI --year 2025
scripts/apple-docs.mjs wwdc topics
scripts/apple-docs.mjs wwdc years
```

### Typical Workflow

1. Search: `search "<API name>"` to find the right page
2. Read: `doc <url> --platform` to get full details with platform info
3. Explore: `related <url>` or `similar <url>` to discover connected APIs
4. Learn: `wwdc search "<topic>"` to find WWDC sessions explaining the concept

For detailed flags and all command options, read `references/commands.md`.
