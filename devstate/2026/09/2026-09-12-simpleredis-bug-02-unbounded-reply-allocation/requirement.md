# Requirement
IssueKey: 2026-09-12-simpleredis-bug-02-unbounded-reply-allocation

## Problem
A RESP `$` or `*` header chooses the size of `make` before any payload byte arrives. Lengths that fit in `int` allocate or panic. A truncated ReadFull after that allocate is EOF → `redis:unreachable`, which retries and repeats the allocation.

## Current (code)
- `simpleredis/resp.go` `readBulk` — `parseLen` then, when length >= 0, `make([]byte, length+2)` and `io.ReadFull`. Negative length is `errMiss`. No ceiling besides `parseLen` overflow (`maxParseLen`).
- `simpleredis/resp.go` `readReply` `*` case — `make([][]byte, count)` when `parseLen` succeeds and count >= 0. No array-count ceiling. Array `$` elements call the same `readBulk`.
- `simpleredis/resp.go` `parseLen` — decimal digits with overflow at MaxInt; overflow is false → `errIssue`. Dest does not use `strconv.Atoi`.
- `simpleredis/resp.go` `do` / `ioError` — `errIssue` stays `errIssue` and dirty; other IO including EOF becomes `errUnreachable`.
- `simpleredis/commands_exec.go` `shouldRetry` / `retryLimits` — `errUnreachable` retries; `errIssue` does not. `MaxRetries` 0 means 3 extra attempts.
- `simpleredis/commands_msetex.go` — write fan-out capped at `maxMSetEXPairs = 1024`. No matching read-side const.
- `simpleredis/config.go` — no bulk/array cap fields.
- `simpleredis/resp_test.go` — garbage length and truncated `$10` / short array; `TestParseLen` overflow string. No over-cap `$`/`*` case, no `$268435456` allocation guard, no `readReply` table for MaxInt panic lengths.
- `openspec/specs/std_go_simpleredis_resp-decode/spec.md` — `readBulk` SHALL keep `make([]byte, length+2)` plus `io.ReadFull`. No reply-size ceiling.
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` — unparseable / negative array count is `redis:issue?`; truncated bulk/array is I/O (`redis:unreachable` on EOF).
- `knowledge/research/index_ext_redis.md` / `index_ext_go-redis.md` — no `proto-max-bulk-len` or go-redis reader-limit finding.

## Desired
1. Package `const` ceilings `maxBulkLength = 64 << 20` and `maxArrayCount = 1 << 20`. Not `Config` fields.
2. After the bulk miss check, length > `maxBulkLength` returns `errIssue` (not IO) without that `make`.
3. Array count > `maxArrayCount` returns `errIssue`, `clean == false`.
4. Array `$` elements inherit the bulk cap via `readBulk`; prove it with a test (MGET / limiter path).
5. Proof on `readReply` with `bufio.Reader`/`strings.Reader`: `$` and `*` just over each cap, at MaxInt64, and at lengths that currently panic; `errIssue` and `clean == false`. Keep an overflow-length regression (`parseLen` already rejects a huge digit string). Allocation assertion around `$268435456\r\n` so the test fails if `make` still ran.

## Affected
- `simpleredis/resp.go` (`readBulk`, `readReply` `*` case, new consts)
- `simpleredis/resp_test.go`
- `openspec/specs/std_go_simpleredis_resp-decode/spec.md` (ceiling; keep `make([]byte, length+2)` for accepted lengths)
- `openspec/specs/std_go_simpleredis_resp-commands/spec.md` (over-cap header is `redis:issue?`, not truncated I/O)

## Out of scope
- Neighbor [bug-04](simpleredisfixes2) bulk-trailer verification
- [bug-01](simpleredisfixes2) panic-leaks-pool-token (this change only makes that panic less likely)
- [ci-01](simpleredisfixes2) race detector / CI fuzz job
- A permanent `Fuzz` target (ticket “worth adding”)
- Cumulative array-reply budget (ticket “consider also”)
- Exposing the caps on `Config`
- `readLine` unbounded remainder (`ReadBytes`) — other finding
- Other `simpleredisfixes2` files

## Unknowns
- Redis `proto-max-bulk-len` default (ticket: 512 MB) and go-redis reader limit are not in `knowledge/research/`.
- Whether 64 MiB / `1 << 20` stay the numbers once explore weighs the existing 100 KB decode alloc guard (`simpleredis/bench_test.go` `TestAllocDecodeBulk100KB`).
- How to spell MaxInt64 cases on 32-bit `int` (`parseLen` already fails above `maxParseLen`).

## Tensions
- Ticket cites `strconv.Atoi` at `resp.go:122` / `:79`. Dest uses `parseLen`; `make` still honours every in-range `int`. Overflow is already `errIssue`; unbounded allocate of 0..MaxInt remains.
- Ticket: keep `$99999999999999999999` Atoi overflow. Dest `TestParseLen` uses a longer overflow string, not that literal.
- Decode spec requires `make([]byte, length+2)` + `ReadFull`. Cap sits before `make`; accepted lengths keep that shape.
- Truncated *small* bulk stays `redis:unreachable` (tcp-session spec). A lying over-cap header with no payload is the same EOF path today; ticket wants `errIssue` before allocate.
- Ticket “consider also” cumulative array budget vs bound-the-ask (per-element + count caps only).
