# Overlap independent release work

Audit cutoff: 2026-10-07 17:07:51 UTC. Main was c7e9c6466698de6736f1037e042932d1185d65f3. This change builds on the Go 1.27 migration at 8125cbc58f85e1e7dd5c0e2a0b65ebc1b7509b1b.

## Observed cost

Completed release runs spent substantial time running quality checks before building and notarizing binaries on the same macOS runner:

| Release | Run | Successful build job | Guardrails | Target compilation | Notarization |
| --- | --- | --- | --- | --- | --- |
| 5.9.1 | 36936639591 | 29:56 | 17:21 | 6:50 | 1:13 |
| 5.10.0 | 37214652207 | 14:36 | 9:01 | 2:34 | 0:55 |
| 5.12.0 | 37407603116 | 28:02 | 17:19 | 5:18 | 1:11 |
| 5.13.0 | 37532599738, attempt 2 | 64:06 | 18:33 | 5:20 | 36:13 |

The 5.13.0 first attempt also hit a one-hour notarization timeout. Queue waits were 6–12 seconds across the audited runs. Compilation and check scheduling can improve; these changes do not reduce Apple's processing time.

## Change and tradeoff

Resolve qualified candidate reuse and freeze the full release source SHA first. For fresh candidates, run macOS quality checks, macOS compilation/signing/notarization, and Ubuntu compilation of Linux/Windows binaries on separate runners. Each producer checks out the frozen SHA and disables shared Go caches. Separate runners duplicate some downloads and compilation, trading runner work for shorter elapsed time.

Use the native macOS release executable for notarization instead of compiling an additional bootstrap CLI. Keep Apple submissions and waits serial. Join both platform artifacts only after every fresh gate succeeds, verify the source SHA and exact target set, preserve executable modes, and create checksums before uploading the qualified candidate. Failed, cancelled, or skipped fresh gates cannot qualify an artifact.

Partial reruns find the latest unexpired artifact for each platform within the same workflow run, so successful work from an earlier attempt remains usable. Qualified candidate reuse, already-published repair, and publication/distribution checks remain. Historical releases build the selected tag while assembly tools come from the workflow SHA.

Same-run candidate reuse also downloads the retained artifact and checks its commit sidecar against the selected release source before skipping work. The final integration review reproduced reuse of an old candidate after its tag moved between attempts. Missing, damaged, or mismatched candidates now fail closed; rebuilding under the existing immutable artifact name would conflict. `TestReleaseWorkflowSameRunCandidateProvenance` runs the actual resolver with matching and moved source commits, missing files, damaged ZIP data, and failed downloads. The five invalid cases failed before this check was added; matching-source reruns remain reusable.

This follows the independent platform jobs and artifact join used by [GitHub CLI's release workflow](https://github.com/cli/cli/blob/996be976cad960a726aabb94be67dfc7e31e317b/.github/workflows/deployment.yml), while preserving ASC's clean-build and notarization requirements.

## Verification

Workflow mutation tests reject weakened job gates, shared release caches, and incorrect source/tool revisions. Real tar fixtures exercise provenance, exact asset sets, bytes, modes, malformed members, and existing-output preservation. The actual inline artifact resolver runs against paginated, mixed-attempt fixtures; missing or expired lanes fail closed. Existing repair and publication tests remain, with checksum assertions at each consumer boundary.

Run the focused release tests, actionlint, build, formatting, docs, lint, full tests, and local branch review. No production release or Apple mutation is needed for these checks. Hosted release elapsed time remains unverified until an authorized release exercises this workflow.
