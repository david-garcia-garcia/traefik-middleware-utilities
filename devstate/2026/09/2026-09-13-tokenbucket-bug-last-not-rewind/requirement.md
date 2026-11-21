# Requirement
IssueKey: 2026-09-13-tokenbucket-bug-last-not-rewind

## Problem
`consumeOne` clamps elapsed when `nowMicro < last` but returns `nowMicro` as `newLast`, so a backward now is persisted. Lua `HSET last, t` does the same. Memory samples `m.now()` before `mu.Lock()`, so a stale timestamp can overwrite a newer `last`. Extra refill / double grant.

## Current (code)
- `tokenbucket/clock.go` `consumeOne` — `if nowMicro < last { last = nowMicro }` then `elapsed := float64(nowMicro - last)`; returns `nowMicro` as `newLast` (not the pre-clamp last).
- `tokenbucket/lua.go` `allowScript` — `if t < last then last = t end` for elapsed; `redis.call('hset', key, 'last', t, 'tokens', tokens)` always writes `t`. MIT notice at top of `lua.go`.
- `tokenbucket/memory.go` `Allow` — `now := m.now()` then `nowMicro := now.UnixMicro()` then `m.mu.Lock()`.
- `tokenbucket/memory.go` `Allow` — stores `entry.last = last` from `consumeOne` (the returned `nowMicro`).
- `tokenbucket/redis.go` `Allow` — passes `nowMicro` as ARGV `t`; does not write the hash from Go.
- `tokenbucket/fake_redis_test.go` EVAL path — applies `consumeOne` and stores that `last` (inherits persist of `nowMicro`).
- Dest `tokenbucket/` has no `repro_hunt_clock_last_backward_test.go` (`not found` on dest). That file exists only on the caller’s dirty tree. Dest helpers `newBurst1Delay0` / `mustAllow` are `not found`; dest `limiter_test.go` has `testTTL` and `Allow(ctx, key)` as `(bool, time.Duration, error)`.
- `openspec/specs/std_go_tokenbucket_lua-eval/spec.md` — copied Traefik `AllowTokenBucketRaw` (MIT notice kept); last/tokens change only inside Eval. Does not name persist-max or clock rewind.
- `openspec/specs/std_go_tokenbucket_allow/spec.md` — Memory uses Lua formulas; no last-not-in-the-past rule.
- `knowledge/devdocs/std_go_tokenbucket.md` — no last-rewind / sample-before-lock gotcha.

## Desired
1. Land product tests in `tokenbucket/` that **fail on current dest** (`go test ./tokenbucket`). Tests first, then the fix. Do not weaken them to pass.
2. Copy/adapt `TestRepro_StaleNowRewindsLastDoubleRefill` from caller `tokenbucket/repro_hunt_clock_last_backward_test.go`: subtests `sequential` and `goroutines_stale_samples_before_fresh_lock`. Dest `Allow` is `(ctx, key) (bool, time.Duration, error)`; tests must use that signature. Copy or inline `newBurst1Delay0` / `mustAllow` (those helpers are not on dest).
3. Agreed how: never persist a `last` in the past. Elapsed still uses the backward clamp so elapsed is not negative.
   - `consumeOne` stores `max(previous last, nowMicro)`, not raw `nowMicro` after the clamp.
   - Lua `HSET` the same (`last = max(bucket.last, t)`). Keep the MIT notice.
   - Memory reads `now` after `mu.Lock()` so lock order is the clock order.
4. Do not only fix the goroutine path. Do not drop the elapsed clamp.
5. Then those tests PASS and existing `tokenbucket` tests stay green.

## Affected
- `tokenbucket/clock.go` (`consumeOne` persisted last)
- `tokenbucket/lua.go` (`HSET last`)
- `tokenbucket/memory.go` (`Allow` clock sample vs lock)
- `tokenbucket/` tests (new failing-then-green coverage; named repro file or `limiter_test.go`)
- `openspec/specs/std_go_tokenbucket_lua-eval/spec.md` (HSET last = max; Traefik copy is no longer bit-identical on that field)
- `openspec/specs/std_go_tokenbucket_allow/spec.md` (last must not rewind)
- `knowledge/devdocs/std_go_tokenbucket.md` (last-rewind / sample-before-lock)

## Out of scope
- Idle-fill-to-burst (new key `last=0` / epoch fill)
- Wait mapping (`allowedFromWait` Duration vs microseconds)
- NaN / Inf rate
- TTL seconds truncation
- Cap eviction fresh burst
- Other packages (`simpleredis`, `reclaim`, `windowcounter`)
- Dropping the elapsed clamp
- Fixing only the goroutine sample-before-lock path

## Unknowns
- Whether new tests land as `repro_hunt_clock_last_backward_test.go` or are folded into `limiter_test.go`.
- Whether Lua uses `math.max(bucket.last, t)` or an equivalent `if t > bucket.last` before HSET.
- Whether a Redis/fake-Redis sequential case is required in the same test file, or memory plus Lua/consumeOne symmetry is enough (fake Redis already calls `consumeOne`).

## Tensions
- Ticket: Lua `HSET last` becomes `max(bucket.last, t)`. Spec `std_go_tokenbucket_lua-eval` says copied Traefik `AllowTokenBucketRaw`. Ticket wins; keep MIT notice; script body changes on that field.
- Caller dirty-tree repro calls 2-value `Allow(key)`; dest `Allow` takes `ctx` and returns three values. Tests copied onto dest must match dest’s signature.
- Ticket: do not only fix the goroutine path. Sequential rewind is `consumeOne`/`HSET` returning `t`; goroutine is sample-before-lock. Both are in scope.
- Ticket: do not drop the elapsed clamp. Dest already clamps `last` for elapsed then persists `nowMicro`; the gap is persist, not the clamp.
