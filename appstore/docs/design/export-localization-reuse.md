# Reuse localization results during exports

Metadata pull and migrate export already fetch the selected version's complete localization collection. Preview discovery then fetched that collection again. The callers now pass their existing collection to `ExportPlanWithVersionLocalizations`, which shares the original export implementation. `ExportPlan` keeps its existing signature and fetch behavior for callers without a collection.

The supplied collection belongs to one export invocation and is read without mutation. A private resolved marker distinguishes a fetched-empty collection from an unfetched collection. Metadata supplies it only when localizations were selected; previews-only pulls retain their own complete fetch. Migrate supplies the collection it already fetched for the same version. No global cache is introduced.

App Clip discovery still runs first. Preview sets, items, ordered linkage, file validation, warning/error handling, ownership preflight, and publication recovery remain unchanged. No endpoints, payloads, flags, or output contracts change.

The same paginated regression fixture failed before the change with four localization requests instead of two. Afterward, both combined export paths fetch each of the two pages once. Empty collections require one request instead of two. A second invocation returns new localization IDs, and previews-only behavior remains covered. Clip failures still prevent preview discovery; missing or omitted ordered assets fail before creating an output directory or success receipt.

Focused tests across metadata, migrate, storeassets and cmdtest passed with `ASC_BYPASS_KEYCHAIN=1 GOMAXPROCS=1 GOFLAGS=-p=4`, as did targeted storeassets/cmdtest race checks. Existing later-page failures, media round trips, ownership and rollback cases remain applicable. Evidence is retained under ignored `build/performance/export-reuse-*`. Request counts are the measurement; no live Apple latency reduction is claimed. Full repository validation and final review are recorded in the pull request.
