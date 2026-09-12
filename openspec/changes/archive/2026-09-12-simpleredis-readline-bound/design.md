## Context

Dest `readLine` (`simpleredis/resp.go`) uses `ReadSlice('\n')` then, on `bufio.ErrBufferFull`, copies the partial and `ReadBytes` the remainder. `dial` stays `bufio.NewReader(netConn)` (4096). `errIssue` from `readReply` is not pooled and not retried. See proposal.md for why. Research: `knowledge/research/ext_go-redis_proto_readline/`. Proceed policies: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Bound header allocation at the existing 4096 `ReadSlice` buffer.
- Keep dest CRLF strip, copy-on-escape for short `+`/`:` , and `parseLen`.
- Compiled over-cap and unterminated proofs that fail if the decoder grew past 4096.

**Non-Goals:**
- A 64 KiB post-`ReadBytes` cap (still grows first).
- Enlarging `bufio.NewReader`, importing go-redis, or changing `IOTimeout`.
- `readBulk` announced-length `make` (bug-02).
- Changing live Pester verbs or compose.

## Decisions

1. **Reject on `ErrBufferFull`, do not `ReadBytes`.** Return `errIssue` immediately. Alternative: copy partial + `ReadBytes` then `maxLineLength` 64 KiB — rejected; Desired names stop-on-full-buffer because the weaker cap still allocates first.

2. **Stay on dest 4096.** Do not raise the reader to go-redis 32 KiB to keep long `-ERR` payloads. A huge Lua or hostile error is `redis:issue?` and the socket is discarded. Alternative: 64 KiB grow-then-check — rejected (explore).

3. **Pin the bound by counting peer bytes, not heap samples.** Call `readLine` from a package test with `bufio.NewReader` over a counting `io.Reader` that never sends `\n` (and a second case that sends a terminated line longer than 4096). Assert `err == errIssue` and bytes read ≤ 4096. Invert `TestLongStatusLineDecodes` through `Eval` / `startStaticRedis` so the 5000-byte `+` is `redis:issue?` and idle 0. Alternative: `runtime.ReadMemStats` — rejected; flaky.

4. **Leave unread remainder on the socket.** `errIssue` already closes the conn on release. Do not drain the rest of an over-cap line.

## Risks / Trade-offs

- [A Redis `-ERR` longer than 4096 becomes `redis:issue?`] → Mitigation: this client’s AUTH/SELECT/NOSCRIPT-class errors are short; a huge payload is treated as a dirty peer.
- [Unread over-cap bytes left in bufio] → Mitigation: dirty socket is closed, not pooled.
- [Short unterminated reply (EOF before 4096)] → Mitigation: still the existing truncated/`io.EOF` path (`redis:unreachable` when `MaxRetries` is off); only buffer-full is `errIssue`.

## Migration Plan

Library-internal decode. Rollback is revert. No exported API change.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
