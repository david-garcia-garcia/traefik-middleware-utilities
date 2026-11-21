# Requirement
IssueKey: 2026-09-13-tokenbucket-bug-allow-wait-units

## Problem
After burst is spent, `Allow` can refund the token (truncated-µs maxDelay) and still return allowed, so the next call repeats forever (fail-open).

## Current (code)
- `consumeOne` refunds when `waitMicro > float64(maxDelayMicro)` with `maxDelayMicro` from `maxDelay.Microseconds()` — `tokenbucket/clock.go`.
- `Memory.Allow` passes `m.clock.maxDelay.Microseconds()` into `consumeOne`, then admits with `waitDuration(waitMicro)` + `allowedFromWait(wait, m.clock.maxDelay)` — `tokenbucket/memory.go`.
- `Redis.Allow` passes `maxDelay.Microseconds()` as ARGV[5]; Lua refunds when `wait_duration > max_delay`; Go then `waitDuration` + `allowedFromWait(wait, r.clock.maxDelay)` — `tokenbucket/redis.go`, `tokenbucket/lua.go`.
- `waitDuration` casts `waitMicro * time.Microsecond` to `time.Duration` (truncates; overflow can go negative). `allowedFromWait` is `wait <= maxDelay` on full Duration — `tokenbucket/clock.go`.
- Dest `Allow` is `(bool, time.Duration, error)`; existing tests use the Duration return — `tokenbucket/limiter_test.go`.
- Dest `tokenbucket/` has no fail-open cases for 1500ns / 1e12+maxDelay 0 / 1e-12 overflow. Caller repros exist only in the dirty checkout, not on dest.
- `tokenbucket/BUGS.md` — not found on dest.

## Desired
- Admit and refund share one microseconds comparison: allowed iff `waitMicro <= float64(maxDelay.Microseconds())` (Lua refunds on `>`; equal wait still admits).
- `Memory.Allow` and `Redis.Allow` both use that. Redis already has `waitMicro`; do not convert to `time.Duration` to decide the bool.
- Leave Lua as Traefik’s script. Leave ARGV as `maxDelay.Microseconds()`.
- Delete `waitDuration` and `allowedFromWait(wait, maxDelay time.Duration)` once the bool is not derived from a Duration.
- Do not round maxDelay up, skip refund, or special-case 1500ns.
- Tests first: add the three dest-failing cases (1500ns + rate 1e6/1.2; 1µs + rate 999999; maxDelay=0 + rate 1e12; rate 1e-12 overflow), confirm they fail on dest, then apply the how, then confirm they pass. Keep assertions. Measure `go test -short -count=1 ./tokenbucket`.

## Affected
- `tokenbucket/clock.go` (`waitDuration`, `allowedFromWait`; admit helper if any)
- `tokenbucket/memory.go` (`Memory.Allow`)
- `tokenbucket/redis.go` (`Redis.Allow`)
- New package tests under `tokenbucket/` (adapted caller repros)

## Out of scope
- Other tokenbucket bugs (Eval nan/Inf wait, and the rest of caller `BUGS.md`).
- Changing Lua / ARGV encoding.
- Rounding maxDelay up, skipping refund, or special-casing 1500ns.

## Unknowns
- Dest `Allow` still returns `time.Duration`. After deleting `waitDuration`, how that second return is produced is not specified (inline convert vs drop the Duration). Existing dest tests read `wait`.
- Caller repros call `Allow` as `(bool, error)`; dest is `(bool, time.Duration, error)` — adapt without weakening.

## Tensions
- Agreed how deletes `waitDuration`; dest `Allow` still returns `time.Duration` (`tokenbucket/memory.go`, `tokenbucket/limiter_test.go`). Do not reopen the µs admit rule.
- Caller `BUGS.md` item 1 is the ask; that file is not on dest.
