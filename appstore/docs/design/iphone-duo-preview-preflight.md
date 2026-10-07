# iPhone Duo preview preflight

Metadata push and migration already inspect App Preview media with ffprobe,
but their shared accepted-resolution table omits `IPHONE_DUO`. A finite,
20-second Duo preview at 886×1920 or 1920×886 therefore fails locally even
though direct preview upload recognizes the public API enum.

Add Duo to the existing 886×1920 resolution family. Keep the existing duration,
poster-frame, rotation, file-size, and ffprobe dependency behavior. Native Duo
screenshot resolutions remain invalid for previews. No command flags, help,
API payloads, or direct-upload dependencies change.

Apple's accepted delivery dimensions are documented in the
[App Preview specifications](https://developer.apple.com/help/app-store-connect/reference/app-information/app-preview-specifications).

The focused regression exercises the existing ffprobe JSON parsing and preview
preflight with valid portrait, landscape, and rotated media reports; it also
rejects all four native Duo screenshot dimensions. It verifies local preflight,
not actual video encoding or Apple processing acceptance.

Focused check:

```sh
ASC_BYPASS_KEYCHAIN=1 GOMAXPROCS=1 go test -p=1 -parallel=1 ./internal/cli/storeassets -run '^TestDuoPreviewPreflight$' -count=1
```
