# Window counter

## Language

**Window counter**:
A sliding-window hit counter on Redis or Dragonfly. Package `windowcounter`. It counts hits in a whole-second window and returns a `Status` (allow/deny, the sliding estimate, and rate-limit values). It is not a token bucket and not Traefik RateLimit.
_Avoid_: naming the package `ratelimit` or `tokenbucket`; reading HTTP or client address inside the library

**Peek**:
Already-used occupancy: whether the sliding estimate is at or under limit, without recording a hit. Same arguments, formula, Redis keys, and store as Take. Not whether the next Take would admit. `Allow` is still Take, not Peek.
_Avoid_: treating Peek as Allow; treating Peek allowed as a reservation for a later Take; a second consistency mode

**Take**:
One hit against an opaque key, a limit, and a whole-second window. Returns a `Status` whose `Estimated` is the sliding estimate after that hit. `Allow` is the same operation.
_Avoid_: a token or leaky bucket

**Status**:
The outcome of one Take, Peek, or Allow: `Allowed`, `Estimated`, and `Limit`, `Remaining`, `Period` (the window), `Reset`, `RetryAfter` for rate-limit headers. A value; the library writes no header.
_Avoid_: rate-limit headers inside the library; reading `Reset` as the time until the full limit is back

**Remaining**:
How many more Takes fit now: `floor(max(limit - Estimated, 0))`.
_Avoid_: `Estimated`; a reservation for later Takes

**Sliding estimate**:
`current + previous × (1 − elapsed/window)` using Redis integers for this window and the previous window.
_Avoid_: fixed-window counters; counting only the current bucket

**sync_rate**:
Zero means every Take talks to Redis (one EVAL: `INCR` then `EXPIRE` if `PTTL < 0`) and that call's Redis error is returned. Greater than zero is the flush interval for local deltas (20 ms floor). During a Redis outage, buffered Take and Peek keep this instance's `limit` with a nil error; Redis is retried one `sync_rate` after the last failure. Negative is invalid.
_Avoid_: in-memory-only mode; GET-then-SET of the counter

## Overview

Import `github.com/david-garcia-garcia/traefik-middleware-utilities/windowcounter`. Inject an already-`Init`ed `*simpleredis.SimpleRedis`. Prefix Redis keys in the caller. Pass `Sleep`/`Wake`/`Close` as `reclaim.Hooks` when the counter is stored in a reclaim table. Do not close the Redis client from the counter.

## How to use

