# Explore
IssueKey: 2026-09-12-simpleredis-risk-05-freeinuseturn-blocks-when-full

## Concepts

**In-use turn**: one slot on `inUseTurns`, a `PoolSize`-buffered `chan struct{}` pre-filled at `New` (`ensureInUseTurns`). `borrow` receives one turn before idle-pop or dial. Idle sockets do not hold a turn. `freeInUseTurn` sends one token back.

**Over-free**: a send when the channel is already at capacity. Dest `freeInUseTurn` (`simpleredis/pool.go:54-59`) has no `default`, so that send parks the goroutine forever.

**OverFrees**: an `atomic.Int64` on `SimpleRedis`, same Yaegi-safe field pattern as `closed atomic.Bool`. Exported as `OverFrees() int64` next to `PoolSize()` / `MaxIdleConns()`. Correct accounting stays at 0.

Units:

- `simpleredis/pool.go` — `freeInUseTurn`, `borrow`, `release`
- `simpleredis/simpleredis.go` — `closed`, `inUseTurns`; no `overFrees` yet
- `simpleredis/commands_exec.go` — `exec` borrows then `release`s; borrow error does not release
- `simpleredis/pool_test.go` — pool reuse, cap, auth, idle, wait; no over-free or `len==cap` + counter invariant
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — live cap / wait / timeout; no over-free
- `knowledge/devdocs/std_go_simpleredis.md` — live cap gotcha; no over-free hang
- `knowledge/research/ext_go-redis_connection-pool/` — go-redis `FastSemaphore` is the same buffered-channel shape; Release is a send with no `default` (we copy the cap/wait shape, not the hang)

```
  exec
    │
    ▼
  borrow ── receive turn ── idle/dial ── do ── release ── freeInUseTurn (send)
                 │                                    │
                 closed/dial fail ── freeInUseTurn     always freeInUseTurn
                                                      │
                                      dest: send with no default
                                      over-free ── goroutine parks forever
                                      wanted: select default ── OverFrees++
```

Reproduced 2026-09-12: `TestThrowawayFreeInUseTurnHang` on a fresh `PoolSize: 2` client (`len==cap==2`) then `freeInUseTurn` in a goroutine. After 200ms the send had not returned. Throwaway deleted; not committed.

## Decisions

- Follow the how-to-fix: `select` + `default` + `overFrees.Add(1)`. Do not panic. Do not replace the channel with a mutex counter (out of scope; that is also bug-03's `len(inUseTurns)` read).
- Dest borrow/release pairing is balanced today (`exec` does not release on borrow error; `borrow` frees on closed-client and dial failure). This change does not retune those paths. It only changes how a future extra `freeInUseTurn` manifests.
- Guard and invariant tests live in `simpleredis/pool_test.go` (same-package owner). Guard calls `freeInUseTurn` on a fresh client, must return promptly, `OverFrees()==1`. Invariant hammers healthy fake, dead address, AUTH reject, and starved pool with a short `PoolTimeout` from many goroutines, then `len(inUseTurns)==cap(inUseTurns)` and `OverFrees()==0`.
- Spec delta on `std_go_simpleredis_tcp-session`: over-free MUST NOT block; `OverFrees()` increments. Tests are the scenarios; the spec names the contract.
- Usage packet `std_go_simpleredis.md` gotcha: dest used to hang on over-free; now it drops and counts. Do not document `OverFrees` on the Traefik probe.
- Sibling findings in `simpleredisfixes2/` stay out.

## Open questions

- Q: Does the tcp-session spec name the non-blocking over-free path, or do only the tests prove it?
  Rank: additive asked — Desired and Affected name `openspec/specs/std_go_simpleredis_tcp-session/spec.md` (over-free must not hang; counter)
  Decision: assumed — the spec SHALL require that returning a turn when `inUseTurns` is already full MUST NOT block and MUST increment `OverFrees()`. Guard and invariant scenarios prove it.
  By: explore

- Q: Is `OverFrees()` compiled-test only, or also observed from `e2e/simpleredisprobe`?
  Rank: additive asked — Desired says a test or an operator can read the method; probe currently has no `PoolSize`/`MaxIdleConns`/`OverFrees` call sites (roots: `e2e/simpleredisprobe`, `simpleredis`)
  Decision: assumed — compiled tests only. Do not add probe wiring or Pester headers. `OverFrees()` stays an exported accessor for tests and any future operator who already holds a `*SimpleRedis`.
  By: explore

- Q: Does this run put `-race` into CI?
  Rank: additive incidental — Out of scope names ci-01 race detector in CI; Desired still wants the invariant run under `-race`
  Decision: assumed — do not change GitHub workflows. Dest already has Unit race; this merge's CI Unit race succeeded. Local `go test -race` not-run (CGO gcc not found). `go test -short ./simpleredis/...` passed including the invariant.
  By: pullrequest
