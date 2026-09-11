# Leaky bucket

## Language

**Leaky bucket**:
A classic water-meter clock. Package `leakybucket`. Pour at `Add`/`Take`, drain at `leak` water per second, refuse a pour that would exceed `capacity`. It is not a Traefik token bucket and not a sliding-window hit counter.
_Avoid_: naming the package `ratelimit` or `bucket`; mixing this clock with `tokenbucket` or `windowcounter`; storing remaining room instead of water

**Add**:
Pour `n` units of water against an opaque key. Returns whether the pour is allowed, the water after leak (and after the pour when allowed), and until-not-full. The caller 429s. The library does not sleep.
_Avoid_: HTTP 429 inside the library; sourceCriterion; prefixing a scheme inside the package

**Take**:
Add with `n` equal to 1.
_Avoid_: a second pour API with a default n; remaining quota

**Level**:
Apply leak to now and return the water. It does not pour. It does not return until-not-full.
_Avoid_: treating Level as Take; HTTP

**until-not-full**:
Duration until leak brings water to `capacity - 1`. Zero when `water + 1 <= capacity`.
_Avoid_: Traefik `maxDelay`; sleeping inside the library

**sync_rate**:
Zero means every Add/Take/Level is EVAL (leak then maybe add). Greater than zero is the flush interval for local pours (20 ms floor). Negative is invalid.
_Avoid_: GET-then-SET of `{water, last}`; replica HSET of the hash

## Overview

Import `github.com/david-garcia-garcia/traefik-middleware-utilities/leakybucket`. For Redis, inject an already-`Init`ed `*simpleredis.SimpleRedis`. Prefix keys in the caller. Pass Redis `Sleep`/`Wake`/`Close` as `reclaim.Hooks` when the meter is stored in a reclaim table. Do not close the Redis client from the limiter.

## How to use

- `NewMemory(leak, capacity, ttl)` or `NewRedis(client, leak, capacity, syncRate, ttl)` once. `sync_rate == 0` for exact EVAL; `> 0` to buffer.
- Call `Take(key)` (or `Add(key, n)`) per event. Call `Level(key)` to observe water without pouring. Match Redis errors by `Error()` text.
- Prove with `go test ./leakybucket/...`. Live files skip without `LEAKYBUCKET_LIVE_REDIS` / `LEAKYBUCKET_LIVE_DRAGONFLY` or under `-short`. CI must set both.

## Pattern snippet

```go
limiter, err := leakybucket.NewRedis(client, 1, 50, 0, 2*time.Second)
if err != nil {
	return err
}
allowed, level, untilNotFull, err := limiter.Take("ip:" + ip)
if err != nil {
	return err
}
if !allowed {
	return errLimited
}
_ = level
_ = untilNotFull
```

## Key files

- `leakybucket/` — I.371 water meter
- `openspec/changes/add-leakybucket/specs/std_go_leakybucket_pour/spec.md`, `openspec/changes/add-leakybucket/specs/std_go_leakybucket_sync-flush/spec.md`

## Gotchas

- `leak <= 0`, `capacity <= 0`, or `ttl < 1s` fails New. `n < 1` fails Add. Nil redis or negative `sync_rate` fails NewRedis.
- Overflow denies and does not pour. Empty water is 0 (room up to `capacity`).
- EVAL scripts must list keys in `KEYS` (Dragonfly). Do not use `table.maxn`. Go must not `SET`/`HSET` the hash.
- Two limiter instances on one Redis key share water. Buffered mode may over-allow by at most one sync interval of replica pours.
- `Sleep` flushes pending pours then stops the ticker. After `Close`, do not start a new flush ticker. The SimpleRedis client is still the caller's. Memory has no reclaim hooks.
