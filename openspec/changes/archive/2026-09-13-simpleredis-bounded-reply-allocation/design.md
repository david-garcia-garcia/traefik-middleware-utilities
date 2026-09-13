## Context

Dest `readBulk` (`simpleredis/resp.go`) still `make([]byte, length+2)` then `io.ReadFull` after the over-cap check. The `*` branch still `make([][]byte, count)`. Caps bound one allocation, not a header-only in-cap header. See proposal.md for why. Proceed policies: `devstate/explore.md`. Research: `knowledge/research/ext_redis_resp_bulk-string/`. Common-path ceilings: `TestAllocDecodeBulk` 112 B, `TestAllocDecodeBulk100KB` 127843 B, `TestAllocDecodeArray10` 344 B.

## Goals / Non-Goals

**Goals:**
- First bulk allocation `min(length+2, 128 KiB)`; grow with chunked `io.ReadFull`.
- Array start cap `min(count, 16)`; append as elements decode.
- Untagged `TotalAlloc` proof that header-only `$maxBulkLength` and `*maxArrayCount` stay far below the announced size.
- Keep dest sentinels, trailer CRLF check, and decode alloc ceilings.

**Non-Goals:**
- `io.CopyN` / `bytes.Buffer`.
- Deadline / `SetDeadline` changes (sibling BUG-3).
- Lowering or exposing the package caps.
- Cumulative array-reply byte budget (existing debt).

## Decisions

1. **Chunked `ReadFull`, not `CopyN`.** `io.CopyN` into `bytes.Buffer` uses a 32 KiB scratch that misses `decodeBulkBytes = 112`. Alternative: grow-from-zero `append` on every bulk — rejected; extra copies miss the 100 KB ceiling (`decode100KBAllocs = 3`).

2. **`bulkReadChunk = 128 << 10`.** First `make` is `min(length+2, bulkReadChunk)`. `$17` and `$102400` stay one `make` + one `ReadFull`. `$67108864` with no payload allocates 128 KiB then short-reads as `redis:unreachable`. Alternative: 32 KiB chunk — rejected; 100 KB would grow-copy. Alternative: lower `maxBulkLength` — rejected; that still trusts the declared length up to the new cap.

3. **`arrayGrowChunk = 16`.** `make([][]byte, 0, min(count, 16))` then append. Ten-slot bench keeps `cap == 10`. `*1048576` with no element allocates 16 headers. Alternative: `make([][]byte, 0, count)` — rejected; that is the same 24 MiB attack.

4. **Trailer still on a complete `length+2` buffer.** After the loop fills `need`, the last two bytes MUST be CRLF (`errIssue` if not). Short `ReadFull` in a chunk returns the IO error unchanged (`redis:unreachable`). Alternative: verify trailer in a second read — rejected; extra syscall, sibling trailer ticket already landed the two-byte check.

5. **Tests call `readReply` on `bufio.NewReader(strings.NewReader(...))`.** Prefix helpers `allocAmp`. `runtime.ReadMemStats` `TotalAlloc` around `$` + `strconv.Itoa(maxBulkLength)` and `*` + `strconv.Itoa(maxArrayCount)` with no payload. Fail if growth reaches the announced size. Keep over-cap tests. Alternative: only the tagged `simpleredis_bugs` file — rejected; ticket wants the default suite.

6. **Simplicity gate: implement this shape.** A loop in `readBulk` and append in `*` is small. Stop only if implement measures a common-path alloc or ns regression versus dest benches. Alternative: always incremental with no one-shot first chunk — rejected at explore (ceilings).

## Risks / Trade-offs

- [A 128 KiB first chunk still trusts an in-cap length up to 128 KiB] → Mitigation: that is ~1 MiB at `PoolSize` 8, versus 512 MiB today; the tagged repro `wantMax` is 8 MiB. Spec requires proportional-to-arrived for the ceiling-sized header, which this chunk satisfies.
- [Array of more than 16 real elements grows] → Mitigation: limiter MGET is small; `BenchmarkDecodeArray10` stays one backing alloc. Do not take cumulative budget.
- [Sibling BUG-3 also edits `readBulk`] → Mitigation: touch only the `make`/`ReadFull` block; leave `do` deadlines alone.
- [Common-path regression] → Mitigation: keep one-shot when `need <= bulkReadChunk`; run dest alloc tests and `go test -bench`.

## Migration Plan

Library behavior: hostile or buggy in-cap headers no longer `make` tens of MiB before a byte arrives. Honest small GET/MGET unchanged. Rollback is revert. No deploy key.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
