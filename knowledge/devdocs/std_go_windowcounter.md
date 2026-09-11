# Window counter

## Language

**Window counter**:
A sliding-window hit counter on Redis or Dragonfly. Package `windowcounter`. It counts hits in a whole-second window and returns allow/deny plus the sliding estimate. It is not a token bucket and not Traefik RateLimit.
_Avoid_: naming the package `ratelimit` or `tokenbucket`; remaining quota; reading HTTP or client address inside the library

**Take**:
One hit against an opaque key, a limit, and a whole-second window. Returns whether the hit is allowed and the sliding estimate after that hit. `Allow` is the same operation.
_Avoid_: remaining quota; a token or leaky bucket

**Sliding estimate**:
`current + previous × (1 − elapsed/window)` using Redis integers for this window and the previous window.
_Avoid_: fixed-window counters; counting only the current bucket

**sync_rate**:
Zero means every Take talks to Redis (`INCR` + `EXPIRE` on first hit). Greater than zero is the flush interval for local deltas (20 ms floor). Negative is invalid.
_Avoid_: in-memory-only mode; GET-then-SET of the counter

## Overview

Import `github.com/david-garcia-garcia/traefik-middleware-utilities/windowcounter`. Inject an already-`Init`ed `*simpleredis.SimpleRedis`. Prefix Redis keys in the caller. Pass `Sleep`/`Wake`/`Close` as `reclaim.Hooks` when the counter is stored in a reclaim table. Do not close the Redis client from the counter.

## How to use

- `New(redis, syncRate)` once. `sync_rate == 0` for exact counts; `> 0` to buffer.
- Call `Take(key, limit, window)` per request. Match Redis errors by `Error()` text.
- Prove with `go test ./windowcounter/...`. Live files skip without `WINDOWCOUNTER_LIVE_REDIS` / `WINDOWCOUNTER_LIVE_DRAGONFLY` or under `-short`. CI must set both.

## Pattern snippet

```go
counter, err := windowcounter.New(client, 0)
if err != nil {
	return err
}
allowed, estimated, err := counter.Take("ip:"+ip, 100, time.Minute)
if err != nil {
	return err
}
_ = estimated
if !allowed {
	return errLimited
}
```

## Key files

- `windowcounter/` — sliding-window hit counter
- `openspec/specs/std_go_windowcounter_sliding-take/spec.md`, `openspec/specs/std_go_windowcounter_sync-flush/spec.md`

## Gotchas

- Window length is whole seconds (Redis TTL is integer seconds).
- Denied Takes still increment.
- `Sleep` flushes pending deltas then stops the ticker. After `Close`, do not start a new flush ticker. The SimpleRedis client is still the caller's.
- EVAL scripts must list keys in `KEYS` (Dragonfly).
