# IAP review screenshot update

`asc iap review-screenshots update` supports upload finalization and resuming
an in-progress file upload.

Use `--checksum` and/or `--uploaded` to finalize an existing upload reservation
with Apple's supported PATCH attributes. These attributes do not replace a
completed asset.

Use `--file` to resume an existing screenshot reservation when the API returns
`uploadOperations`; the file size must match the reservation, and the command
uploads, commits, and verifies the same screenshot ID. App Store Connect
rejects creating a second IAP review screenshot while a completed one exists,
and its update schema has no asset-replacement field. Therefore update fails
before any POST, PATCH, or DELETE for a completed screenshot and prints the
manual delete-then-create workflow. The delete command requires `--confirm`,
so the existing screenshot is never removed implicitly.
