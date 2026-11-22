## Why

An idle token bucket that expires is recreated full. When the caller TTL is shorter than the time to refill `burst` (Traefik's own TTL for 30 req/min is 3s; refill takes 60s), a pause longer than that TTL grants the missing tokens. Reported upstream as https://github.com/traefik/traefik/issues/13957. There is no upstream pull request. Measured here: Memory, rate 0.5, burst 30, ttl 3s, admits 30 again after a 5s pause instead of 2.

## What Changes

- `NewMemory` and `NewRedis` keep accepting the caller's TTL (`validateClock` unchanged: at least 1s, whole seconds) and store `max(callerTTL, refillFloor)` on the shared clock. `refillFloor` is `ceil(burst/rate + maxDelay.Seconds())` whole seconds, clamped so it cannot overflow `time.Duration`.
- Memory expiry and Redis `EXPIRE` keep using that one stored duration. A raised TTL does not change how many tokens an unexpired bucket admits; an expired bucket is already full, so recreating it full grants nothing.
- GoDoc on both constructors: `ttl` is a minimum; the library raises it to the refill time because of traefik/traefik#13957.
- Tests: port the two #13957 repros (Memory admits 2; live admits at most 3) plus the control case, and unit-test the floor (short TTL raised, long TTL kept, `maxDelay` included, overflow clamp). Update the one ARGV assertion that pinned caller seconds `2` when the floor is 3.
- Rejecting a short TTL was considered and rejected: it breaks callers that pass Traefik's formula.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_tokenbucket_allow`: entries expire after the effective TTL, not always the caller argument. Construction still rejects a fractional or sub-second caller TTL. The refill floor is a stored minimum.
- `std_go_tokenbucket_lua-eval`: Redis ARGV ttl seconds and Memory expiry stay one duration, and that duration may be longer than the caller TTL. It is never shorter. A fractional caller TTL still never reaches EVAL.

## Impact

- `tokenbucket/clock.go`, `tokenbucket/memory.go`, `tokenbucket/redis.go` (GoDoc). Lua script text does not change.
- `tokenbucket/ttl_truncation_test.go` (ARGV `"2"` becomes `"3"` for rate 1, burst 2, maxDelay 1µs, caller ttl 2s).
- New `tokenbucket/traefik_issue13957_test.go` and `tokenbucket/traefik_issue13957_e2e_test.go`.
- `knowledge/devdocs/std_go_tokenbucket.md` (ttl is a minimum).
- Downstream importers are not in this repo. The constructor signature does not change. Idle keys live longer when the caller TTL was below the floor.
