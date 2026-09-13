# Requirement
IssueKey: 2026-09-13-tokenbucket-bug-ttl-whole-seconds

## Problem
`NewMemory` / `NewRedis` accept a `ttl` that is at least 1s but not a whole number of seconds (example 1500ms). Memory then expires at `now.Add(ttl)` (full Duration). Redis EVAL ARGV is `int64(ttl / time.Second)` so EXPIRE is 1. The stores disagree. Redis integer-second EXPIRE is the Traefik contract; Memory is not the source of truth. `New` accepted a Duration Redis cannot represent.

## Current (code)
- `tokenbucket/clock.go` `validateClock` — rejects only `ttl < time.Second`; 1500ms returns nil.
- `tokenbucket/clock.go` `errTTL` — `"tokenbucket: ttl must be at least 1s"`; no whole-second check.
- `tokenbucket/clock.go` `ttlSeconds` — `int64(c.ttl / time.Second)`; 1500ms becomes 1.
- `tokenbucket/memory.go` `NewMemory` — calls `validateClock`; stores the full `ttl` Duration.
- `tokenbucket/memory.go` `Allow` — `entry.expireAt = now.Add(m.clock.ttl)` (full Duration). Drop when `!now.Before(entry.expireAt)`. At t0+1200ms with ttl=1500ms the entry is still live.
- `tokenbucket/redis.go` `NewRedis` — same `validateClock`; 1500ms succeeds.
- `tokenbucket/redis.go` `Allow` — ARGV ttl is `strconv.FormatInt(r.clock.ttlSeconds(), 10)`.
- `tokenbucket/lua.go` `allowScript` — `redis.call('expire', key, ttl)` with that integer ARGV. No PEXPIRE.
- `tokenbucket/limiter_test.go` `TestNewMemory_RejectsInvalidClock` — `ttl=time.Millisecond` is `errTTL`; no 1500ms case. `testTTL` is `2 * time.Second`.
- `tokenbucket/fake_redis_test.go` `lastEvalCommand` / EVAL argv — EVAL script numkeys key limit burst TTL now maxdelay; TTL is argv index 6. Fake does not run EXPIRE.
- Dest `tokenbucket/` has no `repro_ttl_truncation_test.go` and no test that New rejects 1500ms.
- `openspec/specs/std_go_tokenbucket_allow/spec.md` — construction fails when `ttl < 1s`; does not require whole seconds. Entries expire after caller `ttl`.
- `openspec/specs/std_go_tokenbucket_lua-eval/spec.md` — ARGV SHALL pass ttl seconds; memory and Redis SHALL agree for the same ttl.

## Desired
1. Tests first (hard). Create tests that FAIL on dest: `NewMemory`/`NewRedis` accept 1500ms; Redis ARGV ttl is `"1"`; Memory is not expired at +1200ms. Then fix so `New(1500ms)` returns `errTTL`. Then PASS. Do not weaken the tests to pass.
2. Copy/adapt caller `tokenbucket/repro_ttl_truncation_test.go` onto dest signatures (`Allow(ctx, key) (bool, time.Duration, error)`). After the fix, reshape from “Memory vs Redis disagree” to “fractional ttl rejected” (`errors.Is(..., errTTL)`). Keep a whole-second ttl accepted (2s).
3. `validateClock` requires ttl to be a whole number of seconds (still `>= 1s`). Same `errTTL` sentinel; error text can say whole seconds. Then Memory `now.Add(ttl)` and Redis EXPIRE `ttlSeconds` are the same lifetime.
4. Do not PEXPIRE. Do not rewrite Lua. Do not silently floor Memory to 1s while `New(1500ms)` succeeds.

## Affected
- `tokenbucket/clock.go` (`validateClock`, `errTTL` text)
- `tokenbucket/` tests (new failing-then-green coverage; adapt the caller repro)
- `openspec/specs/std_go_tokenbucket_allow/spec.md` (construction: whole seconds)
- `openspec/specs/std_go_tokenbucket_lua-eval/spec.md` (agreement by rejecting unrepresentable ttl)
- `knowledge/devdocs/` tokenbucket packet if it documents `ttl < 1s` only

## Out of scope
- PEXPIRE or millisecond Redis expiry
- Rewriting Lua / Traefik `expire` integer seconds
- Silently flooring Memory expire to `ttlSeconds` while New(1500ms) succeeds
- Other `tokenbucket/BUGS.md` items (maxDelay truncation, Eval NaN, NaN/Inf rate, epoch burst, clock last rewind, cap eviction)

## Unknowns
- Exact `errTTL` string (ticket allows “whole seconds”; sentinel must stay `errTTL`).
- Whether the test file keeps the repro name or lands in `limiter_test.go`.
- Whether `NewRedis` 1500ms rejection is a dedicated case or covered only via `validateClock` plus Memory.

## Tensions
- Ticket: reject fractional ttl. Spec `std_go_tokenbucket_allow` currently fails construction only when `ttl < 1s`, so 1500ms is legal. Ticket wins; spec/usage catch up in propose.
- Spec `std_go_tokenbucket_lua-eval` “same ttl, same sequence” vs dest 1500ms: Memory still holds at +1200ms while Redis ARGV is 1. Ticket removes that pair by rejecting New instead of making Memory match EXPIRE 1 for an accepted 1500ms.
- Caller repro `Allow(key)` / two-value return is stale vs dest `Allow(ctx, key) (bool, time.Duration, error)`. Adapt; do not copy as-is.
- Caller repro asserts disagreement; after the fix it must assert New rejects 1500ms and still accept 2s. Do not keep the pre-fix “ARGV is 1 and Memory admits at +1200ms” as the post-fix contract.
