# Requirement
IssueKey: 2026-09-12-simpleredis-bug-04-bulk-trailer-not-verified

## Problem
`readBulk` reads `length+2` bytes and returns the payload without checking that the last two bytes are CRLF. A mis-framed bulk still parses as a clean value, so `do` returns `reusable == true` and `release` pools a socket that is no longer on a reply boundary. The next command on that socket reads leftover bytes from the previous reply.

## Current (code)
- `readBulk` allocates `length+2`, `io.ReadFull`, returns `data[:length]` with no trailer check: `simpleredis/resp.go:124-128`.
- Successful `$` in `readReply` is `clean == true`: `simpleredis/resp.go:66-74`. Array elements use the same `readBulk`: `simpleredis/resp.go:91-99`.
- `do` maps `errIssue` + `!clean` to `reusable == false`; any other `clean == true` path (including success) is reusable: `simpleredis/resp.go:21-28`.
- `release` closes when `reusable` is false; otherwise appends to `idleConns`: `simpleredis/pool.go:128-153`.
- `errIssue` is `redis:issue?` and is not retryable (`shouldRetry` is timeout / unreachable / a small Redis error-text set): `simpleredis/simpleredis.go:17-27`, `simpleredis/commands_exec.go:84-95`, `:107-118`.
- RESP2 bulk form is `$<length>\r\n<data>\r\n` (CRLF after payload, not counted in length): `knowledge/research/ext_redis_resp_bulk-string/notes.md`.
- `readLine` already rejects a missing CR before LF: `simpleredis/resp.go:147-149`. `TestMalformedReplyIsIssueAndNotPooled` covers that for header lines (`:42\n`), not a `$` payload trailer: `simpleredis/resp_test.go:84-115`.
- Truncated bulk (`io.ReadFull` short) is already `redis:unreachable`, not pooled: `simpleredis/resp_test.go:117-137`, `simpleredis/pool_test.go:376-408`. That is a short read, not a full read with a wrong trailer.
- No test feeds `readBulk` a full payload whose last two bytes are not CRLF. `startRawReplyRedis` already scripts per-Accept payloads and counts Accepts internally, but does not return the count: `simpleredis/fake_redis_test.go:774-822`.
- Dest `readLine` already uses `ReadSlice`: `simpleredis/resp.go:131-150`. The ticket's perf-07 note is already the dest path; the trailer check is still missing.

## Desired
- After a complete `io.ReadFull` of `length+2`, if `data[length]` is not `\r` or `data[length+1]` is not `\n`, return `errIssue` (so `readReply` is `clean == false` and `do` does not pool).
- Unit: `readBulk` with trailer `\n\r`, with two payload bytes as trailer, and legitimate `$0\r\n\r\n` still succeeding.
- Desync: `PoolSize: 1`, first reply a trailer-less bulk then a normal second reply. Second command must not return remnants of the first. Fake must show two Accepts (poisoned socket discarded). Reuse `startRawReplyRedis` rather than a second harness.

## Affected
- `simpleredis/resp.go` (`readBulk` trailer check)
- `simpleredis/resp_test.go` (unit + desync)
- `simpleredis/fake_redis_test.go` only if the desync test needs the existing Accept count exported from `startRawReplyRedis`

## Out of scope
- bug-02 length cap before `make` (neighbor finding; this ticket is the trailer check only).
- Other `simpleredisfixes2` files.
- Changing `readLine`, RESP3, nested arrays, or `*-1` (already dirty).
- Re-doing dest `ReadSlice` work (perf-07 already on dest).
- A second scripted-server helper beside `startRawReplyRedis`.

## Unknowns
- Whether exporting `startRawReplyRedis`'s Accept count is enough for the two-Accept assertion, or the test can infer two Accepts from the existing per-Accept payload index plus idle-empty.
- Ticket line numbers `resp.go:122-126` are dest `124-128` (same function, dest has moved).

## Tensions
- Ticket: apply the bug-02 length cap in the same `readBulk` edit. Caller: this run is the trailer check only. Length cap stays out of scope.
- Ticket: "build one harness" with test-04. Dest already has `startRawReplyRedis` used by truncated-bulk tests. Consume that; do not add a parallel fake.
- Ticket cites a measured `$5\r\nhello+OK\r\n` probe not in this tree. Dest code matches that parse (no trailer check). Tests to add are the ones under Desired, not a reproduction of the off-tree probe file.
