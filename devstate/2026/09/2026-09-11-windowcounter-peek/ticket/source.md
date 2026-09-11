# Add Peek to windowcounter

Add `Peek` to the existing `windowcounter` package (already on master). Same sliding-window hit counter. Not a circuit breaker. Not tokenbucket. Not a new package.
Callers need to observe the sliding estimate without counting a hit, so a middleware can skip a backend after a failure threshold and know the window has cooled on the next request.

## Decisions (locked)

- **Peek is Take without increment.** Same args: `(key, limit, window) (allowed bool, estimated float64, err)`. Same formula: `current + previous × (1 − elapsed/window)`. Same Redis key scheme `{opaqueKey}:{windowStartUnix}`.
- **One limiter, one store.** `sync_rate > 0` → Peek reads `redis_known + local_delta` under the same lock as Take (hot path is memory). `sync_rate == 0` → Peek is exact (`GET`/`MGET` every call), because Take is exact. Peek does not get its own consistency mode.
- **Backoff must not Redis-flood.** While Peeking and not Taking, `local_delta` stays 0. Do not GET Redis on every Peek just because delta is 0. Refresh on seed, window roll, and the existing flush/sync — same cadence as buffered Take. Estimate still ages because the weight uses `now`.
- **Allow stays an alias of Take.** Do not make Allow mean Peek.
- **Redis errors:** same as Take (`redis:unreachable` / timeout). No fail-open/fail-close inside the library.
- **Out of scope:** circuit-breaker state machine, half-open, consecutive-failure, error-rate, Peek that always hits Redis in buffered mode, tokenbucket, HTTP, 429.

Intended caller loop (do not implement a breaker in this library):
1. Peek — if denied, skip backend
2. Call backend
3. On failure, Take

## Tests (required)

Same bar as windowcounter today: unit (fake TCP), live Redis + Dragonfly, Yaegi live. CI must not skip live files.
Prove at least:
- Peek does not increment; N Peeks then Take sees count 1
- Peek and Take agree on estimated/allowed for the same clock and keys before the increment
- After enough Takes to deny, Peek stays denied without further Takes, then becomes allowed as the window slides (clock + formula, not a `2×window` guess)
- Buffered Peek during a skip storm (many Peeks, zero Takes) does not GET Redis every call
- Exact mode Peek does hit Redis (GET/MGET)
- Both engines, table-driven addr
- Yaegi live: interpreted probe calls Peek and Take

Update `knowledge/devdocs/std_go_windowcounter.md` (Peek vs Take; buffered hot path). Spec delta on the existing windowcounter specs, not a new package name.
