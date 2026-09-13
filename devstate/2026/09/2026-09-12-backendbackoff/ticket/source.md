# backendbackoff — in-memory backoff gate for an unhealthy backend

New Go package `backendbackoff/` in `github.com/david-garcia-garcia/traefik-middleware-utilities`.
Yaegi-safe. In-memory only. No Redis, no shared state, no background goroutine.

This document is the ask. It is the output of a design exploration; every decision below was
taken deliberately and the rejected alternatives are recorded so review does not re-litigate them.

## Problem

A Traefik middleware that fronts a backend has no reusable way to stop hammering that backend
while it is unhealthy, and no way to back off progressively while it recovers. Today each
middleware reinvents some ad-hoc retry or gives up and forwards every request into a dying
upstream.

Traefik's built-in `CircuitBreaker` middleware is not usable as a library from inside a plugin,
is expression-driven (`NetworkErrorRatio`, `ResponseCodeRatio`), and has no exponential backoff.
The pieces that plugins would otherwise rewrite are exactly what this repo exists to hold.

## Scope

A per-key admission gate that observes backend outcomes reported by the caller, trips when the
observed failure ratio is too high, and then backs off exponentially before letting a probe
through.

Out of scope for this change, deliberately:

- Any shared or distributed state. This is in-memory only. A shared layer is **deferred, not
  rejected** — see `## Deferred` below. Do not add a `simpleredis` dependency to this package.
- HTTP handling. The library never reads a request, a status code, a header, or a client address.
- Classifying what counts as a failure. The caller decides and reports a boolean.
- Executing or scheduling retries. The library answers a question; it does not sleep and does not
  write a response.

## Design decisions

### Trip criterion: a saturating success-credit bucket

One `float64` of credit per key. Credit starts full at `B`.

```
  failure  →  credit -= 1
  success  →  credit += p/(1-p)     (capped at B)
  trip     →  credit <= 0
```

This is algebraically the statement "the observed failure ratio exceeded `p`, with `B` failures of
slack":

```
    F / (F + S) > p
⟺  F > p·(F + S)
⟺  F·(1 − p) > p·S
⟺  F > [ p / (1 − p) ] · S
```

Two configuration knobs fall out:

- `p` — the failure-ratio threshold that trips the gate. A percentage, so the trip point does not
  move when traffic volume changes.
- `B` — the slack, exposed to callers as **how many consecutive failures trip a completely dead
  backend**, because a dead backend produces nothing but failures and each costs exactly 1. This
  is a far more intuitive knob than a raw credit count, and it is simultaneously the noise floor:
  with `B = 5`, one failure out of three requests cannot trip the gate.

Properties this buys, which is why it was chosen:

- **Scale-invariant.** The threshold is a ratio, so one config behaves the same at 10 req/s and at
  1000 req/s.
- **Low-traffic safe with no extra machinery.** `B` is the minimum-sample guard. No separate
  "minimum requests before evaluating" setting is needed.
- **Bounded, self-forgetting memory.** Credit saturates at `B`, so a backend healthy for six hours
  sits at exactly the same credit as one healthy for the last `9B` requests. Time-to-trip is
  therefore bounded and predictable regardless of how long the healthy streak was.
- **Graceful hysteresis.** Per request the credit drains by `q − (1−q)·p/(1−p)` at observed failure
  ratio `q`, which is negative exactly when `q < p`. With `B = 5` and `p = 10%`, a dead backend
  trips in 5 requests, `q = 50%` trips in ~11, `q = 20%` in ~45, and `q = 12%` in ~224. A backend
  hovering near the threshold is tolerated for a long time; one that falls off a cliff trips
  immediately.
- **O(1) state and arithmetic per key.** No windows, no ring buffers, no second counter.

Rejected alternatives, with the reason:

- **Absolute failure rate** (a time-refilled token bucket, i.e. "N failures per second tolerated").
  Rejected because the trip point is coupled to traffic volume: a config tuned at 10 req/s trips on
  a ~1% error rate at 1000 req/s. Same config, wildly different behaviour.
- **Sliding failure ratio over a window** (two `windowcounter`-style counters, failures and totals).
  Rejected because it needs two counters, window bookkeeping, and an explicit minimum-sample guard
  to not be noise at low traffic — three mechanisms to get what saturation gives for free.
- **Consecutive failures only.** Rejected: cannot express "5% of requests are failing", which is the
  case that matters for a partially degraded backend.

### Two independent clocks

The credit decides *whether* to trip and involves no time. An exponential cooldown decides *when*
to try again and involves no outcomes. Keeping them separate is deliberate.

```
   ┌────────┐   credit <= 0   ┌────────┐
   │ CLOSED │ ──────────────▶ │  OPEN  │   cooldown = base·2^n + jitter, capped at max
   └────────┘                 └────────┘
        ▲                          │ cooldown elapsed
        │ probe ok:                ▼
        │  credit → B        ┌───────────┐
        │  n retained ───────│ HALF-OPEN │── probe fails ──▶ OPEN with n+1
        └────────────────────└───────────┘
```

- `CLOSED` admits traffic.
- `OPEN` denies traffic for the cooldown.
- `HALF-OPEN` admits a probe.
- On a probe success the gate closes and credit is restored to `B`, but the backoff exponent `n`
  is **retained**. This is what stops flapping: a backend that keeps bouncing gets progressively
  longer cooldowns even though its credit was restored each time.
