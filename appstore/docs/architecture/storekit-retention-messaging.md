# StoreKit Retention Messaging Support

Status: Implemented as stable
Research date: June 18, 2026
Target API: Retention Messaging API 1.5
Target command root: `asc storekit`

## Design

Retention Messaging is a StoreKit server API, not an App Store Connect API
resource. It has different production and sandbox hosts, a different error
envelope, and dedicated In-App Purchase API keys. It therefore lives in
`internal/storekit` and under the stable Monetization command root:

```text
asc storekit auth ...
asc storekit retention-messaging ...
```

This keeps StoreKit credentials out of the existing `asc auth` profile pool and
prevents an In-App Purchase key from being selected for an App Store Connect
request. The App Store Connect OpenAPI snapshot under `docs/openapi` does not
contain these endpoints.

Alternatives considered:

- Putting the commands under `asc subscriptions` would imply App Store Connect
  authentication and API ownership, both of which are incorrect.
- A raw HTTP command would expose the endpoints but would not safely validate
  image constraints, message limits, environment selection, or destructive
  operations.

## Endpoint mapping

| CLI command | Method and path |
| --- | --- |
| `images upload` | `PUT /inApps/v1/messaging/image/{imageIdentifier}` |
| `images list` | `GET /inApps/v1/messaging/image/list` |
| `images delete` | `DELETE /inApps/v1/messaging/image/{imageIdentifier}` |
| `messages upload` | `PUT /inApps/v1/messaging/message/{messageIdentifier}` |
| `messages list` | `GET /inApps/v1/messaging/message/list` |
| `messages delete` | `DELETE /inApps/v1/messaging/message/{messageIdentifier}` |
| `defaults set` | `PUT /inApps/v1/messaging/default/{productId}/{locale}` |
| `defaults view` | `GET /inApps/v1/messaging/default/{productId}/{locale}` |
| `defaults delete` | `DELETE /inApps/v1/messaging/default/{productId}/{locale}` |
| `endpoint set` | `PUT /inApps/v1/messaging/realtime/url` |
| `endpoint view` | `GET /inApps/v1/messaging/realtime/url` |
| `endpoint delete` | `DELETE /inApps/v1/messaging/realtime/url` |
| `performance start` | `POST /inApps/v1/messaging/performanceTest` |
| `performance view` / `wait` | `GET /inApps/v1/messaging/performanceTest/result/{requestId}` |

The base host is selected explicitly:

- `production`: `https://api.storekit.apple.com`
- `sandbox`: `https://api.storekit-sandbox.apple.com`

Performance-test commands reject the production environment. `performance
wait` polls no faster than every 10 seconds to remain within Apple's sandbox
rate limit.

## Authentication

Create an In-App Purchase API key in App Store Connect after Apple grants
Retention Messaging access. A browser session logged into App Store Connect can
be used to complete Apple's access-request form, but browser cookies are not API
credentials and the CLI does not read them.

Store a named profile:

```bash
asc storekit auth login \
  --name Production \
  --key-id "$KEY_ID" \
  --issuer-id "$ISSUER_ID" \
  --private-key ./SubscriptionKey.p8 \
  --bundle-id com.example.app
```

The keychain is preferred. Use `--bypass-keychain` for config-backed local
development or CI. Environment authentication supports:

```text
ASC_STOREKIT_KEY_ID
ASC_STOREKIT_ISSUER_ID
ASC_STOREKIT_PRIVATE_KEY_PATH
ASC_STOREKIT_PRIVATE_KEY
ASC_STOREKIT_PRIVATE_KEY_B64
ASC_STOREKIT_BUNDLE_ID
ASC_STOREKIT_ENVIRONMENT
ASC_STOREKIT_PROFILE
ASC_STOREKIT_STRICT_AUTH
ASC_STOREKIT_BYPASS_KEYCHAIN
```

Every request gets a newly signed ES256 JWT with `iss`, `iat`, `exp`,
`aud=appstoreconnect-v1`, and `bid=<bundle-id>`. The private key is parsed once
per client but tokens are never cached.

## Input and output contract

- The environment is always explicit through `--environment` or
  `ASC_STOREKIT_ENVIRONMENT`; there is no risky production default.
- Image and message identifiers are caller-generated UUIDs.
- Images must be PNG without transparency. `FULL_SIZE` requires width 3840 and
  height 160–2160; `BULLET_POINT` requires 1024×1024.
- Message payloads use `--file` and reject unknown JSON fields. The CLI validates
  Apple's text limits and requires alternative text for every image.
- Deletes and credential logout require `--confirm`.
- TTY-aware `json`, `table`, and `markdown` output is available on API commands.
- Invalid flags and local payloads return usage exit code `2`. API and transport
  failures return exit code `1` with Apple's error code and message.
- Upload image and upload message are not idempotent. The client never retries
  an ambiguous upload automatically.

Example message file:

```json
{
  "header": "Keep everything you unlocked",
  "body": "Continue your subscription and keep access to every feature.",
  "image": {
    "imageIdentifier": "22222222-2222-4222-8222-222222222222",
    "altText": "The Example app on an iPhone"
  },
  "headerPosition": "ABOVE_IMAGE"
}
```

## Sandbox verification

Use a disposable app and resources. The full live verification sequence is:

```bash
asc storekit auth doctor --environment sandbox --network

asc storekit retention-messaging images upload \
  --image-id 11111111-1111-4111-8111-111111111111 \
  --image-size FULL_SIZE \
  --file ./retention.png \
  --environment sandbox

asc storekit retention-messaging messages upload \
  --message-id 33333333-3333-4333-8333-333333333333 \
  --file ./message.json \
  --environment sandbox

asc storekit retention-messaging messages list --environment sandbox

asc storekit retention-messaging defaults set \
  --product-id com.example.monthly \
  --locale en-US \
  --message-id 33333333-3333-4333-8333-333333333333 \
  --environment sandbox

asc storekit retention-messaging endpoint set \
  --url https://example.com/retention-messaging-api/ \
  --environment sandbox

asc storekit retention-messaging performance start \
  --original-transaction-id 2000000000000000 \
  --environment sandbox \
  --wait
```

The original transaction ID must come from an actual StoreKit sandbox
subscription purchase. The endpoint must be publicly reachable over HTTPS and
implement Apple's Get Retention Message request/response contract. After the
test reports `PASS`, configure the production URL with `--environment
production`.

Clean up disposable resources with the matching `delete --confirm` commands.
Sandbox uploads are automatically approved, so list responses can be verified
without waiting for Apple review.

## Compatibility and tests

This adds a new root and a new optional `storekit` config object. Existing
commands, App Store Connect profiles, and output formats are unchanged. There
is no deprecation or migration.

The RED-to-GREEN test plan covers:

- all 14 documented HTTP operations, request methods, paths, query values,
  content types, and response decoding;
- ES256 header and StoreKit JWT claims;
- dedicated config credential lifecycle and strict environment resolution;
- command discovery, local payload validation, confirmation gates, and output;
- API error and millisecond `Retry-After` decoding;
- built-binary usage exit code `2`.

Live network verification is conditional on Apple granting access and valid
`ASC_STOREKIT_*` credentials being present. It is not replaced by an App Store
Connect browser login.

