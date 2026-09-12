# Explore
IssueKey: 2026-09-12-simpleredis-risk-03-readline-unbounded

## Concepts

- **RESP line**: one CRLF-terminated header (`+`, `-`, `:`, `$n`, `*n`). `readLine` in `simpleredis/resp.go` owns it. Bulk payload after `$n` is `readBulk` (out of scope).
- **ReadSlice view**: `bufio.Reader.ReadSlice('\n')` aliases the connection reader's 4096-byte buffer. Invalid after the next read. Packet: `knowledge/devdocs/std_go_simpleredis_resp-decode.md`.
- **ErrBufferFull remainder**: dest copies the partial then `ReadBytes('\n')` (`simpleredis/resp.go` `readLine`). `ReadBytes` grows until delimiter or the reader errors. go-redis uses the same shape at 32 KiB (`knowledge/research/ext_go-redis_proto_readline/`). This ticket diverges: full buffer is `redis:issue?`, not a grow.
- **Dirty socket**: `errIssue` from `readReply` → `do` returns `clean=false` → `release` closes, does not idle. `shouldRetry` does not retry `errIssue`.
- **Call sites of `readLine`**: `readReply` reply head and each array-element head (`simpleredis/resp.go`). Searched `simpleredis/**/*.go` for `readLine(` — those two. Tests and the resp-decode spec name the grow path.

```
peer bytes ──► bufio 4096 ──► ReadSlice('\n')
                      │
                      ├─ found \n, len>=2, has \r → strip CRLF
                      ├─ found \n, missing \r    → errIssue, dirty
                      └─ ErrBufferFull            → dest: ReadBytes grow
                                                   ticket: errIssue, no remainder read
```

## Decisions

- Honour Desired: on `ReadSlice` `ErrBufferFull`, return `errIssue` and do not `ReadBytes` the remainder. Do not add a 64 KiB `maxLineLength` after grow — that still allocates first.
- Keep dest CRLF strip (`len < 2` or missing `\r` → `errIssue`). Keep shortest legal lines (`+OK`, `:1`, `$-1`).
- Invert `TestLongStatusLineDecodes` (5000-byte `+` today must succeed) so over-cap is `redis:issue?` and idle 0. Add over-cap terminated and unterminated-stream proofs. Do not use `runtime.ReadMemStats`.
- Change `openspec/specs/std_go_simpleredis_resp-decode/spec.md` and `knowledge/devdocs/std_go_simpleredis_resp-decode.md` so the long-line scenario and the grow recipe match reject-on-full. Copy-on-escape for short `+`/`:` stays.
- Do not import go-redis. Do not enlarge `bufio.NewReader`. Do not retouch `readBulk` (bug-02).
- Existing research answers go-redis grow-on-full. No new research folder: the ceiling is this client's DoS bound, not Redis's documented max error size.

## Open questions

- Q: Cap at dest’s 4096 `ReadSlice` buffer versus the finding’s 64 KiB post-`ReadBytes` check?
  Rank: bounded asked — Desired names stop-on-full-buffer; 2 `readLine` call sites in `readReply` plus `TestLongStatusLineDecodes` and spec scenario “Line longer than 4096 bytes” (searched `simpleredis/**/*.go` for `readLine(`)
  Decision: assumed — reject on `ReadSlice` `ErrBufferFull` as `errIssue`; do not `ReadBytes` the remainder; invert the long-line test and that spec scenario.
  By: explore

- Q: Can any legitimate Redis `-ERR` this client must accept exceed 4096 bytes?
  Rank: additive asked — Desired “Legitimate headers are tiny” and stop-on-full-buffer; not a Redis-server reshape
  Decision: assumed — GET/MGET/SET/DEL/INCR/EXPIRE/EVAL/MSetEX heads and AUTH/SELECT/NOSCRIPT-class errors are well under 4096. A huge Lua or hostile `-ERR` is `redis:issue?` and the socket is discarded. Do not grow to 64 KiB to keep that payload. No new research folder.
  By: explore

- Q: How to pin allocation without a flaky heap sample?
  Rank: additive asked — Desired “unterminated multi-megabyte stream → errIssue and allocation stays near the bufio buffer”
  Decision: assumed — count bytes the fake peer delivered into the reader (≤4096). Do not use `runtime.ReadMemStats`. Over-cap terminated: `redis:issue?`, `pooledIdle == 0`. Keep existing shortest-line and missing-CR cases.
  By: explore
