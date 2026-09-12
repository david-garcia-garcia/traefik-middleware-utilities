# Window counter

## Language

**Window counter**:
A sliding-window hit counter on Redis or Dragonfly. Package `windowcounter`. It counts hits in a whole-second window and returns allow/deny plus the sliding estimate. It is not a token bucket and not Traefik RateLimit.
_Avoid_: naming the package `ratelimit` or `tokenbucket`; remaining quota; reading HTTP or client address inside the library

**Peek**:
The same observation as Take without recording a hit. Same arguments, formula, Redis keys, and store. `Allow` is still Take, not Peek.
_Avoid_: remaining quota; treating Peek as Allow; a second consistency mode

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
- Call `Peek(key, limit, window)` to observe the sliding estimate without counting a hit. Call `Take` when the hit should occupy the window (for example after a backend failure).
- Match Redis miss with `simpleredis.IsMiss` (works through wrapping). Other Redis errors still propagate by `Error()` text until those callers convert.
- Buffered Peek (`sync_rate > 0`) is a memory read after the first sight of a window key. It does not GET Redis on every call while `local_delta` stays 0. Exact Peek (`sync_rate == 0`) GETs current and previous every call.
- Prove with `go test -short ./windowcounter/...`. Live Redis/Dragonfly is `limiter_e2e_test.go` / `limiter_yaegi_e2e_test.go` (see `knowledge/devdocs/std_go_test-suites.md`).

## Pattern snippet

```go
counter, err := windowcounter.New(client, 0)
if err != nil {
	return err
}
allowed, estimated, err := counter.Peek("ip:"+ip, 100, time.Minute)
if err != nil {
	return err
}
if !allowed {
	return errLimited
}
// call backend; on failure:
_, _, err = counter.Take("ip:"+ip, 100, time.Minute)
```

## Key files

- `windowcounter/` — sliding-window hit counter
- `openspec/specs/std_go_windowcounter_sliding-take/spec.md`, `openspec/specs/std_go_windowcounter_sync-flush/spec.md`

## Gotchas

- Window length is whole seconds (Redis TTL is integer seconds).
- Denied Takes still increment. Peek does not.
- `Sleep` flushes pending deltas then stops the ticker. After `Close`, do not start a new flush ticker. The SimpleRedis client is still the caller's.
- EVAL scripts must list keys in `KEYS` (Dragonfly).
