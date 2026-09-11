# Explore
IssueKey: 2026-09-11-simpleredis-perf-03-idle-reaper

## Concepts

- **Idle list is LIFO.** `release` `append`s; `borrow` pops `sr.idle[len-1]`. Head (`idle[0]`) is the oldest unused socket; tail is the hottest.
- **Tail-only stale walk.** `borrow` (`simpleredis/simpleredis.go` 215–223) pops from the tail and `break`s on the first conn with `now.Sub(lastUsed) < idleTimeout`. Older tail entries go to `stale` and are closed after unlock. A younger tail leaves older **head** entries in the list. There is no ticker in `simpleredis/simpleredis.go` (`time.Ticker` / `time.Tick` not found).
- **Landmine.** After a period of sequential recycle, idle looks like `[stale, …, fresh-tail]`. One borrow takes the tail and stops. A concurrency spike then pops the cold head: failed attempt plus redial (`exec` retry when `reused && !reusable`).
- **Sweep-on-release.** In `release`, before append, peel idle-head entries whose `lastUsed` is older than `idleTimeout` (stop at the first still-valid head). Close those sockets after unlock (same pattern as `borrow`’s `stale`). Keep LIFO tail reuse. `Close` still copies `sr.idle`, nils it, and closes every remaining socket. No background goroutine.
- **Owners.** `lastUsed` and `idle` are unexported on `pooledConn` / `SimpleRedis`. Same-package tests already backdate `lastUsed` (`TestIdleTimeoutOpensANewConnection`). Live proof belongs in `simpleredis/` so it can see those fields. Pester / `e2e/simpleredisprobe` cannot backdate `lastUsed` and do not inspect `idle`.
- **Sibling live seam.** `windowcounter/live_test.go` and `tokenbucket/live_test.go` table-drive `*_LIVE_REDIS` / `*_LIVE_DRAGONFLY`, skip on `-short` or empty, CI services `127.0.0.1:6379` / `127.0.0.1:6380`. SimpleRedis has no `live_test.go` and CI has no `SIMPLEREDIS_LIVE_*`.
- **Usage / spec gaps (later).** `knowledge/devdocs/std_go_simpleredis.md` documents the idle cap of eight and uncapped in-flight dials; no idle-head reap. `openspec/specs/std_go_simpleredis_tcp-session/spec.md` forbids **reuse** of a conn older than thirty seconds; it does not require closing a cold head while a younger tail is recycled. Spec scenario is propose work. Usage gotcha is after apply. No new Language term (idle head is the slice index, not a catalog noun).
- **Identity.** This change does not set or reconstruct client address, user, tenant, Host, or trust hop.

Measured (worktree `2026-09-11-simpleredis-perf-03-idle-reaper`): `go test ./simpleredis/ -run TestIdleTimeoutOpensANewConnection|TestConcurrentCommandsStayWithinPool` PASS. The idle test backdates the sole (tail) entry and asserts a second dial. Two-entry head-behind-fresh-tail is not in the suite (`simpleredis_test.go` only `lastUsed` write is line 623, `idle[0]` of a one-element list). Gap confirmed by `borrow` 218–221, not by a red test.

```
  idle:  [ stale-head | … | fresh-tail ]
                ▲               ▲
                │               └── borrow pops, age OK, break
                └── never inspected; sits past idleTimeout

  sequential Get ──► borrow(tail) ──► do ──► release(append)
                                              └── WANT: peel stale prefix at [0]
```

## Decisions

- Sweep-on-release in `release`; do not add a background reaper goroutine. Dest has no ticker in `simpleredis/`. `windowcounter/`’s flush ticker is a different job (local delta flush), not a pool reaper. Traefik reload must not leak a goroutine this package does not already own. Caller constraint recorded here.
- Keep `idle []*pooledConn`. Peel from index 0 while the head is older than `idleTimeout`; stop at the first still-valid entry (or empty). That is head-only, not a scan of the hot tail. Cap is eight, so the stale prefix is bounded. Do not switch to `container/list`.
- Do not change `borrow`’s LIFO `break`. All-stale idle is already handled there (walk until empty, then dial). `Close` still drains whatever remains.
- On every reusable `release` path, peel stale heads **before** the append-or-close decision — including when the returning conn is then closed because `closed` or `len(idle) >= maxIdleConns`. Collect peeled conns under the lock; `close()` after unlock.
- Residual: a spike that borrows the stale prefix **before** any post-timeout `release` can still pop a cold head. Sequential traffic after timeout peels on the next release. Accepted tradeoff versus a reaper (Out of scope).
- Proof: (1) fake-server unit test — two idle entries, backdate **head** `lastUsed`, run a command, assert the stale socket was closed and is not left in `sr.idle`. (2) `simpleredis/live_test.go` — same two-entry recipe against live Redis and Dragonfly; assert no landmine (the recycled command succeeds; a follow-up borrow does not use the closed head). Same package so `lastUsed` / `idle` stay unexported. No `Reset`/`Dump` production identifier.
- Live env: `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY`. Reuse CI services. Skip on `testing.Short` or empty. CI `test` job sets both and must not skip. README Tests paragraph gains those names beside the sibling vars. Optional `waitLiveClient` via existing `Get`/`Set`/`Incr` (no Ping command).
- Do not extend `docker-compose.yml`, Pester `/redis` `/dragonfly`, or `e2e/simpleredisprobe` for this proof. They cannot backdate `lastUsed`. Compose already runs both engines without a server idle `timeout`. Traefik e2e stays Yaegi verb smoke.
- Do not set Redis/Dragonfly server `timeout` in compose or CI. Landmine proof is client-side: stale head closed and gone from `idle`, not `CLIENT LIST` / engine-dropped clients. Making server `timeout` a product setting is Out of scope.
- Idle-head tests use `Get` (or another existing verb). No Eval in this proof. Lua 5.1-safe + Dragonfly KEYS remains a constraint **if** a later proof adds Eval; current probe Eval already lists KEYS.
- Propose: add a tcp-session scenario that the cold head is closed while a younger tail is recycled. Do not retitle the existing “shall not be reused” requirement into a reaper. Values of `idleTimeout` (30s) and `maxIdleConns` (8) stay.
- After apply: usage gotcha on `knowledge/devdocs/std_go_simpleredis.md` that `release` peels stale heads. Explore did not write that packet (behavior not landed).

