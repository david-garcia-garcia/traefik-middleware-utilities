## Why

Dest `Allow` refunds in truncated microseconds (`consumeOne` / Lua ARGV `maxDelay.Microseconds()`) but admits with `waitDuration(waitMicro) <= maxDelay`. When both are true, tokens are restored and `Allow` returns true forever (fail-open).

## What Changes

- Admit and refund share one microseconds comparison: `allowed` iff `waitMicro <= float64(maxDelay.Microseconds())` (Lua refunds on `>`; equal wait still admits).
- `Memory.Allow` and `Redis.Allow` both use that. Redis already parsed `waitMicro`; do not convert to `time.Duration` to decide the bool.
- Leave Lua as Traefik’s script. Leave ARGV as `maxDelay.Microseconds()`.
- Delete `allowedFromWait`. Delete named `waitDuration`. Keep `(bool, time.Duration, error)`; inline the Duration conversion only for the second return.
- Do not round maxDelay up, skip refund, or special-case 1500ns.
- Tests first: dest-failing cases (1500ns + rate `1e6/1.2`; 1µs + rate 999999; maxDelay=0 + rate `1e12`; rate `1e-12` overflow), then the how, then pass.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_tokenbucket_allow`: `allowed` is `waitMicro` versus `maxDelay.Microseconds()`, not `waitDuration(waitMicro) <= maxDelay`. Refund and admit are the same comparison. Duration return stays for callers; it is not the admit gate.

## Impact

- `tokenbucket/clock.go`, `tokenbucket/memory.go`, `tokenbucket/redis.go`.
- New package tests under `tokenbucket/` (adapted caller repros). Existing `Allow` arity unchanged.
- Usage packet gotcha that “Go maps allowed from wait vs maxDelay” becomes microseconds.
- Lua / ARGV encoding unchanged. Other tokenbucket bugs out of scope.
