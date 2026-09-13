# Explore
IssueKey: 2026-09-13-simpleredis-bounded-reply-allocation

## Concepts

The package already rejects a `$`/`*` length above `maxBulkLength` / `maxArrayCount` as `errIssue` before `make` (`simpleredis/resp.go`, `TestReadReplyOverCapIsIssue`, `TestReadReply256MiBHeaderDoesNotAllocatePayload`). A header *at* those caps is still the allocation size. `readBulk` does `make([]byte, length+2)` then `io.ReadFull`. The `*` branch does `make([][]byte, count)` then reads elements. Caps bound one allocation, not header-only amplification under the cap.

Reproduced on dest from the caller-only tagged test (`TestBugPeerControlledAllocationAmplification`, `//go:build simpleredis_bugs`):

```
bulk_header:  11 wire bytes -> 67,144,992 bytes allocated  (err redis:unreachable)
array_header: 10 wire bytes -> 25,188,272 bytes allocated  (err redis:unreachable)
```

Short `ReadFull` after that allocate is EOF → `ioError` → `redis:unreachable` (retried). Over-cap stays `errIssue` (not retried). Those sentinels must not swap.

Live decode spec (`openspec/specs/std_go_simpleredis_resp-decode/spec.md`) SHALL `make([]byte, length+2)` plus `io.ReadFull` for accepted lengths, and SHALL allocate `length+2` at or under the bulk ceiling. tcp-session short-read still names `io.ReadFull` of the announced payload (`$100` then 40 bytes then close). Usage packet `knowledge/devdocs/std_go_simpleredis_resp-decode.md` repeats make+ReadFull. Prior change `2026-09-12-simpleredis-reply-alloc-caps` kept that shape on purpose.

Common-path ceilings (`simpleredis/bench_test.go`): `TestAllocDecodeBulk` 3 allocs / 112 B (`$17`); `TestAllocDecodeArray10` 12 allocs / 344 B; `TestAllocDecodeBulk100KB` 3 allocs / 127843 B (`$102400`). A grow-from-zero or `io.CopyN` (32 KiB scratch) would miss those. Small replies are GET/INCR/limiter values, not 64 MiB.

```
header $N / *N
    │
    ├─ parseLen fail / negative *  → errIssue, dirty
    ├─ bulk N < 0                 → errMiss
    ├─ N > package cap            → errIssue, dirty, no make   (already dest)
    └─ dest: make(announced) + ReadFull
         └─ header-only → tens of MiB, then EOF → redis:unreachable
```

Research already on disk: `knowledge/research/ext_redis_resp_bulk-string/` (complete bulk is `$<length>\r\n<data>\r\n`; lying length then close is transport). `ext_redis_proto_max-bulk-len/` (Redis caps incoming client bulks, not GET replies). `ext_go-redis_proto_reader-limit/` (go-redis also make+ReadFull). No new third-party finding. Cumulative array *with real elements* is existing debt `knowledge/debt/2026-09-12-simpleredis-cumulative-array-reply-budget.md` (out of scope).

## Decisions

- Do not trust an in-cap declared length as the first `make` size. Keep the package caps as the hard ceiling. Do not put them on `Config`.
- Bulk: chunked `io.ReadFull` into a growing slice. First allocation is `min(length+2, bulkReadChunk)` with `bulkReadChunk = 128 << 10` so `$17` and `$102400` stay one `make` + one `ReadFull` (both benches). A `$maxBulkLength` header with no payload allocates one 128 KiB chunk, then the short read fails. Trailer CRLF still checked on a complete `length+2` buffer. Do not `io.CopyN` into `bytes.Buffer` (32 KiB scratch blows `decodeBulkBytes`).
- Array: `make([][]byte, 0, min(count, arrayGrowChunk))` then append as elements decode. `arrayGrowChunk = 16` so the 10-slot bench keeps `cap == count`. A `*maxArrayCount` header with no element line allocates 16 slot headers, then `readLine` fails.
- Sentinels unchanged: over-cap `errIssue`; truncated/short read `redis:unreachable`; `redis:unsupported-reply` untouched. Dirty stream stays unusable. Surgical: allocation only in `readBulk` and the `*` branch. Do not edit deadline handling (`do` / `SetDeadline`).
- Spec + usage: drop the accepted-length SHALL that `make`s the announced size. Replace with: memory for a `$` payload and a `*` slot slice is proportional to bytes/elements that arrived, still capped. tcp-session short-read stays `io.ReadFull` (chunked) and still `redis:unreachable`; the `$100`/40-byte fake stays.
- Simplicity gate: this shape is small (a loop in `readBulk`, append in `*`). Proceed to propose and implement. Stopping after propose would be for a common-path regression or a second mechanism (deadlines, Config, cumulative budget).
- Proof: untagged `runtime.ReadMemStats` `TotalAlloc` around `readReply` for `$maxBulkLength\r\n` and `*maxArrayCount\r\n` with no payload. Prefix new helpers `allocAmp…` so they cannot collide with sibling branches. Keep fuzz and `resp_test.go` green.

## Open questions

- Q: What first-chunk size keeps dest decode alloc ceilings while stopping header-only 64 MiB `make`?
  Rank: bounded asked — existing `readBulk` `make` and `TestAllocDecodeBulk` / `TestAllocDecodeBulk100KB` callers (2 benches + `readBulk` in `simpleredis/resp.go`); requirement Desired 1 and 4 name proportional memory and no common-path regression
  Decision: assumed — `bulkReadChunk = 128 << 10`. `$102400` (100 KiB + 2) is under that, so 100 KB stays one-shot. `$67108864` with no payload allocates 128 KiB then short-reads. Smaller chunk (32 KiB) would add grow-copies and miss `decode100KBAllocs = 3`.
  By: explore

- Q: What array start cap keeps `TestAllocDecodeArray10` while stopping a 1 Mi slot `make`?
  Rank: bounded asked — existing `readReply` `*` `make([][]byte, count)` and `BenchmarkDecodeArray10` (1 call site + 1 bench); requirement Desired 1 and 4
  Decision: assumed — `arrayGrowChunk = 16`. Ten elements: `min(10, 16) == 10`, same backing as dest. `*1048576` with no element: 16 slice headers, not 1 Mi. `make([][]byte, 0, count)` is not this — that is the same 24 MiB attack.
  By: explore

- Q: Does replacing the accepted-length make+ReadFull SHALL count as stopping at the simplicity gate?
  Rank: bounded asked — requirement Desired 1 names stop using declared length as allocation; live spec SHALL is the contract this change updates (enumerated: `openspec/specs/std_go_simpleredis_resp-decode/spec.md`, `std_go_simpleredis_tcp-session/spec.md` short-read sentence, `knowledge/devdocs/std_go_simpleredis_resp-decode.md`)
  Decision: assumed — no. Updating those three files is the job, not a second design. The run does not stop after propose. A measurable common-path alloc/ns regression at implement is the remaining stop signal.
  By: explore

- Q: Who already owns client identity (address, user, tenant, Host, trust hop) for this change?
  Rank: additive incidental — no identity reconstruct in this decode alloc
  Decision: assumed — none. The decoder classifies a RESP header; it does not set or rebuild a host fact.
  By: explore
