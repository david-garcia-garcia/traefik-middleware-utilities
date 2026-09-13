# Requirement
IssueKey: 2026-09-13-simpleredis-bounded-reply-allocation

## Problem
A `$` or `*` header at or under the package caps still chooses the size of `make` before any payload byte arrives. Eleven wire bytes `$67108864\r\n` allocate ~64 MiB; ten wire bytes `*1048576\r\n` allocate ~24 MiB of slice headers. `PoolSize` concurrent commands multiply that (default 8 → ~512 MiB from ~88 header bytes). That allocation is in the Traefik process.

## Current (code)
- `simpleredis/resp.go` `readBulk` — after miss and `length > maxBulkLength` → `errIssue`, `make([]byte, length+2)` then `io.ReadFull`. Trailer not CRLF → `errIssue`. Short `ReadFull` returns the IO error (later `ioError` → `redis:unreachable`).
- `simpleredis/resp.go` `readReply` `*` case — after `count > maxArrayCount` → `errIssue`, `make([][]byte, count)` then a per-element loop. A header with no following element lines still allocates the slot slice first; the next `readLine` then fails.
- `simpleredis/resp.go` consts — `maxBulkLength = 64 << 20`, `maxArrayCount = 1 << 20`.
- `simpleredis/config.go` — `defaultPoolSize = 8`, `defaultIOTimeout = 100 * time.Millisecond`. Caps are not `Config` fields.
- `simpleredis/resp.go` `do` / `isDirtyProtocolError` / `ioError` — `errIssue` / `errUnsupportedReply` stay those sentinels and dirty; other IO including EOF becomes `errUnreachable` / timeout.
- `simpleredis/commands_exec.go` `shouldRetry` — `errUnreachable` retries; `errIssue` does not.
- `simpleredis/resp_test.go` — over-cap `$`/`*` and `$268435456` are `errIssue` without payload `make` (`TestReadReplyOverCapIsIssue`, `TestReadReply256MiBHeaderDoesNotAllocatePayload`). No assertion that an *in-cap* `$maxBulkLength` / `*maxArrayCount` header with no payload stays proportional to the header.
- `simpleredis/bench_test.go` — `TestAllocDecodeBulk` / `TestAllocDecodeBulk100KB` ceilings (`decodeBulkBytes = 112`, `decode100KBBytes = 127843`).
- `openspec/specs/std_go_simpleredis_resp-decode/spec.md` — accepted bulk lengths SHALL `make([]byte, length+2)` plus `io.ReadFull`; over-ceiling MUST NOT allocate. Same for array count vs `make` of that announced size.
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — short bulk is `redis:unreachable` not `redis:issue?`; the short-read path is still `io.ReadFull` of the announced payload (`$100` then 40 bytes then close).
- `knowledge/devdocs/std_go_simpleredis_resp-decode.md` — same make+ReadFull instruction for accepted lengths.
- `knowledge/research/ext_redis_resp_bulk-string/notes.md` — complete bulk is `$<length>\r\n<data>\r\n`; a lying length then close is transport, not a Redis command.
- `knowledge/research/ext_redis_proto_max-bulk-len/notes.md` — Redis `proto-max-bulk-len` default 512 MiB caps *incoming client* bulks, not GET replies the server emits.
- `knowledge/research/ext_go-redis_proto_reader-limit/notes.md` — go-redis also `make([]byte, n+2)` then `ReadFull` with no payload cap on this pin.
- `knowledge/debt/2026-09-12-simpleredis-cumulative-array-reply-budget.md` — leftover from the cap ticket: `maxArrayCount` slots each at `maxBulkLength` if the peer actually sends them.
- Caller-only repro (not on dest): `D:\repositories\traefik-middleware-utilities\simpleredis\PRODUCTION-BUGS.md` BUG-4; `bugs_production_test.go` `TestBugPeerControlledAllocationAmplification` (`//go:build simpleredis_bugs`). Dest has neither file.

## Desired
1. Do not treat a peer-supplied in-cap length as an allocation instruction. Memory for `$` payloads and `*` slots must stay proportional to bytes that actually arrived, still hard-capped at `maxBulkLength` / `maxArrayCount`.
2. Ticket shape to evaluate (not mandated if a smaller shape exists): grow a bulk buffer as bytes arrive (`io.CopyN` loop into `bytes.Buffer`, or `append`); append array elements as they decode instead of `make([][]byte, count)`.
3. Keep the existing package caps as the ceiling. Do not put them on `Config`.
4. Small replies are the common path: must not get slower or allocate more than dest. A measurable common-path regression is a signal to reconsider the shape (simplicity gate: stop after propose if the correct fix is not small).
5. Permanent untagged test: oversized-but-in-cap `$` header and `*` header, each with no payload, `runtime.ReadMemStats` `TotalAlloc` around the call, proportional to what arrived. Prefix new fakes/helpers so they cannot collide with sibling branches.
6. Sentinels stay: over-cap `errIssue`; truncated/short read `redis:unreachable`; `redis:unsupported-reply` unchanged. Dirty stream stays unusable. `OverFrees() == 0`. No fd/goroutine leaks.
7. Diff surgical: allocation only. Do not change deadline handling (BUG-3 also edits `readBulk`).

## Affected
- `simpleredis/resp.go` (`readBulk`, `readReply` `*` case)
- `simpleredis/resp_test.go` (new default-suite alloc test)
- `openspec/specs/std_go_simpleredis_resp-decode/spec.md` (accepted-length make+ReadFull SHALL)
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` (short-read still `io.ReadFull` wording)
- `knowledge/devdocs/std_go_simpleredis_resp-decode.md` (same make+ReadFull how-to)

## Out of scope
- Any package other than `simpleredis`
- Per-read / progress deadline (PRODUCTION-BUGS BUG-3)
- Socket desync / leftover-as-length (separate defect)
- Changing which sentinel each malformed shape produces
- Exposing caps on `Config`
- Cumulative budget when the peer *does* send `maxArrayCount` real elements (existing debt file)
- New non-stdlib imports, `unsafe`, reflection
- Editing `PRODUCTION-BUGS.md` / tagged `bugs_production_test.go` (caller-only, not on dest)

## Unknowns
- Whether grow-as-you-go (`bytes.Buffer` / `append`) stays inside `decodeBulkBytes` / `decode100KBBytes` on the compiled Go 1.21 path. Ticket: if it regresses, reconsider.
- Whether the simplest correct fix is small enough to implement, or this run stops after propose (ticket simplicity gate).
- Which sibling branch is the BUG-3 per-read-deadline edit of `readBulk` (conflict expected).

## Tensions
- Live decode spec, tcp-session spec, and usage doc require `make([]byte, length+2)` + `io.ReadFull` for accepted lengths. The ticket asks to stop using declared length as the allocation size. Requirement stays the ticket; that spec SHALL is a reshape later phases must record if they take it.
- Prior change `2026-09-12-simpleredis-bug-02-unbounded-reply-allocation` (PR 34) landed the caps and kept make+ReadFull. This ticket says those caps bound one allocation, not header-only amplification under the cap.
- Caller names `TestBugPeerControlledAllocation`; the untracked file's test is `TestBugPeerControlledAllocationAmplification`.
- Ticket ranks this below availability bugs; still wants a fix because of amplification and because a desynced socket can feed cache bytes as length headers.
