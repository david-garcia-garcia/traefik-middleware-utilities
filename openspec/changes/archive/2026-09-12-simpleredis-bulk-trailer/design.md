## Context

Dest `readBulk` allocates `length+2`, `io.ReadFull`, and returns `data[:length]` with no trailer compare (`simpleredis/resp.go:124-128`). `readLine` already rejects a missing CR before LF. Truncated bulk is already `redis:unreachable` via short `ReadFull`. `startRawReplyRedis` indexes by Accept; `closeAfter` still hits `defer conn.Close()` on both branches. One caller: `TestTruncatedBulkIsUnreachableAndNotPooled` (`closeAfter: true`). Research: `knowledge/research/ext_redis_resp_bulk-string/`. Proceed policies: `devstate/explore.md`. See proposal.md for why.

## Goals / Non-Goals

**Goals:**
- Trailer CRLF check after a complete `ReadFull` in `readBulk`.
- Unit cases for `\n\r`, payload-as-trailer, and `$0\r\n\r\n`.
- Desync on `startRawReplyRedis` with `PoolSize: 1` proving idle empty and a second Accept’s own value.

**Non-Goals:**
- bug-02 length cap before `make`.
- A second scripted-server helper.
- Exporting the Accept counter.
- Changing `readLine`, RESP3, nested arrays, or `*-1`.
- Widening the tcp-session truncated-bulk requirement (short `ReadFull` stays `redis:unreachable`).

## Decisions

1. **Fix the cause in `readBulk`.** After a complete `io.ReadFull`, if `data[length] != '\r'` or `data[length+1] != '\n'`, return `errIssue`. `readReply` already maps that to `clean == false`. Alternative: special-case in `readReply` — rejected; Fix the cause.

2. **Keep `make([]byte, length+2)` plus `io.ReadFull`.** Do not shrink to `length` plus a two-byte discard (out of scope / prior decode non-goal). Alternative: `io.ReadFull` of length then two more reads — extra syscalls for the same bytes.

3. **Unit tests call `readBulk` like `TestReadBulkNonDollarHeadIsIssue`.** Trailer `\n\r`, two payload bytes as trailer, `$0` with CRLF. Alternative: only Get fakes — rejected; Desired names the `readBulk` cases.

4. **Desync reuses `startRawReplyRedis`; infer two Accepts.** First payload `$5\r\nhello+OK\r\n` (`closeAfter: false`). Second payload a complete bulk (`closeAfter: true`). `MaxRetries: -1`, `PoolSize: 1`. Idle empty after first Get; second Get returns the second payload. Alternative: export Accept count — rejected; truncated-bulk already infers from the second payload.

5. **`closeAfter: false` waits for the client to disconnect.** `io.Copy` discard then Close (defer still runs). Leave `closeAfter: true` as write-then-return. Alternative: leave both paths closing — rejected; remnant bytes in the pooled reader are never observed if the second write fails on a dead peer.

6. **Fold specs; update usage.** MODIFIED `std_go_simpleredis_resp-decode` and `std_go_simpleredis_resp-commands`. Add a trailer sentence to `knowledge/devdocs/std_go_simpleredis_resp-decode.md`. Alternative: new spec leaf — rejected; FindSpecHost fold.

## Risks / Trade-offs

- [Second Get can pass while a dirty conn was pooled if retries redial] → Mitigation: `MaxRetries: -1` and idle empty after the failed Get (decision 4).
- [Truncated tests break if `closeAfter: true` changes] → Mitigation: only the false path waits (decision 5). 1 existing caller uses true.
- [Length cap still missing] → Mitigation: out of scope (bug-02); do not take it here.

## Migration Plan

Library-internal decode plus same-package tests. Rollback is revert. No exported API change. No compose or probe edit.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
