# Requirement
IssueKey: 2026-09-13-tokenbucket-bug-idle-fill-burst

## Problem
A new token-bucket key is supposed to start full (`burst`) then consume 1. Dest fills from `tokens=0` `last=0` and elapsed since Unix epoch. That is empty at `Unix(0,0)` (elapsed 0) and still below burst when `burst` is huge (e.g. `1e12`) relative to `elapsed*rate`.

## Current (code)
- `tokenbucket/lua.go` — empty `HGETALL` (`#rl_source ~= 4`) leaves `tokens = 0` and `last = 0`, then `elapsed = t - last` (Unix microseconds since epoch) and `min(tokens+limit*elapsed, burst) - 1`. MIT notice is present.
- `tokenbucket/memory.go` — new `memEntry{}` (including after TTL delete) is zeros; `Allow` always calls `consumeOne` with those zeros.
- `tokenbucket/clock.go` `consumeOne` — same formulas: `elapsed = nowMicro - last`; at `nowMicro=0` and `last=0`, `elapsed=0`, `tokens = -1`.
- `tokenbucket/fake_redis_test.go` — missing hash uses `tokens=0` `last=0` then `consumeOne` (Lua empty-hash math, not a Lua interpreter).
- `tokenbucket/limiter_test.go` `TestMemory_BurstAfterIdle`, `TestMemory_IdlePastTTLStartsFull`, `TestMemoryAndRedis_Agree` — freeze at `Unix(1_700_000_000)` where `elapsed*rate` already exceeds typical burst, so they pass without a true full start.
- Dest `tokenbucket/` has no `repro_epoch_idle_burst_test.go`. Caller tree file of that name calls `Allow("new-key")`; dest Allow is `Allow(ctx, key)` returning `(bool, time.Duration, error)` (`tokenbucket/memory.go`, `tokenbucket/redis.go`).
- `openspec/specs/std_go_tokenbucket_allow/spec.md` — “Idle fills to burst”: new key treated as filled to `burst` before consuming 1; MUST NOT import `golang.org/x/time/rate`.
- `openspec/specs/std_go_tokenbucket_lua-eval/spec.md` — Eval of the copied Traefik script; Memory and Redis must agree on allowed and wait class.
- `knowledge/research/ext_traefik_ratelimiter_token-bucket/notes.md` — Traefik Redis Lua uses `last=0`; Traefik in-memory `x/time/rate` starts full. This package must not import `x/time/rate`.

## Desired
1. Tests first: create tests that **fail on dest**, then fix, then PASS. Copy/adapt caller `tokenbucket/repro_epoch_idle_burst_test.go` `TestRepro_NewKeyFillsToBurstAtEpoch` onto dest (`epoch_clock` burst 5; `huge_burst_elapsed_below_burst` burst `1e12`). Use dest `Allow(ctx, key)`. No `//go:build bugrepro`. Run under `go test -short ./tokenbucket`.
2. Missing state is a full bucket, then consume 1. Both stores.
3. Lua: empty `HGETALL` (`#rl_source ~= 4`) sets `tokens = burst` and `last = t` (elapsed 0), then existing refill/consume. Keep the MIT notice. Do not rely on `last=0` + elapsed-from-epoch.
4. Memory: a new `memEntry` (including after TTL delete) starts `tokens = burst`, `last = nowMicro`, not zeros. Same formulas after that.
5. Do not fill Memory only — Redis Lua must match.
6. Prefer Memory starting full so `consumeOne` stays consume math. If fake missing-hash still seeds zeros, agreement tests break after Memory starts full — treat missing hash like Lua (`tokens=burst`, `last=now`) instead of changing `consumeOne` for `last=0`.
7. Prove Redis/fake or document that Lua empty hash matches (Eval encoding / `TestMemoryAndRedis_Agree` without live engines; `-short`).
8. Do not import `golang.org/x/time/rate`.

## Affected
- `tokenbucket/lua.go` (empty hash seed)
- `tokenbucket/memory.go` (new `memEntry` after miss / TTL delete)
- `tokenbucket/fake_redis_test.go` (missing-hash seed so agreement still holds)
- `tokenbucket/` tests (new failing-then-green coverage; adapt dest Allow signature)
- `openspec/specs/std_go_tokenbucket_lua-eval/spec.md` (copied Traefik script vs intentional empty-hash fill)
- `knowledge/devdocs/std_go_tokenbucket.md` (new-key fill)

## Out of scope
- Wait mapping / `allowedFromWait` vs refund units (BUGS.md item 1)
- Eval wait `nan` / `Inf` (item 2)
- NaN / Inf rate (item 3)
- TTL seconds truncation (item 4)
- Stale `now` rewind of `last` (item 6) unless required for this fill
- Source-cap eviction (item 7)
- Importing `golang.org/x/time/rate`
- Changing dest `Allow(ctx, key)` signature or dropping wait from the return
- Live Redis/Dragonfly as the only proof (`-short` must suffice)

## Unknowns
- Whether the adapted test lives as `repro_epoch_idle_burst_test.go` or is folded into `limiter_test.go`.
- Whether Redis proof under `-short` is fake missing-hash + `TestMemoryAndRedis_Agree` only, or also an Eval argv/script assertion that the empty-hash branch is in `allowScript`.
- Whether TTL-delete new entry is covered only by existing `TestMemory_IdlePastTTLStartsFull` plus the new epoch/huge cases, or needs its own subtest at epoch.

## Tensions
- Ticket: change copied Traefik Lua empty hash to `tokens=burst` `last=t`. Spec `std_go_tokenbucket_lua-eval` says admit via the copied Traefik script. Ticket wins; propose must record the intentional Lua delta and keep the MIT notice.
- Spec `std_go_tokenbucket_allow` already requires idle/new key filled to burst; dest Lua/Memory implement Traefik’s `last=0` approximation instead. Ticket aligns code to that Allow spec, not to Traefik Redis idle math.
- Caller repro calls `Allow("new-key")`; dest Allow is `Allow(ctx, key)` with wait. Adapt the test; do not change the dest Allow API for this bug.
- Ticket: prefer not to change `consumeOne` for `last=0`. Fake currently shares `consumeOne` zeros with dest Lua. After Memory starts full, fake must seed like Lua or agreement fails.
