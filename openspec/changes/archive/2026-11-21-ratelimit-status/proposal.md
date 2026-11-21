## Why

The middleware that calls `tokenbucket` and `windowcounter` cannot set `Retry-After` or `X-RateLimit-Limit` / `-Remaining` / `-Period` / `-Reset` on its responses, because both limiters return only `(allowed, wait)` and `(allowed, estimated)`. The data already exists inside each call (tokens left in the bucket, current and previous window counts, the window start) and is thrown away. The requester decided to change the existing signatures to return one status value instead of adding parallel methods.

## What Changes

- **BREAKING** `tokenbucket.Memory.Allow` and `tokenbucket.Redis.Allow` return `(tokenbucket.Status, error)` instead of `(bool, time.Duration, error)`.
- **BREAKING** `windowcounter.Limiter.Take`, `Peek`, and `Allow` return `(windowcounter.Status, error)` instead of `(allowed bool, estimated float64, err error)`.
- Both `Status` types share the fields `Allowed`, `Limit`, `Remaining`, `Period`, `Reset`, `RetryAfter`. `tokenbucket.Status` adds `Delay` (the wait of an admitted consume when `maxDelay > 0`); `windowcounter.Status` adds `Estimated` (today's sliding estimate, unchanged).
- `tokenbucket`: `Limit = burst`, `Remaining` = whole tokens left after this Allow, `Period = burst / rate` (time to refill an empty bucket; the library has no configured period), `Reset` = time until the bucket is full again. Today's single wait splits into `Delay` (admitted) and `RetryAfter` (denied). Duration fields saturate at the maximum duration instead of overflowing to negative.
- `tokenbucket.Redis` reads the third field the Lua script already returns (tokens left). A value that is not a finite number is a new error, `tokenbucket: eval tokens is not a number`, never a status with a nil error. The script itself does not change.
- `windowcounter`: `Limit = limit`, `Period = window`, `Remaining` = Takes that still fit now, `Reset` = time until the end of the current epoch-aligned fixed window (requester's decision), `RetryAfter` = time until the first whole second at which a Take with no further traffic would be admitted (0 when allowed; equal to `Reset` when `limit < 1`). In buffered mode every field comes from this instance's view, the same one that already decides `Allowed`.
- Neither package writes HTTP. Header formatting (integer seconds, HTTP date) stays with the middleware.
- Every caller in this repo migrates: compiled tests, Yaegi probe sources, benchmarks, `README.md`, and the usage snippets in `knowledge/devdocs/std_go_tokenbucket.md` and `std_go_windowcounter.md`. Benchmarks are measured before and after, compiled and under Yaegi.

## Capabilities

### New Capabilities

- `std_go_tokenbucket_status`: what each field of `tokenbucket.Status` means after an Allow (limit, remaining, period, reset, delay vs. retry-after), duration saturation, and memory/Redis agreement of those fields.
- `std_go_windowcounter_status`: what each field of `windowcounter.Status` means after a Take or Peek (limit, remaining, period, fixed-window reset, closed-form retry-after, `limit < 1`), and that buffered mode reports this instance's view.

### Modified Capabilities

- `std_go_tokenbucket_allow`: Allow returns a `Status` and an error; the admit/wait/deny rule is unchanged but the wait is reported as `Delay` or `RetryAfter`; the Redis store reads the tokens field and rejects a non-finite one.
- `std_go_windowcounter_sliding-take`: Take, Peek, and Allow return a `Status` and an error; the sliding estimate is the `Estimated` field and is not `Remaining`.

## Impact

- **Code**: new `tokenbucket/status.go`, `windowcounter/status.go`; `tokenbucket/memory.go`, `tokenbucket/redis.go`, `tokenbucket/clock.go` (new error and saturating conversion); `windowcounter/limiter.go` (`slidingAt`, the four take/peek paths, shared weight function). `tokenbucket/lua.go` and the admit rule of both limiters do not change.
- **Tests**: every `_test.go` in `tokenbucket/` (78 call lines in 16 files) and `windowcounter/` (110 lines in 15 files), including Yaegi probe sources and benchmarks; new status tests per package.
- **Docs**: `README.md` (`## Window counter`, `## Token bucket` examples, `## Rate limiters compared` numbers if they move), `knowledge/devdocs/std_go_tokenbucket.md` and `std_go_windowcounter.md` (snippets, gotchas).
- **Callers outside this repo**: the middleware breaks at compile time and must migrate; it cannot be enumerated from here.
- **Unchanged**: `allowScript` and its digest, `simpleredis/`, `backendbackoff/`, Traefik Hub semantics (`denyOnError`, `limit 0`, `sourceCriterion`), `go.mod`, CI.
