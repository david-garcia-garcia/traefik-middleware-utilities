# Explore
IssueKey: 2026-09-13-simpleredis-panic-safe-release

## Concepts

```
  dest exec loop
  ─────────────────
  borrow ──► do ──► release
               │
               └─ panic ──► Traefik recover
                             release never runs
                             turn stays taken
                             socket fd stays open
                             inUseTurns never refilled
```

`inUseTurns` is a buffered channel sized once in `New`. `freeInUseTurn` only sends; nothing creates a replacement token. After `PoolSize` recovered panics the channel is empty, idle is empty, and every later command is `redis:unreachable` against a healthy Redis.

PR #29 left `release` non-deferred on the belief that a deferred restore would not run under Yaegi. Closed PRs #66 / #70 then recovered by refill (`heldSockets`, `LostTurns`, `borrowAfterPoolWait`) without closing the panicked fd. That is the opposite of this ticket: prevent the leak; do not detect it.

`origin/proto-defer-net` already has the keep-the-design fix. It is based on an older master: its three-dot diff against current `origin/master` also deletes `simpleredis/BUGS.md` and `chaos_pool_test.go` from #68. Do not merge that branch. Copy the two product edits and the two tests onto dest, then rename the tests.

## Decisions

- Keep the prototype design. `exec` calls new `runOnConn`. `runOnConn` declares `reusable := false` before `defer func() { sr.release(conn, reusable) }()`, then `values, reusable, err = sr.do(...)` (plain `=`, named returns). Panic or `do` saying not reusable both destroy the socket.
- `borrow` takes a `handedOff` flag and defers `freeInUseTurn()` when the socket is not handed to the caller. Remove the two explicit `freeInUseTurn()` calls on the closed-idle and dial-error paths; keeping both double-frees and inflates `OverFrees`.
- Do not add `heldSockets`, `lostTurns`, `turnRecoverMu`, `LostTurns()`, `recoverLostTurnsLocked`, or `borrowAfterPoolWait`. Closed #66 recovered turns without closing the fd; this change makes both leaks impossible.
- Do not create the #66 debt files. Do not edit `simpleredis/BUGS.md` (#68 owns it).
- Do not merge, cherry-pick, or pre-empt OPEN #67 / #68 / #69. #67 will later conflict mechanically on `borrow` (`handshakeFailed` bool). Apply `handedOff` on dest's current `borrow`; do not take #67's handshake shape.
- Prototype tests: `simpleredis/panic_safety_test.go` (`TestPanicInDoReturnsTurnAndClosesSocket`) and `simpleredis/yaegi_defer_test.go` (`TestYaegi_DeferRunsOnPanic`). Drop `zz_`, `proto`, `scratch`. Keep the Yaegi probe as a permanent test — it is the evidence that overturns #29. Its comment names the three panic classes and why the file exists.
- Panic injection stays a `bufio.Writer` over a panicking `io.Writer` so the panic is inside `do` while `conn.netConn` is a healthy socket `release` can close.
- Update the stale `do` comment in `resp.go`. Add the panic-returns-turn guarantee on `openspec/specs/std_go_simpleredis_tcp-session/spec.md`. Add a Yaegi-defer-runs gotcha on `knowledge/devdocs/std_go_simpleredis.md`.
- Dest already has `knowledge/devdocs/std_go_simpleredis.md` and research `ext_traefik_plugins_yaegi-afterfunc/` plus `ext_traefik_plugins_yaegi-generics/.sources/traefik-v3.7.11-go.mod.md`. No new research folder. Usage produce is the gotcha, not a new packet.

Measured dest (throwaway `TestExploreDestPanicInDoLosesTurn`, deleted after, `go test -v -count=1 -run TestExploreDestPanicInDoLosesTurn ./simpleredis/`):

```
after 2 recovered panics: turns=0/2 idle=0 OverFrees=0 accepts=2
Get after panics: err=redis:unreachable
```

Requester measured the prototype: `turns=2/2 idle=0 accepts=3 OverFrees=0` after two recovered panics; `go test -count=1 -timeout 300s ./simpleredis/` green in 11.89s with no existing-test edits.

## Open questions

- Q: Which Yaegi version does the Traefik this plugin targets actually ship, versus the v0.16.1 probe?
  Rank: additive asked — requirement Unknowns line on dest pin vs Traefik image; no existing caller reshape
  Decision: resolved — this module requires `github.com/traefik/yaegi v0.16.1` (`go.mod`). Compose image is `traefik:v3.7.11`. Official Traefik v3.7.11 `go.mod` directly requires `github.com/traefik/yaegi v0.16.1` (`knowledge/research/ext_traefik_plugins_yaegi-generics/.sources/traefik-v3.7.11-go.mod.md`, fetched 2026-09-11). Same pin as the probe. No assumed fallback; no detection machinery.
  By: explore