- On a probe failure the gate re-opens with `n+1`, regardless of credit.
- **`n` resets to 0 once the gate has been continuously `CLOSED` for one capped (maximum) cooldown.**
  Chosen because it is time-based (so an idle-but-healthy backend is eventually forgiven), and it
  reuses the maximum-cooldown value that is already configured rather than introducing a fourth
  time knob.
- Cooldowns are jittered. Even with purely local state, N replicas that trip on the same backend
  outage will otherwise probe in lockstep and re-flood a recovering backend.

### The Report contract

- `Report` accepts the outcome of a **real backend attempt only**.
- Requests the gate itself denied MUST NOT be reported. Reporting them would poison the ratio with
  failures the backend never caused, and an open gate could never earn its way back to closed.
- What counts as a failure is the caller's classification. The library does not distinguish an
  application 500 from a dead connection, and it does not look at HTTP. This matches the existing
  packages, whose usage docs explicitly rule out reading HTTP or the client address inside the
  library.
- A probe outcome in `HALF-OPEN` is reported through the same path. A probe failure re-opens the
  gate regardless of credit.

### Storage and the hot path

- A bounded map of key → state with TTL eviction of idle keys. A key idle for longer than the TTL
  is dropped, so the backend is presumed healthy when it is next seen.
- Follow the **shape** of `tokenbucket/memory.go`: a `maxMemorySources`-style cap, `dropExpired`,
  and `dropOne` so the map cannot grow without bound.
- Do **not** import `tokenbucket`. `consumeOne` is time-refill arithmetic and `Memory.Allow`'s
  contract is "consume a token now, tell me the wait", neither of which is this job. Reusing it
  would mean shaping the problem to fit a function that does something else.
- Mutex-guarded. The admission decision is a memory read: no I/O of any kind, and no allocation
  beyond a map entry on first sight of a key.

### Lifecycle

- No ticker and no goroutine, therefore no `Sleep` / `Wake` hooks are required. `Close` at most.
- The gate SHOULD be storable in a `reclaim` table. Without that, a Traefik config reload discards
  every gate and instantly presumes every backend healthy again — losing the protection at the
  worst possible moment. State continuity across reload is the reason reclaim matters here even
  though there is no goroutine to park.

### Yaegi

- Standard library only, concrete types, no generics, no `unsafe`.
- Prove with `go test -short`, plus an interpreted (Yaegi) test following the existing per-package
  convention (`limiter_yaegi_test.go` in `tokenbucket/` and `windowcounter/`).
- Add an allocation-ceiling test on the admission path, matching the `TestAlloc*` convention in
  `simpleredis/bench_test.go`.
- No live-engine E2E and no `*_LIVE_*` environment variable: this package has no backing store, so
  it must not appear in the `e2e-redis` or `e2e-dragonfly` CI jobs.

## Affected

- New `backendbackoff/` package.
- `README.md`: a new section plus the `## Layout` block. Note that `## Why this exists` currently
  justifies the shared module as "one Redis-backed stack" and says the counter and bucket "are
  built on SimpleRedis" — that sentence does not cover a package with no Redis dependency and needs
  one line. The justification that still holds is the Yaegi constraint and the shared middleware
  consumers.
- `openspec/specs/domains.md` and `openspec/specs/map.md`: a new `std_go_backendbackoff` family.
- `knowledge/devdocs/`: a new usage packet plus its `index_std_go.md` entry.
- No change to `simpleredis/`, `windowcounter/`, `tokenbucket/`, or `reclaim/`.

## Unknowns for the propose phase to settle

- Default values for `p`, `B`, the base cooldown, the maximum cooldown, the jitter fraction, and the
  idle TTL. Pick defaults and justify them; Traefik's own `CircuitBreaker` defaults and the existing
  `tokenbucket` defaults are the reference points.
- Whether the admission call returns a `retryAfter` duration alongside the boolean, so a middleware
  can set a `Retry-After` header. The inclination is yes, mirroring how `tokenbucket.Allow` returns
  a wait and leaves the HTTP response to the caller.
- Exported names for the knobs and for the gate type itself.
- Whether the gate exposes its current state for observability, and if so how, without inviting
  callers to make decisions from a stale read.

## Deferred

A distributed layer that lets one instance learn about an unhealthy backend from its peers.
It was explored and deliberately dropped from this change to keep the first version simple. If it
is added later, the exploration concluded it must be **strictly additive**: local state stays
authoritative and always answers, the shared layer only ever accelerates learning something the
instance would have observed itself, and no Redis I/O ever happens on the request path. This is
worth a deferred follow-up note rather than silence.

## Exploration findings that must not be lost

Two grounded facts came out of reading the existing code, and both should survive into the change
so nobody re-derives them:

1. **Neither `windowcounter` nor `tokenbucket` degrades when Redis fails.** Both propagate the Redis
   error to the caller (`takeExact` returns `l.redis.Incr`'s error; `Redis.Allow` returns
   `r.redis.Eval`'s error). The buffered mode of `windowcounter` looks resilient but only
   accidentally: once a window key is warm with a non-zero `local_delta` it serves from memory and
   the background flush swallows its own errors, but at every window rollover `windowLocked`
   GET-seeds the new key and that `Take` fails. Do not cite either package as precedent for
   surviving a store outage.
2. **Request-path store I/O was rejected partly on existing evidence.**
   `knowledge/debt/2026-09-12-simpleredis-dial-circuit-breaker.md` records that during an outage
   every concurrent request rediscovers a dead Redis independently, paying the full retry ladder
   while holding a pool turn. A health gate that consulted a store synchronously would convert a
   store outage into a latency outage on every request — the exact inverse of its purpose.
