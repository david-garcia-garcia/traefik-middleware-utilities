# Requirement
IssueKey: 2026-09-11-simpleredis-perf-03-idle-reaper

## Problem
SimpleRedis idle pooling is LIFO. `borrow` pops the tail and stops at the first socket still inside `idleTimeout`, so a cold head connection is never inspected while traffic recycles the newest socket. There is no reaper. Those head sockets stay open past idle, and under a concurrency spike they are borrowed as likely-dead landmines (failed attempt plus redial).

## Current (code)
- `simpleredis/simpleredis.go` `borrow` — pops `sr.idle` from the tail; `break`s on the first conn with `now.Sub(lastUsed) < idleTimeout`; older tail entries go to `stale` and are closed; a younger tail leaves older head entries in the list.
- `simpleredis/simpleredis.go` `release` — sets `lastUsed` and `append`s to the idle tail; does not inspect or drop the head.
- `simpleredis/simpleredis.go` `Close` — copies `sr.idle`, nils it, closes every remaining socket.
- `simpleredis/simpleredis.go` — `idleTimeout` is 30s, `maxIdleConns` is 8; no ticker or reaper goroutine.
- `simpleredis/simpleredis_test.go` `TestIdleTimeoutOpensANewConnection` — one idle entry, backdates that sole (tail) `lastUsed`, asserts a second dial. Does not cover a stale head behind a fresh tail.
- `simpleredis/` — no `live_test.go` (not found). Yaegi tests talk to the in-process fake only.
- `.github/workflows/ci.yml` — Redis and Dragonfly CI services exist; env is `WINDOWCOUNTER_LIVE_*` and `TOKENBUCKET_LIVE_*` only. No `SIMPLEREDIS_LIVE_*`.
- `e2e/simpleredisprobe/plugin.go` — one sequential request runs Set/Get/MGet/Del/Incr/IncrBy/Expire/ExpireAt/Eval. Eval already lists KEYS and is Lua 5.1-safe. No idle-head assertion.
- `scripts/integration-tests.Tests.ps1` — `GET /redis` and `GET /dragonfly` assert verb headers only.
- `docker-compose.yml` — `redis:7-alpine` and `dragonfly:v1.40.2`; neither service sets a server idle `timeout`.
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` — idle pool of eight; an idle conn older than thirty seconds SHALL not be reused. No scenario that the cold head is closed while a younger tail is being recycled.
- `knowledge/devdocs/std_go_simpleredis.md` — documents the idle cap of eight and that in-flight dials are uncapped; no idle-head reap.

## Desired
- Age out the cold idle head so a connection past `idleTimeout` cannot sit behind a recycled tail. Prefer **sweep-on-release**: in `release`, drop head entries older than `idleTimeout` before appending; keep it O(1)–O(2) (head only, not a full scan). Keep LIFO tail reuse. `Close` still drains the rest.
- Do not add a background reaper goroutine unless dest evidence requires it (it does not: no existing ticker in `simpleredis/`).
- Prove with the finding’s fake-server unit test: backdate `lastUsed` on a **head** entry while a fresh entry sits at the tail, run a command, assert the stale socket was closed and is not left in `sr.idle`.
- Prove idle-head reap / no landmine on **both** live Redis and Dragonfly (supported backends), not only the fake.
- Extend `docker-compose.yml`, Pester `/redis` `/dragonfly`, and `e2e/simpleredisprobe` if that is what the live proof needs. Any Eval used in proof MUST be Lua 5.1-safe and list touched keys in KEYS.

## Affected
- `simpleredis/simpleredis.go` (`release`, possibly the idle list around `borrow`)
- `simpleredis/simpleredis_test.go` (two-entry idle-head case)
- `simpleredis/` live tests / CI env if that is how both engines are proved (pattern exists on dest in `windowcounter/live_test.go` and `tokenbucket/live_test.go`)
- `openspec/specs/std_go_simpleredis_tcp-session/spec.md` (idle-pool scenario)
- `e2e/simpleredisprobe/`, `docker-compose.yml`, `scripts/integration-tests.Tests.ps1` if Pester is part of the proof
- `knowledge/devdocs/std_go_simpleredis.md` after apply (usage gotcha)

## Out of scope
- Background reaper ticker (finding’s other option; dest evidence and caller prefer sweep-on-release).
- perf-01 total/in-flight connection cap; perf-02 I/O timeout fan-out; changing `idleTimeout` or `maxIdleConns` values.
- EVALSHA, pipelining, go-redis, TLS, Unix sockets.
- Making Redis/Dragonfly server `timeout` a product setting.
- Other simpleredisfixes findings.

## Unknowns
- How to prove idle-head reap on live Redis and Dragonfly: dest SimpleRedis has no live Go tests; sibling packages prove engines with `*_LIVE_REDIS` / `*_LIVE_DRAGONFLY` and skip on `-short`. Pester cannot backdate `lastUsed`. Explore picks the seam (live Go test vs probe/Pester).
- Whether the landmine proof on live engines needs the server to close idle clients (`timeout`), or client-side `lastUsed` backdate plus a closed-socket assertion is enough. Compose currently does not set `timeout`.
- Whether compose/Pester/probe must change if live Go tests already cover both engines (conductor: extend them **if needed**).

## Tensions
- Finding accepts sweep-on-release **or** a background reaper. Caller and dest (no ticker; Traefik reload must not leak goroutines) prefer sweep-on-release.
- Finding’s proof recipe is fake-server only. Conductor requires the same idle-head / no-landmine proof on live Redis and Dragonfly as well. Both are the ask.
- tcp-session spec today only forbids **reuse** of a stale idle conn; it does not require closing the cold head while the tail stays hot. Closing the head is the finding; a spec scenario is propose work, not a new product ask.
