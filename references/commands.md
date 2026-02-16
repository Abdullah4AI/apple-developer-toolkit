# Apple Docs CLI - Command Reference

Read this file when the user needs detailed help on specific commands or flag options.

## All Commands

### search
Search Apple Developer Documentation.
```
apple-docs search <query> [--type all|documentation|sample]
```
- `query`: API names, framework names, or technical terms. Be specific (e.g. "NSPredicate" not "how to filter")
- `--type`: Filter results. "documentation" for API refs, "sample" for code snippets

### doc
Get detailed content from a specific documentation page.
```
apple-docs doc <url> [--related] [--refs] [--similar] [--platform]
```
- `url`: Must start with `https://developer.apple.com/documentation/`
- `--related`: Include inheritance hierarchy and protocol conformances
- `--refs`: Resolve all referenced types and APIs
- `--similar`: Find APIs with similar functionality
- `--platform`: Analyze platform availability and version requirements

### technologies
Browse all Apple technologies and frameworks.
```
apple-docs technologies [--category <cat>] [--language swift|occ] [--beta false] [--limit N]
```
Categories (case-sensitive): "App frameworks", "Graphics and games", "App services", "Media", "System"

### symbols
Search symbols within a specific framework.
```
apple-docs symbols <framework> [--type <type>] [--pattern <pat>] [--language swift|occ] [--limit N]
```
- `framework`: e.g. "SwiftUI", "UIKit", "Foundation"
- `--type`: all, class, struct, protocol, enum, method, property, function, typealias, macro, case
- `--pattern`: Wildcard pattern (e.g. "UI*Controller", "*Animation*")

### related
Analyze API relationships.
```
apple-docs related <url> [--no-inherited] [--no-conformance] [--no-see-also]
```

### references
Deep dive into all referenced types on a documentation page.
```
apple-docs references <url> [--max N] [--filter <type>]
```
- `--max`: 1-50, default 20
- `--filter`: all, symbol, collection, article, protocol, class, struct, enum

### compatibility
Check API platform availability.
```
apple-docs compatibility <url> [--mode single|framework] [--related]
```
- `--mode framework`: Shows all APIs in the entire framework

### similar
Find alternative and replacement APIs.
```
apple-docs similar <url> [--depth shallow|medium|deep] [--category <cat>] [--no-alternatives]
```

### updates
Track latest Apple documentation changes.
```
apple-docs updates [--category all|wwdc|technology|release-notes] [--tech <tech>] [--year <year>] [--query <q>] [--beta false] [--limit N]
```

### overviews
Access comprehensive guides and tutorials.
```
apple-docs overviews [--category <cat>] [--platform all|ios|macos|watchos|tvos|visionos] [--query <q>] [--subcategories false] [--limit N]
```
Categories: "app-design-and-ui", "games", "ai-machine-learning", "augmented-reality", "privacy-and-security"

### samples
Browse complete sample code projects.
```
apple-docs samples [--framework <fw>] [--beta include|exclude|only] [--query <q>] [--limit N]
```

## WWDC Commands

### wwdc list
Browse WWDC session videos.
```
apple-docs wwdc list [--year <year>] [--topic <topic>] [--has-code true|false] [--limit N]
```
Available years: 2014-2025

### wwdc search
Full-text search across WWDC transcripts and code.
```
apple-docs wwdc search <query> [--in transcript|code|both] [--year <year>] [--language <lang>] [--limit N]
```

### wwdc video
Get complete WWDC session content with transcript and code.
```
apple-docs wwdc video <year> <videoId> [--no-transcript] [--no-code]
```

### wwdc code
Browse code examples from WWDC sessions.
```
apple-docs wwdc code [--framework <fw>] [--topic <topic>] [--year <year>] [--language swift|objc|javascript|metal] [--limit N]
```

### wwdc topics
Browse WWDC topic categories.
```
apple-docs wwdc topics [--topic <id>] [--videos false] [--year <year>] [--limit N]
```
Topic IDs: accessibility-inclusion, app-services, app-store-distribution-marketing, audio-video, business-education, design, developer-tools, essentials, graphics-games, health-fitness, machine-learning-ai, maps-location, photos-camera, privacy-security, safari-web, spatial-computing, swift, swiftui-ui-frameworks, system-services

### wwdc related
Find related WWDC videos.
```
apple-docs wwdc related <year> <videoId> [--no-explicit] [--no-topic] [--year-related] [--limit N]
```

### wwdc years
List all available WWDC years with video counts.
```
apple-docs wwdc years
```

## Common Patterns

**Find an API and read its docs:**
```bash
apple-docs search "NavigationStack"
apple-docs doc https://developer.apple.com/documentation/swiftui/navigationstack --platform
```

**Explore a framework:**
```bash
apple-docs symbols SwiftUI --type struct --pattern "*View"
apple-docs symbols UIKit --type protocol
```

**Find WWDC content on a topic:**
```bash
apple-docs wwdc search "@Observable macro" --in both --year 2024
apple-docs wwdc video 2024 10101
```

**Check what's new:**
```bash
apple-docs updates --category wwdc --year 2025
apple-docs samples --beta only
```
