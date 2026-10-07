# OpenAPI snapshot (offline)

This folder keeps an offline snapshot of the App Store Connect OpenAPI spec for
agents that cannot access the internet.

## Files

- `latest.json`: full OpenAPI spec snapshot (see source below)
- `paths.txt`: generated path+method index for quick existence checks

## Source

Preferred sources for the OpenAPI spec:

- Official Apple download (zip): `https://developer.apple.com/sample-code/app-store-connect/app-store-connect-openapi-specification.zip`
- Community mirror that tracks Apple's published spec: `https://github.com/EvanBacon/App-Store-Connect-OpenAPI-Spec`

Note: The published OpenAPI spec can lag reality and may omit some operations that
still work in the API (parity checks can surface these gaps).

## Update process

1. Replace `latest.json` with a newer spec file.
2. Run `make update-openapi` to regenerate `paths.txt`.
3. Run `make update-schema-index` to regenerate the runtime `asc schema` index.
4. Run `make check-docs` to verify both generated indexes are current.
5. Update the "Last synced" date below and commit the snapshot and indexes together.

Last synced: 2026-10-07

Snapshot version: `4.5.1`, from the official Apple archive listed above.

- Archive SHA-256: `7b826ae9ca3e45b0e7df2643cbb811c9930632640944db11a8af4586fab9db46`
- Extracted `openapi.oas.json` SHA-256: `7518d3a94a8bd701ac25c1c601b95b8f53aad92091affb15a2a9d331713dac1a`

The snapshot preserves the extracted JSON bytes. Both indexes are generated
from that snapshot using the update process above.
