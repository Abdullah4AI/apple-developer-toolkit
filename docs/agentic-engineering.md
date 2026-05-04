# Agentic Engineering Compatibility

This repo receives recurring upstream sync commits. Keep the local skill packaging and agent rules compatible after each sync by running:

```bash
bash scripts/apply-agentic-engineering.sh
```

## What the script enforces

- `README.md` and `skills/apple-developer-toolkit/README.md` use the current Agent Skills command:
  - `npx skills add Abdullah4AI/apple-developer-toolkit`
- Codex and Cursor are listed as supported Agent Skills consumers.
- Duplicate trailing `## License / MIT` sections are removed from README files because the MIT badge already appears at the top.
- Unsupported image assets are removed from `appstore/docs/images/` so skill installers do not treat `.png` files as unsupported documents.
- `SKILL.md` keeps these required sections:
  - `Agent Skills Installation`
  - `Agent Safety, Permissions, and Observability`
  - memory discipline
  - tool permission design
  - observability requirements
  - workflow orchestration
- The root `AGENTS.md` and Cursor rule file explain how coding agents should operate in this repo.

## Verification

```bash
git diff --check -- README.md SKILL.md skills/apple-developer-toolkit/README.md AGENTS.md .cursor/rules/agentic-engineering.mdc
npx skills add . --list --full-depth
npx skills add . --agent codex cursor --skill apple-developer-toolkit -y --copy
```

Use a temporary directory for install verification if you do not want to modify the working tree.
