# Handoff: Leaky bucket (`leakybucket/`)

Pass this to explore, then implement. Sibling of `tokenbucket/` (Traefik) and `windowcounter/` (Kong) — **do not merge the clocks**. Do not invert Traefik’s Lua and call it leaky.

## Goal

A Yaegi-safe **leaky bucket**: water is poured in, leaks at a constant rate, overflow = deny. Distributed via Redis/Dragonfly. Cheap Redis uses **Kong’s sync pattern** (additive deltas on a timer), not Kong’s **window math**.

Clock (classic leaky): [Leaky bucket](https://en.wikipedia.org/wiki/Leaky_bucket).  
Flush transport (pours, not snapshots): [Kong `sync_rate` + `INCRBY` pipeline](https://github.com/Kong/kong/blob/master/kong/plugins/rate-limiting/policies/init.lua) — steal **delta + timer**, not sliding-window weights.

`tokens ≈ capacity − water` is why this is not `tokenbucket/`. Callers that think in **fill** (errors, unhealthy backend) use this. HTTP burst-after-idle stays Traefik.

## Decisions (locked)

- **Clock:** `water(t) = max(0, water(t0) + poured − leak×Δt)`, cap `capacity`. `Take`/`Add` pours `n` (default 1). Overflow → deny (do not pour). `Level` is current water after leak. Empty bucket = room to burst up to `capacity`; then wait for leak. Not a sliding window; no minute-boundary double-dip.
- **Package:** `leakybucket/` (`package leakybucket`). Import `…/leakybucket`. Not `ratelimit/`, not `bucket/`, not `tokenbucket/`.
- **API:** opaque key; `Add`/`Take` → allowed + level (and optional time-until-not-full). `Level` does not pour. No HTTP, no 429, no fail-open/fail-close, no health-gate. Caller prefixes keys.
- **`sync_rate` (Kong Advanced knobs):** `0` = exact: every pour is Redis `EVAL` (leak once on the server, then add). `>0` = seconds (min ~20ms): admit from `leaked(redis_water, last_sync, now) + local_pours`; timer `EVAL` applies leak **once**, adds `local_pours`, stores `{water, last}`. **Never `SET` a replica’s `{water, last}` blob** (last-write-wins / double-leak).
- **Stores:** in-memory (mutex, single process, exact) **and** Redis/Dragonfly as above. Same `leak` / `capacity`. Memory is a full backend, not only a buffer (the buffer is `local_pours` when `sync_rate>0`).
- **Redis:** `simpleredis.Eval` (script: leak, add delta, cap, `EXPIRE`). Keys in `KEYS`. Lua 5.4 / Dragonfly: no `table.maxn`. No `go-redis`, no `EVALSHA` in v1. Blocked if `Eval` is missing.
- **Timer:** reclaim `Sleep`/`Wake`/`Close` so the flush goroutine dies on Traefik reload.
- **Redis errors:** return `redis:unreachable` / timeout. Policy stays in the middleware.
- **EVALSHA:** later.

Do **not** implement Traefik `maxDelay` refund, Kong window keys, or `x/time/rate`.

## Tests (required)

Same bar as [`handoff-kong-window-limiter.md`](handoff-kong-window-limiter.md) / [`handoff-traefik-token-limiter.md`](handoff-traefik-token-limiter.md).

**Unit:** fake TCP + in-memory leak (pour to cap → deny; idle → water drains; `Level` after Δt).

**Live Go e2e:** start **Redis and Dragonfly**, `go test` calls `Add`/`Take`/`Level` on those ports. No Traefik, no Pester.

Prove at least:

- `sync_rate=0`: pour `capacity` then deny; after leak time, `Take` succeeds again
- `sync_rate>0`: two limiter instances share one key; **no** last-write-wins (two pours both count); over-allow is bounded by the sync interval, not unbounded
- in-memory and Redis agree on admit/deny for the same leak/capacity sequence when `sync_rate=0`
- **both** engines (table-driven addr)

**Yaegi live:** the **same live scenarios** interpreted (stdlib only, `useunsafe` false). Compiled test starts/skips engines; interpreted probe calls `Take`. CI starts both engines and **must not** skip.

Pester/Traefik plugin is optional extra, not a substitute.
