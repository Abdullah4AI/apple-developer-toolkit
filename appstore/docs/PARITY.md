# Parity map

Lookup table for remaining fastlane/ASC parity after the 2026-08-30 backlog
(#2227–#2236) and the 2026-09-02 web-session issues. Command claims were
checked against a built `asc` binary's `--help`. Live inventory:
`asc capabilities` (use `--status web-session` for web-only rows). Do not
duplicate that catalog here.

Each area is exactly one of: covered by a command, tracked by issue(s), or an
intentional non-goal. Do not copy this file into `AGENTS.md`.

## Remaining areas

| Area | Resolution |
| --- | --- |
| App creation plus Developer Portal service associations as one end-to-end flow | **Intentional non-goal.** No single end-to-end command. Compose `asc web apps create` with `asc web bundle-ids`. Remaining identifier/services/iCloud work is #2290. |
| Metadata, screenshots, preview videos, binary, and submission as one operation | **Intentional non-goal.** Separate commands by design. Closest composed paths: `asc publish appstore` (IPA or local build, optional metadata, optional submit) and `asc release stage` (metadata, attach, validate). Screenshots and preview videos stay `asc screenshots` and `asc video-previews`. Chain with `asc workflow`. |
| Simulator creation, booting, app installation, and screenshot capture beyond the bounded matrix in #2230 | **Intentional non-goal for mutation.** `asc xcode test-destinations` lists simulators and the local Mac with a read-only `simctl list`. `asc screenshots run` and default iOS `asc screenshots capture` require a booted simulator and an installed app. `asc screenshots capture --provider macos` captures a running macOS app. `asc xcode install` targets a connected physical device, not a simulator. Bounded matrix capture is #2230. Create/boot/clone/delete stay on host Simulator/`simctl`. |
| Test configuration and reporting beyond the first `asc xcode test` slice (#2227) | **Intentional non-goal for extra test knobs.** First slice shipped as `asc xcode test` (`test`, `build-for-testing`, `test-without-building`, destinations, test plans, filters, result bundle, JUnit via `--report junit --report-file PATH`). An existing `.xcresult` converts with `asc xcode test junit`. Further scan-style knobs stay `--xcodebuild-flag` unless demand appears. |
| Code signing on a fresh CI runner: certificate and private-key creation, profiles, shared encrypted identity, keychain lifecycle, and Xcode signing settings | **Covered.** `asc signing fetch --create-missing --create-missing-certificate` creates the key, CSR, certificate, profile, and `.p12`; `--match-extensions` adds extension bundle IDs; `--delete-stale-profiles` removes expired or invalid profiles (`asc profiles list --stale-only` lists them). `asc signing sync push` and `pull` share the encrypted identity (Git, S3-compatible object storage, or experimental GitLab Secure Files and AWS Secrets Manager). `asc signing keychain install`, `list`, `unlock`, `lock`, `set-timeout`, `set-partition-list`, and `delete` own the dedicated keychain. `asc xcode signing plan` (with `--profile`) and `asc xcode signing apply` set per-target signing settings and `ExportOptions.plist`; `asc xcode archive` and `asc xcode export` produce the signed artifact. Sequence: [CI_CD.md](CI_CD.md#code-signing-on-a-fresh-runner). |
| Signing profile renewal, bulk cleanup, device refresh, and Mac profile inclusion | **Covered.** With Git storage, `asc signing sync push --renew-expired` replaces an expired synced profile with a same-name successor; `--force-for-new-devices` recreates a development or ad hoc profile when its devices change, and `--include-mac-in-profiles` adds enabled Apple silicon Macs to iOS development and ad hoc device sets; `asc signing sync nuke` deletes every profile of one type, revokes its certificates, and removes their encrypted files. Stale cleanup outside sync is `asc signing fetch --delete-stale-profiles`; certificates are deactivated or revoked with `asc certificates`. |
| Signing object storage | **Covered.** `asc signing sync push`, `pull`, and `rotate-password` with `--storage object --object-bucket NAME` (optional `--object-prefix`, `--object-region`, `--object-endpoint` for S3-compatible services) store the same ciphertext in the Git tree's layout, with conditional writes that never overwrite a concurrent push. Credentials come from the standard AWS chain, including workload identity; no flag accepts one. Profile renewal, device refresh, and `sync nuke` remain Git-only. CI example: [CI_CD.md](CI_CD.md#signing-assets-in-object-storage-with-workload-identity). |
| Web-session-only capabilities with no public Apple API | **Tracked by** open 2026-09-02 web-session issues: #2275, #2278, #2283, #2284, #2286, #2287, #2288, #2290, #2291, #2292, #2293, #2294, #2295, #2297, #2298, #2299, #2300, #2301. Shipped the same day (do not reopen): #2268, #2269, #2271, #2272, #2274, #2277, #2280, #2281, #2289, #2296. Shipped inventory: `asc capabilities --status web-session`. |
| Android tooling, plugin/lane runtime, chat connectors, documentation generators, obsolete actions | **Intentional non-goal.** See list below. |

## Intentional non-goals

Unless user demand appears, do not build:

- Android tooling (Play/Gradle/supply). `asc android-ios-mapping` is Apple's Android-to-iOS mapping API, not Android CI.
- Plugin/lane runtime (Fastfile plugins). `asc workflow` runs repo-local steps; it is not a plugin host.
- Chat notification connectors beyond existing `asc notify slack` (incoming webhook): Mailgun, Chatwork, IFTTT, Flock, desktop notifications.
- Documentation generators (API/doc-site generators, fastlane docs plugins). `asc docs` / `asc init` are CLI reference helpers.
- Obsolete actions Apple or fastlane have already retired.
