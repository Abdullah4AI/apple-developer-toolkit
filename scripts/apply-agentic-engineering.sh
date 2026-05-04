#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

python3 - <<'PY'
from pathlib import Path
import re

ROOT = Path.cwd()

README_TEXT = """Skills work out of the box with any agent that supports the [Agent Skills](https://agentskills.io) format, including Claude Code, Codex, Cursor, Windsurf, Gemini CLI, and other compatible coding agents."""

SKILL_INSTALL_SECTION = """## Agent Skills Installation

Use the current Agent Skills CLI syntax when installing this repository into coding agents:

```bash
# Install both skills
npx skills add Abdullah4AI/apple-developer-toolkit

# Install a specific skill
npx skills add Abdullah4AI/apple-developer-toolkit --skill ios-rules
npx skills add Abdullah4AI/apple-developer-toolkit --skill swiftui-guides
```

Supported agents include Claude Code, Codex, Cursor, Windsurf, Gemini CLI, and any agent that supports the Agent Skills format. For Codex or Cursor, install the same skills with `npx skills add ...` from the project where the agent should use the Apple rules, then let the agent load the generated skill instructions from that workspace.
"""

SAFETY_SECTION = """## Agent Safety, Permissions, and Observability

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
"""

def update_readme(path: Path) -> None:
    if not path.exists():
        return
    s = path.read_text()
    s = s.replace("npx add-skill Abdullah4AI/apple-developer-toolkit", "npx skills add Abdullah4AI/apple-developer-toolkit")
    s = re.sub(
        r"Skills work out of the box with any agent that supports the \[Agent Skills\]\(https://agentskills\.io\) format(?:, including Claude Code, Codex, Cursor, Windsurf, Gemini CLI, and other compatible coding agents)?\.",
        README_TEXT,
        s,
    )
    s = re.sub(r"\n## License\n\nMIT\n(?=\n<div align=\"center\">)", "\n", s)
    path.write_text(s)

def update_skill(path: Path) -> None:
    if not path.exists():
        return
    s = path.read_text()
    s = s.replace("npx add-skill Abdullah4AI/apple-developer-toolkit", "npx skills add Abdullah4AI/apple-developer-toolkit")

    if "## Agent Skills Installation" not in s:
        marker = "## Agent Safety, Permissions, and Observability"
        setup_marker = "## Credential Requirements by Feature"
        if marker in s:
            s = s.replace(marker, SKILL_INSTALL_SECTION + "\n" + marker, 1)
        elif setup_marker in s:
            s = s.replace(setup_marker, SKILL_INSTALL_SECTION + "\n" + setup_marker, 1)

    if "## Agent Safety, Permissions, and Observability" not in s:
        marker = "## Credential Requirements by Feature"
        if marker in s:
            s = s.replace(marker, SAFETY_SECTION + "\n" + marker, 1)

    path.write_text(s)

for readme in [ROOT / "README.md", ROOT / "skills/apple-developer-toolkit/README.md"]:
    update_readme(readme)

update_skill(ROOT / "SKILL.md")

# Unsupported by Agent Skills document ingestion. Keep the skill package text-only.
for image in (ROOT / "appstore/docs/images").glob("*.png") if (ROOT / "appstore/docs/images").exists() else []:
    image.unlink()
PY

# Validate required text and absence of unsupported image assets.
python3 - <<'PY'
from pathlib import Path
import re

root = Path.cwd()
skill = (root / "SKILL.md").read_text()
readme = (root / "README.md").read_text()
nested = (root / "skills/apple-developer-toolkit/README.md").read_text()

required = [
    "Agent Skills Installation",
    "Agent Safety, Permissions, and Observability",
    "Memory Discipline",
    "Tool Permission Design",
    "Observability Requirements",
    "Workflow Orchestration",
    "Codex",
    "Cursor",
    "npx skills add Abdullah4AI/apple-developer-toolkit",
]
for phrase in required:
    if phrase not in skill:
        raise SystemExit(f"SKILL.md missing: {phrase}")

for label, text in [("README.md", readme), ("skills/apple-developer-toolkit/README.md", nested)]:
    if "npx add-skill" in text:
        raise SystemExit(f"{label} still uses old add-skill syntax")
    if "npx skills add Abdullah4AI/apple-developer-toolkit" not in text:
        raise SystemExit(f"{label} missing new skills CLI syntax")
    if re.search(r"\n## License\n\nMIT\n", text):
        raise SystemExit(f"{label} has duplicate trailing License section")

pngs = list(root.rglob("*.png"))
if pngs:
    raise SystemExit("Unsupported PNG assets remain: " + ", ".join(str(p.relative_to(root)) for p in pngs))
PY

git diff --check -- README.md SKILL.md skills/apple-developer-toolkit/README.md AGENTS.md .cursor/rules/agentic-engineering.mdc docs/agentic-engineering.md scripts/apply-agentic-engineering.sh

echo "Agentic engineering compatibility applied and verified."
