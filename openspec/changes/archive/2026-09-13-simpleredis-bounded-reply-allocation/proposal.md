## Why

An in-cap `$` or `*` header still chooses the size of `make` before any payload byte arrives. Eleven wire bytes `$67108864` allocate about 64 MiB; ten wire bytes `*1048576` allocate about 24 MiB of slice headers. The default pool of 8 multiplies that to about 512 MiB in the Traefik process.

## What Changes

- Stop treating an in-cap declared length as the first allocation size. Keep `maxBulkLength` and `maxArrayCount` as the hard ceiling. Do not put them on `Config`.
- Bulk: chunked `io.ReadFull` into a growing slice. First allocation is `min(length+2, 128 KiB)` so `$17` and `$102400` stay one-shot. A `$maxBulkLength` header with no payload allocates one 128 KiB chunk, then the short read fails. Trailer CRLF still checked on a complete `length+2` buffer.
- Array: start cap `min(count, 16)`, append as elements decode. A `*maxArrayCount` header with no element line does not `make` 1 Mi slot headers.
- Untagged `runtime.ReadMemStats` `TotalAlloc` proof for those two header-only cases. Prefix new helpers `allocAmp`.
- Sentinels unchanged. Diff is allocation only; do not change deadline handling.
- Out of scope: other packages, Config knobs, cumulative array-reply budget, tagged `PRODUCTION-BUGS.md`.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_simpleredis_resp-decode`: accepted in-cap `$`/`*` headers MUST NOT `make` the announced size before payload or elements arrive; memory stays proportional to bytes/elements that arrived, still capped. Over-ceiling rejection is unchanged.

## Impact

- `simpleredis/resp.go` (`readBulk`, `readReply` `*` case)
- `simpleredis/resp_test.go` (untagged header-only `TotalAlloc` proof)
- Main spec `std_go_simpleredis_resp-decode` after archive
- Usage packet `knowledge/devdocs/std_go_simpleredis_resp-decode.md` after apply
