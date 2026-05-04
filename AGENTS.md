# Agentic Engineering Rules

This repository is updated from upstream on a recurring cadence. After every upstream sync, preserve or reapply the project-level agent rules below.

## Core Principles

- Do not let temporary run state poison future agent behavior.
- Make tool permissions explicit before side effects.
- Keep observability artifacts for non-trivial runs.
- Route workflows through the right surface instead of guessing.

## Memory Discipline

- Do not persist task logs, raw Apple docs output, generated code snippets, App Store state, build errors, credentials, or temporary decisions as long-term memory.
- Save only stable reusable facts: durable user preferences, environment conventions, tool quirks, or confirmed setup details.
- Prefer repo artifacts, run summaries, status files, and skill references for workflow state.

## Tool Permission Design

Read-only by default:

- `node cli.js search`, `symbols`, `doc`, `overview`, `samples`, and `wwdc-*`
- `appledev store ... list/status/validate/diff/docs/doctor`
- Reading reference files, README files, config defaults, and docs

Requires explicit user approval before side effects:

- App Store Connect creates, updates, deletes, uploads, TestFlight publish, App Store submit, pricing, IAP, subscriptions, certificates, profiles, devices, webhooks, notarization, and Xcode Cloud runs
- `appledev build` actions that write a local project, fix files, launch simulators, install hooks, or invoke credentials
- Any GitHub write action: commits, pushes, PR creation, merges, releases, tags, or workflow changes

Never use this repository or skill to post to X/Twitter or invoke any X publishing command.

## Observability Requirements

For non-trivial runs, leave enough audit trail to understand what happened:

- Logs: preserve relevant command output or failure excerpts.
- Traces: state which surface was used: `node cli.js`, `appledev store`, or `appledev build`.
- Artifacts: write plans, diffs, reports, metadata exports, or build summaries to project-local files when appropriate.
- Status files: for long release or app-builder workflows, keep a compact status summary with current step, target app/project, next safe action, and blockers.
- Run summaries: finish with what was checked, what changed, what did not change, and remaining risks.

## Workflow Orchestration

- Apple docs and WWDC lookup: use `node cli.js` from this repo.
- App Store Connect workflows: use `appledev store`.
- App scaffolding and fix loops: use `appledev build`.
- Platform and design rules: load relevant `references/`, `skills/ios-rules/`, or `skills/swiftui-guides/` files progressively.
- Multi-step release/build work should follow: inspect → plan → dry-run or validate → execute with approval → verify → summarize.
- Ask before mutating when ambiguity affects app ID, bundle ID, release version, TestFlight group, pricing, certificate, or project path.

## Upstream Sync Hygiene

After upstream sync commits, run:

```bash
bash scripts/apply-agentic-engineering.sh
```

Then verify:

```bash
git diff --check -- README.md SKILL.md skills/apple-developer-toolkit/README.md
npx skills add . --list --full-depth
```

The script reapplies the current Agent Skills install syntax, removes unsupported PNG assets from the skill package, preserves the agent-safety sections in `SKILL.md`, and removes duplicate License sections from README files.
