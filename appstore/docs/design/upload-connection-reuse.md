# Reuse screenshot and preview upload connections

Screenshot and preview batches created a separate upload transport for every
asset. A ten-asset HTTP/1.1 TLS fixture opened ten connections. The regression
initially called the existing package-level `UploadAssetFromFile` and failed
because it expected one connection; this was a behavioral failure, not a build
failure. Evidence is in `build/performance/upload-pool-red.txt`.

The ASC client now owns a lazily initialized upload transport. Screenshot and
preview uploads use its `UploadAssetFromFile` method, and the command closes idle
owned connections after its workers finish. Every upload invocation still gets
a fresh HTTP client with the current upload timeout. The transport is separate
from the API client, so API cookies, middleware, and authentication cannot carry
into uploads. Signed operation headers remain request-local; redirect rejection,
retry body reconstruction, operation validation, nil-context handling, and file
worker limits are unchanged. Custom default transports remain externally owned.
The package-level upload function retains its existing contract and signature.

Only screenshots and previews adopt the shared pool. No command flags, output,
API payloads, endpoints, migration, or deprecation change. Captured current help
is in `build/performance/upload-help.txt` and `preview-help.txt`.

A process-global pool would make cleanup and transport test isolation harder.
A context-carried pool could be dropped by timeout-parent context restoration.
Client ownership follows the existing per-command client lifetime instead.

## Verification

Commands use `ASC_BYPASS_KEYCHAIN=1 GOMAXPROCS=1 GOFLAGS=-p=4`.

- RED: `go test ./internal/asc -run '^TestClientAssetUploadsReuseConnections$' -count=1`
- Focused GREEN: `go test ./internal/asc -run '^TestClient(AssetUploadsReuseConnections|UploadPool)' -count=1`
- Adjacent tests: `go test ./internal/asc ./internal/cli/assets -count=1`
- Race check: `go test -race ./internal/asc -run '^TestClient(AssetUploadsReuseConnections|UploadPool)' -count=1`
- Benchmark: `go test ./internal/asc -run '^$' -bench '^BenchmarkAssetUploadConnections$' -benchtime=10x -benchmem -count=3`

Tests cover reuse, simultaneous uploads through one pool, retry bytes and
operation headers, isolation from an API transport and cookie jar, redirect
rejection, current timeout resolution, idle-connection cleanup, and custom
transport ownership. Existing package tests cover the shared executor's detailed
retry, timeout, validation, resume, rollback, and output behavior.

The local TLS benchmark uploads ten small assets per batch. Each improved batch
owns and closes its client pool. Three baseline runs took 11.16–11.92 ms per
batch, opening ten connections, allocating 1.47–1.49 MB and 9,412–9,451 objects.
Three improved runs took 1.71–2.54 ms, opening one connection, allocating about
200 KB and 1,675–1,678 objects. These are fixture measurements on Apple M5;
network latency, upload size, provider origins, and keep-alive policies determine
actual App Store Connect savings. No live Apple mutations were performed.

Repository-wide validation and final branch review are recorded in the pull request.

Go documents that [transports retain connections for reuse and support concurrent requests](https://pkg.go.dev/net/http#Transport).
