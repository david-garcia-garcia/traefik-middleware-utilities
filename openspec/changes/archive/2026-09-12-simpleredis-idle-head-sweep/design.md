## Context

Dest `simpleredis/simpleredis.go` `borrow` pops `sr.idle` from the tail and `break`s on the first conn with `now.Sub(lastUsed) < idleTimeout`. Older tail entries go to `stale` and close after unlock. A younger tail leaves older head entries in the slice. There is no `time.Ticker` in this package. `release` sets `lastUsed` and `append`s. Same-package tests already backdate `lastUsed` (`TestIdleTimeoutOpensANewConnection`); that case is a one-element list (the tail). Sibling live seam: `windowcounter/live_test.go` / `tokenbucket/live_test.go` with per-package `*_LIVE_*` on CI `127.0.0.1:6379` / `127.0.0.1:6380`. See proposal.md for why. Proceed policies: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Head-only stale prefix peel in `release` on every reusable path, O(prefix) not a tail scan.
- Fake two-entry unit plus same-package live Redis and Dragonfly.
- CI env vars that skip independently of windowcounter/tokenbucket.

**Non-Goals:**
- Background reaper, `container/list`, changing `borrow`’s LIFO `break`.
- Compose / Pester / `e2e/simpleredisprobe` / server `timeout` / `CLIENT LIST`.
- Yaegi idle-head proof (cannot backdate unexported fields; no test-only export).
- Eval in this proof. perf-01 cap, perf-02 I/O timeout, EVALSHA, pipelining.

## Decisions

1. **Sweep-on-release, not a ticker.** Peel in `release` before append-or-close. Alternative: background reaper — rejected; dest has no ticker here; Traefik reload must not leak a goroutine this package does not own. Residual: a spike that borrows the stale prefix before any post-timeout `release` can still pop a cold head.

2. **Keep `idle []*pooledConn`.** While `idle[0]` is older than `idleTimeout`, drop it into a peeled slice; stop at the first still-valid head or empty. Cap is eight, so the stale prefix is bounded. Alternative: `container/list` — rejected; consume before produce.

3. **Peel under the lock; `close()` after unlock.** Same pattern as `borrow`’s `stale`. Peel even when `sr.closed` or `len(idle) >= maxIdleConns` then closes the returning conn. Non-reusable `release` still `conn.close()` and returns without touching idle.

4. **Do not change `borrow`.** All-stale idle already walks until empty then dials. `Close` still copies `sr.idle`, nils it, and closes every remaining socket.

5. **Fake-server two-entry unit in `simpleredis_test.go`.** Two concurrent Gets then release so idle has two sockets; backdate **head** `lastUsed`; Get; keep a pointer to the head `pooledConn`; assert it is not in `idle` and its socket is closed. Existing one-element idle-timeout test stays.

6. **`simpleredis/live_test.go` same recipe.** Table-drive `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY`. Skip on `testing.Short` or empty. Optional `waitLiveClient` via `Get`/`Set`/`Incr` (no Ping). CI `test` job sets both addrs and must not pass `-short`. README Tests paragraph names those vars beside the sibling ones. Alternative: Traefik Pester — rejected; Pester cannot backdate `lastUsed`.

7. **Client-side age, not engine `timeout`.** Landmine proof is “closed and gone from `idle`”, not `CLIENT LIST`. Do not set Redis/Dragonfly server `timeout` in compose or CI.

8. **Get, not Eval.** Idle-head tests use `Get`. Lua 5.1-safe + Dragonfly KEYS stays a constraint if a later proof adds Eval; current probe Eval already lists KEYS.

## Risks / Trade-offs

- [Spike borrows stale prefix before a post-timeout release] → Mitigation: accepted vs a reaper; sequential traffic after timeout peels on the next release.
- [Live tests skip in CI] → Mitigation: set both `SIMPLEREDIS_LIVE_*` on the existing `test` job services; do not reuse `WINDOWCOUNTER_LIVE_*` / `TOKENBUCKET_LIVE_*`.
- [Yaegi cannot see `lastUsed`/`idle`] → Mitigation: compiled same-package tests only; do not add a production `Reset`/`Dump`.
- [Two concurrent Gets may not leave two idle] → Mitigation: wait until `len(idle)==2` after the burst; fail the test if not.

## Migration Plan

Library-internal pool behavior. Rollback is revert. No public API change. Callers keep using Get/Set as today.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
