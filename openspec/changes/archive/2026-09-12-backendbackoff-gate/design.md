## Context

DestBranch has `tokenbucket.Memory` (time-refill Allow, `maxMemorySources` 65536, `dropExpired`/`dropOne`, `SetNowForTest`) and `reclaim.Open` (store `any`, optional Close). Neither is a backend-health gate. See proposal.md for why. Specs: `std_go_backendbackoff_allow`, `std_go_backendbackoff_cooldown`. Explore: `devstate/2026/09/2026-09-12-backendbackoff/explore.md`. Yaegi: stdlib only, concrete types, no generics, no unsafe.

## Goals / Non-Goals

**Goals:**
- One `Gate` type, mutex, in-process map. Credit clock and cooldown clock stay separate fields.
- Copy the memory-map shape from `tokenbucket/memory.go` without importing that package.
- Prove with `-short` plus Yaegi interp and `TestAlloc*` on warm Allow.

**Non-Goals:**
- Redis / `simpleredis` / shared layer (debt note already written).
- Snapshot / State getter.
- Pester plugin, `*_LIVE_*`, e2e job edits.
- Importing `tokenbucket` or changing `reclaim/`.

## Decisions

1. **Type `Gate`, `New(Config) (*Gate, error)`, methods `Allow`, `Report`, `Close`.** Zero Config fields apply explore defaults. Alternative: positional New like `tokenbucket.NewMemory` — rejected; six knobs. Alternative: name `CircuitBreaker` — rejected; that is Traefik's HTTP middleware.

2. **Credit is one `float64` per key; cooldown is `openUntil`, `n`, `closedSince`, and `probeUntil`.** Trip is `credit <= 0` in CLOSED. Alternative: windowed ratio — rejected on the requirement. Alternative: reuse `tokenbucket.Memory.Allow` — rejected; that consumes a refill token.

3. **`Allow(ctx, key) (bool, time.Duration, error)` and `Report(key, success bool)`.** Report has no context so a canceled request that already hit the backend still lands. Alternative: Report(ctx, ...) checking ctx.Err() — rejected; that drops the outcome.

4. **Duplicate `maxMemorySources = 65536` and the dropExpired/dropOne pair.** Alternative: import tokenbucket — rejected; consumeOne is the wrong job.

5. **HALF-OPEN: one outstanding probe, lease = BaseCooldown.** Lost Report: next Allow after the lease may probe. Idle TTL is last resort. Alternative: Traefik recovering (linear ramp) — rejected; the requirement is one probe. Alternative: stuck HALF-OPEN until TTL — rejected; Retry-After would be up to 60s.

6. **Jitter via `math/rand` under the mutex (or a per-Gate source).** `Jitter` 0 disables it for tests. Alternative: crypto/rand — rejected; not needed for desync.

7. **`SetNowForTest` on Gate.** Same job as tokenbucket. Production callers must not use it.

8. **Yaegi test copies non-test `backendbackoff` sources into a GOPATH interp (stdlib only), no simpleredis copy.** Follow `tokenbucket/limiter_yaegi_test.go` layout without the Redis fake.

9. **Alloc ceiling: warm Allow on an existing CLOSED key.** First-sight Allow may allocate a map entry; the ceiling is the steady path. Skip under race. Measure on Go 1.21 then set slack like `simpleredis/bench_test.go` (+1 alloc, +64 B or +20%).

10. **README one line in Why this exists:** Yaegi plus shared middleware consumers, not "one Redis-backed stack" as the only justification. Layout adds `backendbackoff/`.

## Risks / Trade-offs

- [N replicas each burn B failures independently] → Mitigation: deferred shared layer; jitter desyncs probes. Debt file already notes this.
- [Lost probe Report sticks HALF-OPEN] → Mitigation: BaseCooldown lease then another probe; TTL drop.
- [Deny-without-refresh would admit as a new CLOSED key] → Mitigation: Allow always refreshes expireAt.
- [float64 credit drift] → Mitigation: trip is `<= 0`; cap at B; B is small integers.
- [Yaegi interp misses a compile-only test] → Mitigation: trip path in limiter_yaegi_test.go; no LIVE env.

## Migration Plan

New package. Callers opt in. Rollback is revert. Reclaim keys should be prefixed (`backendbackoff:` + hash) when sharing Default.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
