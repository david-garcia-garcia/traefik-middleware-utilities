## Why

A RESP `$` or `*` header currently chooses the size of `make` before any payload byte arrives. Lengths that fit in `int` allocate or panic. A truncated ReadFull after that allocate is EOF → `redis:unreachable`, which retries and repeats the allocation. A 12-byte `$268435456` header is enough to size a 256 MiB heap slice.

## What Changes

- Add package consts `maxBulkLength = 64 << 20` and `maxArrayCount = 1 << 20`. Not `Config` fields.
- After the bulk miss check, length > `maxBulkLength` returns `redis:issue?` without that `make`. Array count > `maxArrayCount` returns `redis:issue?`, `clean == false`.
- Array `$` elements inherit the bulk cap via `readBulk`.
- Keep `make([]byte, length+2)` plus `io.ReadFull` for accepted lengths.
- Prove on `readReply` with `bufio.Reader`/`strings.Reader`: `$` and `*` just over each cap, MaxInt64 header digits, lengths that currently panic, and `$268435456\r\n` so the test fails if `make` still ran. Keep the `parseLen` overflow-digit regression.
- Out of scope: bulk-trailer verification, panic-leaks-pool-token, CI fuzz, a permanent Fuzz target, cumulative array-reply budget, exposing caps on `Config`, `readLine` unbounded remainder.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_resp-decode`: bulk and array headers above the package caps MUST NOT `make`; accepted lengths keep `make([]byte, length+2)` plus `io.ReadFull`.
- `std_go_simpleredis_resp-commands`: an over-cap `$` or `*` header is `redis:issue?` (dirty, not retried), not truncated I/O.

## Impact

- `simpleredis/resp.go` (`readBulk`, `readReply` `*` case, new consts)
- `simpleredis/resp_test.go`
- Main specs `std_go_simpleredis_resp-decode` and `std_go_simpleredis_resp-commands` after archive
- Usage packet `knowledge/devdocs/std_go_simpleredis_resp-decode.md` after apply (ceiling gotcha)
