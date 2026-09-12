# Requirement
IssueKey: 2026-09-12-simpleredis-risk-03-readline-unbounded

## Problem
`readLine` still grows without a byte ceiling when a RESP line exceeds the `bufio` buffer. A peer that never sends `\n`, or that sends a huge terminated line, allocates until `IOTimeout` or the Traefik process OOMs. Legitimate headers are tiny; the bound should be a stated maximum per socket, not elapsed time.

## Current (code)
- `simpleredis/resp.go` `readLine` — `ReadSlice('\n')`; on `bufio.ErrBufferFull` copies the partial, then `ReadBytes('\n')` and appends. `ReadBytes` grows until delimiter or the reader errors. After a complete line, strip CRLF; `len < 2` or missing `\r` is `errIssue`.
- `simpleredis/resp.go` `readReply` — calls `readLine` for the reply head and again for every array-element head. `+`/`: ` copy `line[1:]` / `head[1:]` before return.
- `simpleredis/resp.go` `do` — `errIssue` returns `clean=false` and is not mapped through `ioError`.
- `simpleredis/commands_exec.go` `shouldRetry` — `errIssue` is not timeout, not unreachable, not a retryable Redis reply, so it is not retried.
- `simpleredis/pool.go` — `bufio.NewReader(netConn)` (Go default 4096). The reader is not enlarged.
- `simpleredis/config.go` — `defaultIOTimeout` is 1s; that caps time, not bytes.
- `simpleredis/resp_test.go` `TestLongStatusLineDecodes` — a 5000-byte `+` status succeeds (exercises the grow path).
- `simpleredis/resp_test.go` `TestMalformedReplyIsIssueAndNotPooled` — missing CR, empty line, HTTP-shaped head → `redis:issue?` and idle empty. No unterminated-stream allocation assertion.
- `openspec/specs/std_go_simpleredis_resp-decode/spec.md` — SHALL recover from a full buffer by `ReadBytes` remainder; a status/error line longer than 4096 SHALL decode; unit tests SHALL fail if that line is not decoded.
- `knowledge/devdocs/std_go_simpleredis_resp-decode.md` — same grow-on-`ErrBufferFull` recipe.
- `knowledge/research/ext_go-redis_proto_readline/notes.md` — go-redis also copy-partial + one `ReadBytes` (their buffer is 32 KiB). Dest copied that shape at 4096.

## Desired
- Bound the *allocation*, not only the returned length. Preferred: on `ReadSlice` `ErrBufferFull`, return `errIssue` and do not `ReadBytes` the remainder (nothing grows past the existing 4096 buffer).
- A post-`ReadBytes` `maxLineLength` (finding: 64 KiB) is the weaker option: it still grows first.
- Dirty socket, not retryable: keep `errIssue` through `do`.
- Proof: a terminated line just over the cap → `errIssue`, `clean == false`. An unterminated multi-megabyte stream → `errIssue` and allocation stays near the `bufio` buffer (not “error but still grew”). Keep shortest legal lines (`+OK`, `:1`, `$-1`) and the existing `len(line) < 2` reject.

## Affected
- `simpleredis/resp.go` (`readLine` `ErrBufferFull` path)
- `simpleredis/resp_test.go` (`TestLongStatusLineDecodes` today requires success; plus new over-cap / unterminated / short-line cases)
- `openspec/specs/std_go_simpleredis_resp-decode/spec.md` (today requires long-line decode)
- `knowledge/devdocs/std_go_simpleredis_resp-decode.md` (same recipe)

## Out of scope
- bug-02 unbounded `$` payload allocation (`readBulk` `make` of announced length).
- Other `simpleredisfixes2` files.
- Changing `IOTimeout`, pool size, idle reaper, or `bufio.NewReader` size.
- Importing `go-redis`.
- Re-doing the already-landed ReadSlice copy-on-escape for short `+`/`: ` payloads.

## Unknowns
- Cap at dest’s 4096 `ReadSlice` buffer versus the finding’s 64 KiB post-check. Ticket prefers stop-on-full-buffer.
- Whether any legitimate Redis `-ERR` this client must accept can exceed 4096 (dest currently decodes a 5000-byte `+` status in unit tests).
- How to pin allocation (`runtime.ReadMemStats` as the finding says, versus `testing.AllocsPerRun`) without a flaky heap sample.

## Tensions
- Finding cites `readLine` as `ReadBytes` only at `resp.go:130-139`. Dest already uses `ReadSlice` plus a `ReadBytes` remainder (perf-07 / `2026-09-11-simpleredis-readslice-decode` landed). The unbounded path that remains is that remainder.
- Finding says land together with perf-07 ReadSlice. Dest already has ReadSlice and chose grow-on-full, matching go-redis.
- Dest spec + `TestLongStatusLineDecodes` require decoding lines longer than 4096. This ticket requires rejecting them as `redis:issue?`.
- Finding’s 64 KiB post-`ReadBytes` cap vs dest 4096 buffer stop: honouring the letter of the first snippet still grows; the second snippet is the bound the ticket asks for.
- go-redis still grows via `ReadBytes`; dest copied that. This ticket wants to diverge for DoS.
