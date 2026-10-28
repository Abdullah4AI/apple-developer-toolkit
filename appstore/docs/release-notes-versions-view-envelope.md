# Versions view prints Apple's envelope

`asc versions view` and `asc versions list` now keep every `appStoreVersions`
attribute App Store Connect returns. Earlier releases dropped `reviewType` and
`usesIdfa` and removed null attributes such as `"earliestReleaseDate": null`.

`asc versions phased-release view` now prints `"data": null` when the version
has no phased release, instead of an empty `{"type":"","id":""}` resource.
`asc status` reports that phased release as `"configured": false` instead of
configured with an empty ID.

## 6.0.0: the default envelope

`asc versions view` now prints Apple's `appStoreVersions` envelope even without
`--include`. The reduced summary deprecated in 5.x has been removed. JSON keeps
all attributes, relationships, links, included resources, and null values Apple
returns. Table and Markdown output continue to render the version and its linked
build or submission.

`--include-build` and `--include-submission` remain supported and request the
same relationships as `--include build` and `--include appStoreVersionSubmission`.
They return the envelope as well. Update scripts that read the summary fields:

| Before | After |
|---|---|
| `asc versions view --version-id ID --include-build` | `asc versions view --version-id ID --include build` |
| `asc versions view --version-id ID --include-submission` | `asc versions view --version-id ID --include appStoreVersionSubmission` |
| `.versionString`, `.platform`, `.downloadable` | `.data.attributes.versionString`, `.data.attributes.platform`, `.data.attributes.downloadable` |
| `.state` | `.data.attributes.appVersionState` (falls back to `.data.attributes.appStoreState`) |
| `.buildId`, `.buildVersion` | `.data.relationships.build.data.id`, and the `builds` entry in `.included` |
| `.submissionId` | `.data.relationships.appStoreVersionSubmission.data.id` |
