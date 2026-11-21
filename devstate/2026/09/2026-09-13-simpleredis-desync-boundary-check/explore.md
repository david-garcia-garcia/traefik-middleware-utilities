# Explore
IssueKey: 2026-09-13-simpleredis-desync-boundary-check

## Concepts

```
  command N write
        │
        ▼
  readReply → one well-formed RESP value
        │
        ├─ leftover Buffered() > 0  → return that value, reusable=false, destroy
        └─ Buffered() == 0          → reusable=true, pool (compliant peer)
```

A **reply boundary** is the point after one complete RESP value where `conn.reader.Buffered() == 0`. Dest `do` treats any clean parse as that point. It is not: one extra unread value shifts the socket one reply ahead for every later command. `release` refreshes `lastUsed`, so traffic never ages the socket out.

`readReply` is already fail-closed on over-cap `$`/`*`, bad CRLF, nested array, error-in-array, unknown type byte, mid-array timeout, and partial write (`clean = false`). The hole is a *second well-formed value* still in the bufio buffer.

Identity (client address, user, tenant, Host, trust hop) is not set or reconstructed by this change. The wrong-key Get is a decode/pool bug, not a reconstructed tenant fact.

## Decisions

- Reproduce first. Throwaway `TestExploreDesyncedSocketKeepsServingPreviousReplies` against dest `do` (port of `origin/bugfixes20260913:simpleredis/bugs_repro_test.go` `startStrayExtraReplyFake`, nth=5, 12 Gets). **Failed:** `Get(k5) = "STRAY", want "v5" or an error (command 5)`. Same first mismatch as the reference (wire is `$5\r\nSTRAY\r\n`, not the prose `STRAYYY`). Deleted the throwaway; do not land it tagged `bugrepro`.
- Keep the asked gate inside `simpleredis/resp.go` `do`. After a successful `readReply`, `reusable` is true only when `conn.reader.Buffered() == 0`. Leftover: return the decoded slots (this command's reply was well-formed) and `reusable = false` so `release` closes the socket. Do not drain. Do not touch `pool.go` or `commands_exec.go` unless a later test proves AUTH leftover cannot be closed any other way.
- Handshake: `dial` calls `do` for AUTH then SELECT and ignores `reusable`. A compliant `+OK` leaves `Buffered() == 0`, so the post-read check is not a false positive. If AUTH leftover would make SELECT parse that leftover as its reply, add a **pre-write** `Buffered() != 0` refuse inside the same `do` (do not write; return dirty + `reusable = false`) so `dial` still closes on `err != nil` without editing `pool.go`. Prove with `TestAuthAndSelectOncePerDial`.
- `-LOADING ` / `-TRYAGAIN ` are complete `-` replies (`clean = true`, buffer empty). They stay reusable and retried. Do not treat leftover as those retries.
- Spec: one OpenSpec change. Fold leftover-bytes destroy into `std_go_simpleredis_tcp-session` **Dirty reply is not returned to the idle pool** (same owner as truncated bulk). Add a Get scenario on `std_go_simpleredis_resp-commands` so Get MUST NOT return another key's bytes after a stray extra. Do not create a new spec leaf.
- Tests (permanent, untagged, default suite): (1) port of `TestBugDesyncedSocketKeepsServingPreviousReplies` (own-key or error, never another key); (2) after the stray command, `pooledIdle == 0` and the next Get dials a new connection (accept count +1); (3) `TestConnectionIsReused` stays 25 Gets → 1 TCP connection (assert the count, not only pass). Helper `startStrayExtraReplyFake` lives in `fake_redis_test.go` next to the other fakes. Regression tests live in `resp_test.go` next to other dirty-reply proofs.
- Yaegi: `bufio.Reader.Buffered()` is stdlib. Existing `TestYaegi_NewGetSetDel` already covers the empty-buffer reuse path interpreted. Add an interpreted Get against the compiled stray fake if `writeGopathSimpleredis` + `evalClientprobe` can express it without new interpreter helpers; otherwise compiled tests are the gate.
- Consume: `knowledge/devdocs/std_go_simpleredis.md` and `std_go_simpleredis_resp-decode.md` do not mention leftover-bytes destroy. `knowledge/research/ext_redis_resp_bulk-string` already states Redis emits one RESP value per command except pipelining, Pub/Sub, MONITOR, and RESP3 Push. No new research folder. Usage update is propose/devdocsimpact, not a new packet name.
- Do not create or edit `simpleredis/BUGS.md`. Do not port bugs 1–2.

## Open questions

- Q: Which spec leaves get leftover-bytes destroy — `std_go_simpleredis_tcp-session` dirty-reply, `std_go_simpleredis_resp-commands` Get, or both?
  Rank: additive asked — new scenarios on existing spec folders this change updates; requirement Desired and Affected name those two leaves
  Decision: assumed — both: tcp-session dirty-reply owns “not pooled when the decoder cannot prove a boundary”; resp-commands Get adds the own-key / never-another-key scenario. One change, no new leaf.
  By: explore

- Q: Does a dedicated Yaegi test for `Buffered()` land, or only compiled tests?
  Rank: additive asked — optional new test this change would create; criterion is “add if practical”
  Decision: resolved — compiled tests plus `TestYaegi_StrayExtraReplyOwnKey` via existing `clientprobe.GetOwnKeys`
  By: implement

- Q: AUTH leftover (`dial` ignores `reusable`) — handshake-specific assertion, or later-command destroy only?
  Rank: additive asked — new branch inside `do` this change creates; requirement Tension says do not fail AUTH/SELECT solely because `reusable` is false, and prefer not to edit `pool.go`
  Decision: assumed — post-read leftover still returns the AUTH/SELECT value with `reusable = false` (dial ignores that flag). If `Buffered() != 0` *before* the next write on that same conn (SELECT after leftover AUTH), refuse the write inside `do` so SELECT is not parsed from AUTH leftover; `dial` then sees `err` and closes. Compliant handshake stays `Buffered() == 0` at both checks (`TestAuthAndSelectOncePerDial`).
  By: implement

- Q: Permanent test file name on dest (reference is build-tagged `bugs_repro_test.go`)?
  Rank: additive asked — new untagged tests this change creates; requirement Desired names the three proofs
  Decision: resolved — `simpleredis/resp_test.go` proofs plus `startStrayExtraReplyFake` in `simpleredis/fake_redis_test.go`
  By: implement