- `New(redis, syncRate)` once. `sync_rate == 0` for exact counts; `> 0` to buffer.
- Take, Peek, and Allow return a `Status`: `Allowed` and `Estimated` (the sliding estimate), plus `Limit`, `Remaining` (Takes that still fit now), `Period` (the window), `Reset`, and `RetryAfter` for rate-limit headers. Formatting is the middleware's job. On a non-nil error the `Status` is the zero value.
- Call `Peek(ctx, key, limit, window)` to observe occupancy without counting a hit. Do not treat Peek allowed as a reservation for a later Take. Call `Take` when the hit should occupy the window (for example after a backend failure). Pass `req.Context()` on the request path; pass `context.Background()` when there is no deadline.
- Match Redis miss with `simpleredis.IsMiss` (works through wrapping). Exact mode (`sync_rate == 0`) returns the Redis error on that Take/Peek. Buffered mode during a Redis outage returns a nil error and keeps this instance's `limit` (per-node cap). Combined admits across instances can exceed the global `limit`. Other Redis errors still propagate by `Error()` text until those callers convert.
- Buffered Peek (`sync_rate > 0`) is a memory read after the first sight of a window key. It does not GET Redis on every call while `local_delta` stays 0. Buffered Take GETs the previous-window key when `local_delta` is 0 the first time it uses that key as previous, then at most once per `sync_rate` (another instance may flush into it after this instance did). Not once per Take. When previous still has `local_delta` greater than 0, Take keeps memory and MUST NOT GET that key. Exact Peek (`sync_rate == 0`) GETs current and previous every call.
- Prove with `go test -short ./windowcounter/...`. Live Redis/Dragonfly is `zzz_limiter_e2e_test.go` / `zzz_limiter_yaegi_e2e_test.go` / `zzz_limiter_flush_batch_e2e_test.go` (see `knowledge/devdocs/std_go_test-suites.md`).
- Benchmark with `go test -run '^$' -bench . -benchtime 1s -count 5 ./windowcounter/`: `zzz_bench_test.go` (compiled Take at `sync_rate` 0 and 20ms plus `BenchmarkFlush` of 1000 dirty keys, fake Redis with a warm script cache, `redis-cmds/op`), `zzz_bench_yaegi_test.go` (interpreted Take, plus `BenchmarkYaegiFlush` of 1000 dirty keys through a test-only `zz_bench_hooks.go` written into the interpreter's GOPATH copy of the package), `zzz_bench_e2e_test.go` (`-bench Live`, needs `WINDOWCOUNTER_LIVE_*`, skips under `-short`). CI does not run benchmarks. Results and machine: `README.md` `## Rate limiters compared`.

## Pattern snippet

```go
counter, err := windowcounter.New(client, 0)
if err != nil {
	return err
}
ctx := req.Context()
status, err := counter.Peek(ctx, "ip:"+ip, 100, time.Minute)
if err != nil {
	return err
}
if !status.Allowed {
	return errLimited // 429 with Retry-After from status.RetryAfter
}
// call backend; on failure:
_, err = counter.Take(ctx, "ip:"+ip, 100, time.Minute)
```

## Key files

- `windowcounter/` — sliding-window hit counter; `status.go` builds the `Status` and the retry-after search
- `windowcounter/zzz_bench_test.go`, `zzz_bench_yaegi_test.go`, `zzz_bench_e2e_test.go` — compiled, interpreted, and live benchmarks
- `openspec/specs/std_go_windowcounter_sliding-take/spec.md`, `openspec/specs/std_go_windowcounter_sync-flush/spec.md`

## Gotchas

- Window length is whole seconds (Redis TTL is integer seconds). Take and Peek reject a window that is shorter than one second or not an integer number of seconds (for example 1500ms); they do not truncate into second buckets.
- Denied Takes still increment. Peek does not. A client that keeps hammering after a deny pushes its own `RetryAfter` further; it is not compensated.
- `Reset` is the time to the end of the current epoch-aligned window, with the fraction of a second of the clock reading. The previous window still weighs at that instant, so the full `limit` is not back at `Reset`.
- `RetryAfter` is 0 when `Allowed` (Peek too). When denied it is the time to the first whole second at which one more Take, with no other traffic, is admitted, checked with the same weight function as Take; at most two windows. With `limit < 1` no Take is ever admitted and `RetryAfter` equals `Reset`.
- `Remaining` is `floor(max(limit - Estimated, 0))`. `Estimated` is not `Remaining`.
- Buffered `Status` fields come from this instance's view (`redis_known + local_delta`), the same counts that decide `Allowed`, including the per-node cap during an outage. No GET or EVAL is added to fill them.
- After occupancy equals limit, Peek allows and the next Take denies and still increments.
- `Sleep` flushes pending deltas then stops the flush timer. Concurrent `Sleep` calls each wait for the in-flight flush; one Sleep does not end another's stop. Concurrent `Wake` during any pending Sleep does not start a timer and every Sleep still returns. After every Sleep returns, `Wake` starts the timer again when `sync_rate` is greater than zero. After `Close`, do not start a new flush timer. The SimpleRedis client is still the caller's.
- Buffered flush uses `time.AfterFunc` because this package is interpreted under Yaegi. Do not put `go` plus `select` on a ticker and a stop channel in a limiter method; Yaegi v0.16.1 can miss the close and `Sleep`/`Close` hang.
- EVAL scripts must list keys in `KEYS` (Dragonfly).
- Buffered flush sends one EVAL per batch of up to 256 window keys (`flushBatchKeys`; the spec ceiling is 1024). Each key runs under `redis.pcall` and gets its own reply slot: an applied key leaves pending, a key whose INCRBY failed keeps its `local_delta` for the next flush, and the first failure is stored. A reply without one slot per key, or a whole-EVAL error, fails every key of that batch. Do not drop `pcall`: Redis does not undo the keys a script already applied, so a mid-script error would make the retry count them twice.
- A reply lost after the engine applied a batch can re-apply up to 256 keys on retry. It is the same accepted EVAL double-apply as `std_go_simpleredis.md`, now per batch instead of per key.
- A multi-key flush EVAL needs every key on one node. A Redis Cluster engine or cluster proxy would answer CROSSSLOT; SimpleRedis does not support Redis Cluster (no hash tags, no `MOVED`).
- Exact Take expires only when the key has no TTL (`PTTL < 0`). It does not refresh TTL on every hit and does not `DEL` on expire failure.
- Buffered Take/Peek during a Redis outage return a nil error and admit until this instance's `limit`. Exact mode still returns Redis errors. Do not check `err` to fail closed on the buffered path.
- A stored buffered outage skips Redis for one `sync_rate`, then the next Take/Peek GET retries. Keys seeded at zero during the outage are GET again on that retry, so a Peek-only limiter recovers without a flush.
- Buffered GET and flush EVAL run without the limiter mutex. A slow Redis round trip on one opaque key must not stall Take on another key.
