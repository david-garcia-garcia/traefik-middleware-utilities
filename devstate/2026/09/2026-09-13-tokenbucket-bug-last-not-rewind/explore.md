# Explore
IssueKey: 2026-09-13-tokenbucket-bug-last-not-rewind

## Concepts

```
  dest consumeOne                          dest Memory.Allow
  ─────────────────                        ─────────────────
  last_elapsed = last                      now = m.now()          ← BEFORE lock
  if now < last: last = now                Lock
  elapsed = now - last                     consumeOne(..., now)
  refill; consume 1                        entry.last = newLast   ← raw nowMicro
  return tokens, now, wait                 Unlock
```

Two ways `last` moves backward, same extra refill:

1. Sequential: Allow at t2 grants the 5s refill; a later Allow with now=t1 clamps elapsed (no negative refill) but **stores t1**. Next Allow at t2 treats 5s as new elapsed and grants again.
2. Concurrent Memory: goroutine A samples t1 before the lock and waits; goroutine B samples t2, runs first, stores last=t2; A then writes last=t1. Same rewind.

Lua `AllowTokenBucketRaw` (Traefik pin + this package) already clamps `last` for elapsed, then `HSET last, t`. Redis is the sequential case on another store. Fake Redis EVAL applies `consumeOne`, not a Lua VM, so a Memory-only test cannot fail a stale `lua.go` HSET.

Identity: Allow takes an opaque key. This change does not set or reconstruct client address, user, tenant, or Host.

## Decisions

- Persist `last = max(previous last, nowMicro)` in `consumeOne`. Keep the elapsed clamp (`if nowMicro < last { last = nowMicro }`) so elapsed is never negative. Capture previous last **before** that clamp.
- Lua sibling: after the same elapsed clamp, `HSET last` is `max(bucket.last, t)`, not raw `t`. Keep the MIT notice. Do not keep Traefik bit-identity on that field (requirement tension; ticket wins).
- Memory: read `m.now()` **after** `mu.Lock()` so lock order is clock order. `expireAt` uses that same `now`.
- Tests first: copy/adapt `TestRepro_StaleNowRewindsLastDoubleRefill` (`sequential` + `goroutines_stale_samples_before_fresh_lock`) into dest `tokenbucket/repro_hunt_clock_last_backward_test.go`. Dest `Allow` is `(ctx, key) (bool, time.Duration, error)` — tests use that. Inline `newBurst1Delay0` / `mustAllow` (or same-package `testTTL`). Confirm FAIL on dest, then fix, then PASS.
- Lua proof: fake Redis stores `consumeOne`'s last, so it cannot catch an unfixed script. Add a small assertion in that test file that `allowScript` HSET last is not raw `t` (max of `bucket.last` and `t`). Do not add a live-Redis sequence for this ticket.
- Out of scope stays out: idle-fill-to-burst, wait mapping, NaN/Inf, TTL truncation, cap eviction.
- Devdocs: consume `knowledge/devdocs/std_go_tokenbucket.md`. Usage is enough to call Allow. Last-not-rewind is a library invariant; fill the gotcha at implement / devdocsimpact. No new Language this phase.
- Research: Traefik `ext_traefik_ratelimiter_token-bucket` already pins `HSET last, t`. No new research folder.

## Open questions

- Q: Where do the new tests live — `repro_hunt_clock_last_backward_test.go` or `limiter_test.go`?
  Rank: additive asked — new test file this change creates; Desired line 2 names copy/adapt of that hunt file
  Decision: assumed — land `tokenbucket/repro_hunt_clock_last_backward_test.go` with dest Allow signature; helpers copied or inlined there (`testTTL` already in package).
  By: explore

- Q: Lua persist as `math.max(bucket.last, t)` or an equivalent `if t > bucket.last` before HSET?
  Rank: additive asked — Desired names `last = max(bucket.last, t)`; the Lua operator is mechanism, not a second contract
  Decision: assumed — two explicit steps matching `consumeOne`: elapsed clamp unchanged, then persist `bucket.last` unless `t` is greater. `math.max` is equivalent if the script already uses `math.min`; either form is the same persist-max.
  By: explore

- Q: Is a Redis/fake-Redis sequential Allow case required in the same test file?
  Rank: additive asked — Desired names Lua HSET max; it does not name a Redis Allow sequence. Fake Redis applies `consumeOne`, so it cannot fail a stale `lua.go`.
  Decision: assumed — Memory sequential + goroutine subtests as specified; plus one script assertion that HSET last is persist-max, not raw `t`. No extra Redis Allow sequence.
  By: explore
