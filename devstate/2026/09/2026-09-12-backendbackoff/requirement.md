# Requirement
IssueKey: 2026-09-12-backendbackoff

## Problem
A Traefik middleware that fronts a backend has no reusable admission gate that stops forwarding into an unhealthy upstream and then backs off exponentially while it recovers. Each middleware reinvents an ad-hoc retry or forwards every request. Traefik's built-in CircuitBreaker is not a library a plugin can import.

## Current (code)
- `backendbackoff/` — not found on `origin/master` (`beede1dc6c6d314283ead9319f8b54a63f3a5518`)
- Traefik `CircuitBreaker` as an importable library — not found in this tree
- In-memory bounded map with TTL eviction, cap, `dropExpired`, `dropOne` — `tokenbucket/memory.go` (`maxMemorySources`, `Memory.Allow`)
- `tokenbucket.Memory.Allow` — consume one refill token now and return wait (`tokenbucket/memory.go`); `consumeOne` is time-refill arithmetic (`tokenbucket/clock.go`)
- `tokenbucket.Redis.Allow` — returns `r.redis.Eval`'s error (`tokenbucket/redis.go`)
- `windowcounter.takeExact` — returns `l.redis.Incr`'s error (`windowcounter/limiter.go`)
- `windowcounter.windowLocked` — GET-seeds `redis_known` on first sight and whenever `localDelta == 0` (`windowcounter/limiter.go`); a Redis error there propagates
- `reclaim.Open` / `Table` — stores `any` per key with optional `Close` hook; `Sleep`/`Wake` are for parked goroutines (`reclaim/table.go`, `knowledge/devdocs/std_go_reclaim.md`)
- Yaegi unit convention — `tokenbucket/limiter_yaegi_test.go`, `windowcounter/limiter_yaegi_test.go`
- Allocation-ceiling convention — `TestAlloc*` in `simpleredis/bench_test.go`; unit CI `-short` runs them (`knowledge/devdocs/std_go_test-suites.md`)
- Go E2E Redis/Dragonfly jobs — `.github/workflows/ci.yml` jobs `e2e-redis` / `e2e-dragonfly` set `SIMPLEREDIS_LIVE_*`, `WINDOWCOUNTER_LIVE_*`, `TOKENBUCKET_LIVE_*` only
- Reclaim has no `*_LIVE_*` and no Go E2E files (`knowledge/devdocs/std_go_test-suites.md`)
- README presents the module as one Redis-backed stack (window counter and token bucket on SimpleRedis) — `README.md` `## Why this exists`, `## Layout`
- Spec family `std_go_backendbackoff` — not found (`openspec/specs/domains.md`, `openspec/specs/map.md`)
- Usage packet for a backend backoff gate — not found (`knowledge/devdocs/index_std_go.md`)
- Library usage docs rule out HTTP/client-address classification inside the library — `knowledge/devdocs/std_go_windowcounter.md`, `knowledge/devdocs/std_go_tokenbucket.md`
- SimpleRedis dial-failure memory (different job) — `knowledge/debt/2026-09-12-simpleredis-dial-circuit-breaker.md`

## Desired
New Yaegi-safe package `backendbackoff/`: in-memory per-key admission gate. Caller reports a boolean outcome of a **real backend attempt only**. The library does not read HTTP, classify failures, sleep, or write a response.

Settled (do not re-open):
- Trip criterion is a saturating success-credit bucket (one `float64` per key, start/cap `B`; failure `credit -= 1`; success `credit += p/(1-p)` capped at `B`; trip when `credit <= 0`). Not a windowed ratio. Not an absolute failure rate.
- Two independent clocks: credit decides whether to trip (no time); exponential cooldown decides when to probe (no outcomes). States `CLOSED` / `OPEN` / `HALF-OPEN`. Probe success: close, credit → `B`, retain backoff exponent `n`. Probe failure: re-open with `n+1` regardless of credit. `n` resets to 0 after one capped (maximum) cooldown of continuous `CLOSED`. Cooldowns jittered.
- `Report` only for real backend attempts. Denied requests MUST NOT be reported. Probe outcomes use the same `Report` path.
- Storage: bounded map key → state, idle TTL eviction, `maxMemorySources`-style cap, `dropExpired` / `dropOne` **shape** of `tokenbucket/memory.go`. Do **not** import `tokenbucket`. Mutex-guarded. No I/O on the admission path. No Redis, no shared state, no background goroutine. `Close` at most (no `Sleep`/`Wake` required). Gate SHOULD be storable in an existing `reclaim` table (use `reclaim.Open`; do not change `reclaim/`).
- Prove with `go test -short` plus a Yaegi test matching the per-package convention, plus a `TestAlloc*` ceiling on the admission path. No live-engine E2E and no `*_LIVE_*` env: this package must not appear in `e2e-redis` or `e2e-dragonfly`.
- `README.md`: new section plus `## Layout`. One line in `## Why this exists` so the Yaegi/shared-consumers justification still holds without claiming every package is Redis-backed.
- New `std_go_backendbackoff` spec family and a usage packet plus `index_std_go.md` entry (propose/devdocs phases).

## Affected
- New `backendbackoff/`
- `README.md` (`## Why this exists`, `## Layout`)
- `openspec/specs/domains.md`, `openspec/specs/map.md`, new `std_go_backendbackoff` specs (after propose)
- `knowledge/devdocs/` usage packet + `index_std_go.md`

## Out of scope
- Distributed / Redis / `simpleredis` layer (deferred; later phases write the follow-up note)
- Windowed-ratio or absolute-rate trip designs (rejected)
- Importing or reshaping `tokenbucket` / `windowcounter` / `simpleredis` / `reclaim`
- HTTP handling, failure classification, executing or scheduling retries
- Live Redis/Dragonfly E2E, `*_LIVE_*` env, Pester plugin unless explore proves it is required for Yaegi
- SimpleRedis dial circuit breaker (`knowledge/debt/2026-09-12-simpleredis-dial-circuit-breaker.md`) — different owner

## Unknowns
- Default values for `p`, `B`, base cooldown, maximum cooldown, jitter fraction, and idle TTL. Pick defaults and justify them; Traefik CircuitBreaker defaults and existing `tokenbucket` defaults are the reference points.
- Whether the admission call returns a `retryAfter` duration alongside the boolean (inclination: yes, mirroring `tokenbucket.Allow`).
- Exported names for the knobs and for the gate type itself.
- Whether the gate exposes current state for observability, and if so how, without inviting decisions from a stale read.

## Tensions
- Ticket vs existing debt: `knowledge/debt/2026-09-12-simpleredis-dial-circuit-breaker.md` is a SimpleRedis dial-failure policy. This ticket is a backend-outcome admission gate. Do not merge the two.
- Ticket vs README: `README.md` still describes the module as one Redis-backed stack; this package has no store. The ticket already asks for the one-line README fix — not a reshape of the gate.
- Ticket vs `windowLocked`: the dump says buffered windowcounter only GET-seeds at window rollover; `windowcounter/limiter.go` `windowLocked` also GET-seeds whenever `localDelta == 0`. The exploration conclusion still holds (do not cite either package as surviving a store outage). No product change to windowcounter.
