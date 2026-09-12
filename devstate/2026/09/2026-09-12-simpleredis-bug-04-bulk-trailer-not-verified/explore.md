# Explore
IssueKey: 2026-09-12-simpleredis-bug-04-bulk-trailer-not-verified

## Concepts

- **Bulk trailer**: the two bytes after a RESP2 `$` payload. Official form is `$<length>\r\n<data>\r\n`; those last two bytes are CRLF and are not counted in length (`knowledge/research/ext_redis_resp_bulk-string/notes.md`).
- **`readBulk`**: allocates `length+2`, `io.ReadFull`, returns `data[:length]`. Dest `simpleredis/resp.go:124-128` does not inspect the trailer. Array `$` elements use the same function (`simpleredis/resp.go:91-99`).
- **`clean` / `reusable`**: `readReply` sets `clean == false` on `readBulk` error except `errMiss`. `do` maps `errIssue` + `!clean` to `reusable == false`. `release` closes a non-reusable socket (`simpleredis/pool.go:128-132`).
- **Wrong trailer vs short read**: a full `ReadFull` whose last two bytes are not CRLF is protocol garbage (`redis:issue?`). A short `ReadFull` is already `redis:unreachable` (`TestTruncatedBulkIsUnreachableAndNotPooled`).
- **`startRawReplyRedis`**: one canned payload per Accept (`simpleredis/fake_redis_test.go:780-822`). Indexes by Accept, not command. `closeAfter` currently still hits `defer conn.Close()` on both branches. One caller: `TestTruncatedBulkIsUnreachableAndNotPooled` (`closeAfter: true`).
- **Hosts**: fold into existing `std_go_simpleredis_resp-decode` (readBulk contract) and `std_go_simpleredis_resp-commands` (malformed RESP not pooled). Usage gap: `knowledge/devdocs/std_go_simpleredis_resp-decode.md` tells implementers to leave `readBulk` as allocate+ReadFull with no trailer check.

## Decisions

- Fix the cause in `readBulk`: after a complete `io.ReadFull` of `length+2`, if `data[length] != '\r'` or `data[length+1] != '\n'`, return `errIssue`. Do not paper over at `readReply`.
- Keep `make([]byte, length+2)` plus `io.ReadFull`. Do not apply bug-02’s length cap (Out of scope).
- Unit tests call `readBulk` the same way `TestReadBulkNonDollarHeadIsIssue` does: trailer `\n\r`, two payload bytes used as trailer, legitimate `$0\r\n\r\n` still succeeds.
- Desync uses `startRawReplyRedis` (no second harness). `PoolSize: 1`, `MaxRetries: -1`. First payload is a complete bulk whose trailer is not CRLF and that still has leftover bytes in the client reader (`$5\r\nhello+OK\r\n`). Second Accept writes a normal bulk. First Get is `redis:issue?`, idle empty; second Get returns the second payload, not remnants of the first.
- Two Accepts are inferred the way truncated-bulk already does: idle empty after the dirty Get, and the later Get returns the second `rawReply` payload. Do not export the helper’s Accept counter.
- `closeAfter: false` must actually keep the socket until the client disconnects, otherwise the second write dies on a closed peer and remnants in the pooled `bufio.Reader` are never observed. Leave `closeAfter: true` as write-then-close (truncated tests).
- Specs: MODIFIED `std_go_simpleredis_resp-decode` and `std_go_simpleredis_resp-commands`. Trailer mismatch is `redis:issue?`, sibling of missing CR on header lines. Do not widen the tcp-session truncated-bulk requirement (that is short `ReadFull`).
- No new research folder: RESP2 bulk form is already sourced. Usage packet `std_go_simpleredis_resp-decode.md` needs a trailer sentence at implement / devdocs-impact.

Reproduced: dest `readBulk($5, "hello+OK\r\n")` returns `"hello", nil` (`go test ./simpleredis -run TestThrowawayBulkTrailerNotVerified`; throwaway deleted).

## Open questions

- Q: How does the desync test prove two Accepts without exporting `startRawReplyRedis`’s Accept counter?
  Rank: additive asked — requirement Unknowns names export vs infer; Desired names two Accepts
  Decision: resolved — do not export. Infer two Accepts like `TestTruncatedBulkIsUnreachableAndNotPooled`: idle empty after the dirty Get, and the later Get returns the second `rawReply` payload (Accept index 1).
  By: explore

- Q: Must `serveRawReply` keep the connection open when `closeAfter` is false so leftover bytes in the pooled reader can be read on the next command?
  Rank: bounded incidental — 1 existing `startRawReplyRedis` caller (`simpleredis/pool_test.go:380-382`), both replies `closeAfter: true`; no criterion names keep-alive (it is a means to remnants)
  Decision: assumed — when `closeAfter` is false, wait for the client to disconnect before Close; leave `closeAfter: true` as write-then-close. Without keep-alive, the second Get’s write fails on a dead socket and remnants are not measured.
  By: explore

- Q: Which spec leaves take the trailer check?
  Rank: additive asked — Desired trailer check; existing malformed-RESP and readBulk contracts
  Decision: assumed — MODIFIED `std_go_simpleredis_resp-decode` (`readBulk` SHALL verify CRLF after a complete ReadFull) and `std_go_simpleredis_resp-commands` (malformed RESP includes a wrong bulk trailer → `redis:issue?`, not pooled). No new spec leaf. tcp-session truncated-bulk stays short-read `redis:unreachable`.
  By: explore
