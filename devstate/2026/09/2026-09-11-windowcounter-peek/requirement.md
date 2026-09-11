# Requirement
IssueKey: 2026-09-11-windowcounter-peek

## Problem

`windowcounter` can only observe the sliding estimate by calling `Take`, which always increments. A middleware that skips a backend after a failure threshold cannot tell whether the window has cooled without counting another hit. There is no `Peek`.

## Current (code)

- `windowcounter/limiter.go:77-100` — `Take(key, limit, window) (bool, float64, error)` increments then returns allowed + sliding estimate. No `Peek`.
- `windowcounter/limiter.go:102-105` — `Allow` is an alias of `Take` (increments).
- `windowcounter/limiter.go:107-124` — exact mode (`sync_rate == 0`): `INCR` current, `EXPIRE` on first hit, `GET` previous via `getCount`. Formula `current + previous × weight`.
- `windowcounter/limiter.go:126-143` — buffered Take: under `l.mu`, `windowLocked` then `bufferedCountLocked`, then `localDelta++`. Estimate from `redis_known + local_delta` plus previous × weight.
- `windowcounter/limiter.go:145-168` — `windowLocked` GETs Redis on first sight **and again whenever `localDelta == 0`** on an already-buffered key.
- `windowcounter/limiter.go:170-182` — `bufferedCountLocked` returns `redis_known + local_delta` without GET when the key is already in `windows`.
- `windowcounter/limiter.go:184-198` — `getCount` uses `SimpleRedis.Get`; miss is zero; other errors propagate.
- `windowcounter/limiter.go:323-326` — Redis key `{opaqueKey}:{windowStartUnix}`.
- `simpleredis/simpleredis.go:88-117` — `Get` and `MGet` exist. `windowcounter` Take never calls `MGet`.
- `windowcounter/fake_redis_test.go:41-89` — fake RESP handles AUTH/SELECT/GET/INCR/EXPIRE/EVAL. No GET-count. `MGET` falls through to `+OK` (not an array). `testBulk` comment mentions MGET; serve has no MGET case.
- `windowcounter/limiter_test.go` — unit Take/Allow/flush/share; `TestAllow_IsTake` (`:239`). No Peek cases.
- `windowcounter/live_test.go:11-36` — table-driven Redis/Dragonfly from `WINDOWCOUNTER_LIVE_*`; skip on `-short` or both addrs unset. Scenarios are Take-only.
- `windowcounter/yaegi_test.go:15-56` — interpreted probe calls Take only (`takeprobeSrc`).
- `.github/workflows/ci.yml:27-51` — Test job starts Redis+Dragonfly, sets both live addrs, runs `go test -v ./...` (not `-short`).
- `openspec/specs/std_go_windowcounter_sliding-take/spec.md` — Take/Allow increment-then-compare; Redis errors; unit/Yaegi of Take. No Peek requirement.
- `openspec/specs/std_go_windowcounter_sync-flush/spec.md` — exact INCR vs buffered EVAL; live both engines. Peek not mentioned.
- `knowledge/devdocs/std_go_windowcounter.md` — documents Take/Allow and sync_rate; no Peek vs Take.

## Desired

- Add `Peek(key, limit, window) (allowed bool, estimated float64, err)` on the existing `Limiter`: same args, same formula, same Redis keys as Take; **no increment**.
- One limiter, one store. `sync_rate > 0`: Peek reads `redis_known + local_delta` under the same lock as Take (memory hot path). `sync_rate == 0`: Peek is exact (`GET`/`MGET` every call). Peek does not get a separate consistency mode.
- Buffered skip storm (many Peeks, zero Takes): `local_delta` stays 0; do **not** GET Redis on every Peek because delta is 0. Refresh on seed, window roll, and the existing flush/sync. Estimate still ages via `now` in the weight.
- `Allow` stays an alias of Take. Do not make Allow mean Peek.
- Redis errors: same as Take (`redis:unreachable` / timeout). No fail-open/fail-close in the library.
- Tests at today’s bar: unit fake TCP, live Redis + Dragonfly (table-driven addr), Yaegi live. CI must keep running live files (no `-short` skip of those files in CI). Prove: N Peeks then Take sees count 1; Peek and Take agree on estimated/allowed for the same clock and keys **before** the increment; after enough Takes to deny, Peek stays denied without further Takes, then becomes allowed as the window slides (clock + formula, not a `2×window` guess); buffered skip storm does not GET every Peek; exact Peek does hit Redis (GET/MGET); both engines; Yaegi probe calls Peek and Take.
- Spec delta on existing `std_go_windowcounter_*` leaves, not a new package name. Update `knowledge/devdocs/std_go_windowcounter.md` (Peek vs Take; buffered hot path).

Intended caller loop (library does not implement a breaker): Peek (if denied, skip backend) → call backend → on failure, Take.

## Affected

- `windowcounter/limiter.go` (Peek; likely a buffered read that does not reuse `windowLocked`’s GET-when-delta-0)
- `windowcounter/limiter_test.go`, `fake_redis_test.go` (GET counting; MGET if exact Peek uses it)
- `windowcounter/live_test.go`, `windowcounter/yaegi_test.go` (Peek + Take; both engines)
- `openspec/specs/std_go_windowcounter_sliding-take/spec.md` and/or `std_go_windowcounter_sync-flush/spec.md`
- `knowledge/devdocs/std_go_windowcounter.md`

## Out of scope

- Circuit-breaker state machine, half-open, consecutive-failure, error-rate
- Peek that always hits Redis in buffered mode
- Token bucket / new package
- HTTP, 429, identity inside the library
- Changing `Allow` to mean Peek
- Fail-open/fail-close policy inside the library

## Unknowns

- Exact-mode Peek: two `GET`s vs one `MGet` of current+previous. Ticket allows either; fake today does not serve MGET.
- How Peek shares Take’s buffer without the GET-when-`localDelta==0` path (`windowLocked` vs a sibling reader). Ticket forbids flood; Take still GETs on that path today.
- Whether GET-count on the fake is a counter field or command log (needed to prove skip-storm vs exact Redis hits).

## Tensions

- Ticket says buffered Peek refresh is “same cadence as buffered Take.” `windowLocked` (`limiter.go:152-158`) GETs whenever `localDelta == 0`. For Take that is once per post-flush Take. For Peek-only skip storms it would GET every call. Honour no-flood; do not treat that GET as Peek’s cadence.
- Specs and usage packet describe Take only; Peek is a delta on those units, not a new spec family.
- Fake `MGET` is unimplemented; exact Peek tests that send MGET will not see real Redis replies until the fake grows an array reply.