## Open questions

- Q: Sweep-on-release in `release`, or a background reaper goroutine?
  Rank: bounded asked — `release` has 2 call sites in `exec` (`simpleredis/simpleredis.go` 188 and 199; searched `simpleredis/` for `sr.release(`); Desired “Prefer sweep-on-release”; Out of scope “Background reaper ticker”
  Decision: resolved — sweep-on-release; no ticker, no goroutine. Dest `simpleredis/simpleredis.go` has no `time.Ticker`.
  By: explore

- Q: How to prove idle-head reap / no landmine on live Redis and Dragonfly as well as the fake-server unit test?
  Rank: additive asked — new `live_test.go` this change creates; Desired “Prove idle-head reap / no landmine on both live Redis and Dragonfly”; Unknowns names the seam
  Decision: assumed — same-package `simpleredis/live_test.go` table-driven on both engines; two idle conns (concurrent commands then release), backdate head `lastUsed`, run a command, assert stale socket closed and not in `sr.idle`, command succeeds. Fake-server unit test keeps the same two-entry recipe. Skip on `-short` or missing addrs. CI sets both addrs. Yaegi does not backdate unexported fields; do not add a test-only export.
  By: explore

- Q: Does the live landmine proof need the server to close idle clients (`timeout`), or is client-side `lastUsed` backdate plus a closed-socket assertion enough?
  Rank: additive asked — Unknowns names timeout vs lastUsed; Out of scope “Making Redis/Dragonfly server timeout a product setting”
  Decision: assumed — client-side `lastUsed` backdate plus “closed and not in `sr.idle`” is enough on both engines. Do not set compose/CI server `timeout`. Do not add `CLIENT LIST` to the client.
  By: explore

- Q: Must compose, Pester `/redis` `/dragonfly`, and `e2e/simpleredisprobe` change if live Go tests already cover both engines?
  Rank: additive asked — Desired “Extend docker-compose.yml, Pester, and e2e/simpleredisprobe if that is what the live proof needs”; Unknowns names “if needed”
  Decision: assumed — not needed. Live Go tests are the proof. Leave compose, Pester headers, and the probe as Yaegi verb smoke. Do not wait 30s in Pester.
  By: explore

- Q: Live env var names, and does CI reuse the existing Redis/Dragonfly pair?
  Rank: additive asked — Desired live both engines; sibling packages already use per-package `*_LIVE_*` on the same CI services
  Decision: assumed — `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY`. Reuse CI `127.0.0.1:6379` and `127.0.0.1:6380`. Skip without addrs or under `-short`. CI `test` job sets both. Do not reuse `WINDOWCOUNTER_LIVE_*` / `TOKENBUCKET_LIVE_*` (packages skip independently).
  By: explore

- Q: Peel the whole stale prefix from the head, or only one or two head entries per `release` (letter of “O(1)–O(2)”)?
  Rank: additive asked — Desired “a connection past idleTimeout cannot sit behind a recycled tail” and “drop head entries older than idleTimeout”; “O(1)–O(2) (head only, not a full scan)” is the bound on how far to look
  Decision: assumed — while `idle[0]` is older than `idleTimeout`, close and drop it; stop at the first still-valid head. Do not inspect past that into the hot tail. LIFO means stale entries are the prefix; leaving a stale middle would violate Desired.
  By: explore

- Q: Who already owns idle-socket age / pool membership that a live or Pester proof would read?
  Rank: additive asked — Desired live proof; One job, one owner for lastUsed/idle
  Decision: resolved — `SimpleRedis` owns `idle` and `lastUsed`. Reuse those fields from same-package tests. Do not reconstruct age from Redis `timeout`, `CLIENT LIST`, or a probe header.
  By: explore
