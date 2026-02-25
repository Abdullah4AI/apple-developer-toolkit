# Workflows

`asc workflow` lets you define named, multi-step automation sequences in a repo-local file: `.asc/workflow.json`.

This is designed as a single, versioned workflow file that composes existing `asc` commands and normal shell commands.

## Quick Start

1. Create `.asc/workflow.json` in your repo.
2. Validate the file:

```bash
asc workflow validate
```

3. Run a workflow:

```bash
asc workflow run beta
asc workflow run beta BUILD_ID:123456789 GROUP_ID:abcdef
```

## Security and Trust Model

Workflows intentionally execute arbitrary shell commands. This is by design: `asc workflow run` is effectively "run a repo-local script".

`asc` does not sandbox workflow execution. A workflow runs with the same permissions as the `asc` process: it can read files, make network requests, and access anything in the environment.

- Treat workflow files like code. Review changes to `.asc/workflow.json` the same way you'd review any code or CI config.
- Do not run workflow files from untrusted sources (e.g., copied from the internet, or from a PR/fork you haven't reviewed).
- In CI, avoid running `asc workflow run` for untrusted pull requests/forks if secrets or write-capable tokens are available in the environment. A safer pattern is to run `asc workflow validate` on PRs and run workflows only on trusted branches.
- Be careful with `--file`: it can point to any path, not just `.asc/workflow.json`.
- Step commands inherit your process environment (`os.Environ()`), so secrets present in the environment are visible to steps.
- Avoid printing secrets in commands; prefer passing secrets as env vars via your CI secret store.
- Treat params as untrusted input. Quote expansions in shell commands to avoid injection issues (e.g., `--app "$APP_ID"` not `--app $APP_ID`).
- `asc workflow validate` checks structure and references, not safety of the commands.

## Example `.asc/workflow.json`

Notes:
- The file supports JSONC comments (`//` and `/* */`).
- Output is JSON on stdout; step and hook command output streams to stderr.
- On failures, stdout still remains JSON-only and includes a top-level `error` plus `hooks` results.

```json
{
  "env": {
    "APP_ID": "123456789",
    "VERSION": "1.0.0"
  },
  "before_all": "asc auth status",
  "after_all": "echo workflow_done",
  "error": "echo workflow_failed",
  "workflows": {
    "beta": {
      "description": "Distribute a build to a TestFlight group",
      "env": {
        "GROUP_ID": ""
      },
      "steps": [
        {
          "name": "list_builds",
          "run": "asc builds list --app $APP_ID --sort -uploadedDate --limit 5"
        },
        {
          "name": "list_groups",
          "run": "asc testflight beta-groups list --app $APP_ID --limit 20"
        },
        {
          "name": "add_build_to_group",
          "if": "BUILD_ID",
          "run": "asc builds add-groups --build $BUILD_ID --group $GROUP_ID"
        }
      ]
    },
    "release": {
      "description": "Submit a version for App Store review",
      "steps": [
        {
          "workflow": "preflight",
          "with": {
            "NOTE": "running private sub-workflow"
          }
        },
        {
          "name": "submit",
          "run": "asc submit create --app $APP_ID --version $VERSION --build $BUILD_ID --confirm"
        }
      ]
    },
    "preflight": {
      "private": true,
      "description": "Private helper workflow (callable only via workflow steps)",
      "steps": [
        {
          "name": "preflight",
          "run": "echo \"$NOTE\""
        }
      ]
    }
  }
}
```

## Semantics

### Environment Merging

- Entry workflow env: `definition.env` -> `workflow.env` -> CLI params (`KEY:VALUE` / `KEY=VALUE`)
- Sub-workflow env: `sub_workflow.env` provides defaults, caller env overrides, and step `with` overrides win over everything.

### Conditionals

Add `"if": "VAR_NAME"` to a step to skip it when the variable is falsy.

Conditionals check the workflow env/params first, then fall back to `os.Getenv("VAR_NAME")`.
For deterministic behavior (especially in CI), prefer setting conditional variables in the workflow env or passing them as params.

Truthy values (case-insensitive): `1`, `true`, `yes`, `y`, `on`.

### Hooks

Hooks are definition-level commands:
- `before_all` runs once before any steps
- `after_all` runs once after all steps (only if steps succeeded)
- `error` runs on any failure (step failure, hook failure, max call depth, etc.)

Hooks are recorded in the structured JSON output as `hooks.before_all`, `hooks.after_all`, and `hooks.error`.

### Output Contract

- stdout: JSON-only (`asc workflow run` prints a structured result)
- stderr: step/hook command output, plus dry-run previews

This makes it safe to do:

```bash
asc workflow run beta BUILD_ID:123 GROUP_ID:xyz | jq -e '.status == "ok"'
```

