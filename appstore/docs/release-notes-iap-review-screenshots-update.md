# IAP review screenshot update

`asc iap review-screenshots update` now maps directly to Apple's supported
PATCH attributes. Use `--checksum` and/or `--uploaded` with
`--screenshot-id` to update an existing screenshot.

The previous file-based re-upload form is retained as a hidden deprecated
compatibility flag. It now stops before any API request and explains the
migration: upload a replacement with
`asc iap review-screenshots create --iap-id "IAP_ID" --file "./review.png"`,
delete the old screenshot with
`asc iap review-screenshots delete --screenshot-id "SHOT_ID" --confirm`, or
patch the existing resource with the supported update flags. The compatibility
flag may be removed in a future major release.
