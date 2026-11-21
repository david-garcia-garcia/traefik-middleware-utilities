# 2026-09-13-tokenbucket-bug-allow-wait-units

THIS BUG ONLY. Do not fix the other tokenbucket bugs.

## Problem

Allow refunds in truncated microseconds (consumeOne / Lua ARGV maxDelay.Microseconds()) but admits with waitDuration(waitMicro) <= maxDelay (full Duration). When both are true, tokens are restored and Allow returns true forever (fail-open).

## Agreed how (implement this, do not reopen)

Admit and refund are one comparison, in microseconds, same units Lua already uses.

- allowed iff waitMicro <= float64(maxDelay.Microseconds()) (Lua refunds on >; equal wait still admits).
- Memory.Allow and Redis.Allow both use that. Redis already parsed waitMicro; do not convert to time.Duration to decide the bool.
- Leave Lua as Traefik’s script. Leave ARGV as maxDelay.Microseconds().
- waitDuration / allowedFromWait(wait, maxDelay time.Duration) are unused once the bool is not derived from a Duration — delete them rather than keep a conversion Allow no longer returns.
- Do not round maxDelay up, skip refund, or special-case 1500ns.

Source of truth: tokenbucket/BUGS.md item 1 (Agreed how). Production sources on dest: tokenbucket/clock.go, memory.go, redis.go.

## Tests first (hard)

Before changing production behavior, CREATE tests that reproduce the fail-open, run them, confirm they FAIL on dest code, then implement the agreed how, then confirm they PASS.

Copy/adapt these existing repros from the caller workspace (they are uncommitted there; copy content into YOUR worktree as package tests, not BUGS.md):

- tokenbucket/repro_maxdelay_truncation_test.go — TestRepro_MaxDelayTruncationFailOpen (rate 1e6/1.2 + maxDelay 1500ns; rate 999999 + maxDelay 1µs)
- tokenbucket/repro_hunt_maxdelay0_wait_trunc_test.go — maxDelay=0 + rate 1e12
- tokenbucket/repro_hunt_wait_duration_overflow_test.go — rate 1e-12 waitDuration overflow

You may rename TestRepro_ to durable names. Keep the cases. Do not weaken assertions.

Measure: go test -short -count=1 ./tokenbucket (and the new tests isolated). Report fail then pass.
