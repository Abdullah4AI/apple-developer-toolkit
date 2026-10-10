# Request replay body allocation

HTTP retry setup used io.ReadAll for every body, repeatedly growing buffers even when bytes.Reader, bytes.Buffer or strings.Reader exposed an exact remaining length. Allocate one private snapshot for these three concrete types and consume it with io.ReadFull. Keep io.ReadAll and existing error wrapping for every other reader.

The snapshot continues to own its bytes; borrowing caller storage would allow later changes to alter retry payloads. Cursor position, empty/nil bodies, outbound headers, security validation, retry policy and response handling remain unchanged. No public flags, API endpoints or output contracts change.

Characterization covers partial consumption, empty/nil, arbitrary-reader errors, caller-buffer changes and repeated replay. Existing HTTP retry contracts are reused. Fixed-count before/after benchmarks at 512B, 64KiB and 1MiB measure time and allocations; see ignored build/performance/replay-evidence.md for measured results. Full gates and final review remain parent-owned.
