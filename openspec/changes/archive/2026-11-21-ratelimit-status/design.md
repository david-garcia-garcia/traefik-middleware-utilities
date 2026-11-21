## Context

See `proposal.md` — Why. Explore (`devstate/2026/11/2026-11-21-ratelimit-status/explore.md`) read every unit, ran the baseline tests, and probed the Lua tokens field on Redis 7 and Dragonfly v1.40.2. This design fixes the shapes that the specs leave to implementation.

Current state that constrains the approach:

- `tokenbucket/memory.go` `Allow` and `tokenbucket/redis.go` `Allow` each map `waitMicro` to `(allowed, wait)` with the same three lines (duplicated). `consumeOne` (`tokenbucket/clock.go`) already returns the tokens kept after the consume (refunded on deny); Memory stores them in `entry.tokens` and drops them from the return. `Redis.Allow` checks for three Eval fields and parses only `values[1]`.
- `allowScript` returns `{"true", tostring(wait), tostring(tokens)}`. Measured in explore: the tokens text is always a finite decimal `strconv.ParseFloat` accepts (`4`, `4.0`, `-1.0`, `0.3`, `1e+15`); Redis formats with 14 significant digits, Dragonfly appends `.0`. On deny it is the refunded value and can be negative.
- `time.Duration(waitMicro * float64(time.Microsecond))` overflows to a negative value for large waits (measured in explore: `-2562047h47m16.854775808s`). Admission compares `waitMicro` as a float, so `allowed` is safe today; only the duration is wrong.
- `windowcounter/limiter.go` `slidingAt` reads the clock once, computes the window start in whole seconds and `weight = 1 - float64(elapsed)/(float64(windowSec)*float64(time.Second))`, and does not keep `now` or the window end. `takeExact`, `takeBuffered`, `peekExact`, `peekBuffered` hold `current` and `previous` separately and return only `current + previous × weight`.
- Both packages run compiled and interpreted under Yaegi v0.16.1. Returning a struct by value already works interpreted (`slidingAt` returns `slidingWindow`, covered by `TestYaegi_TakeUntilDeny`).
- No Go importer of either package exists outside the two packages in this repo. The middleware that calls them lives elsewhere.

## Goals / Non-Goals

**Goals:**

- One `Status` per package carrying the data for `Retry-After` and `X-RateLimit-Limit` / `-Remaining` / `-Period` / `-Reset`, computed from values each call already has.
- The admit/deny decision of both limiters stays bit-for-bit the same.
- No extra Redis command, no new dependency, stdlib only.

**Non-Goals:**

- Header names, formats, or units (integer seconds, HTTP date): the middleware owns them.
- A shared `Status` type across packages, compatibility wrappers, or parallel methods.
- Changing `allowScript`, `consumeOne`, the sliding formula, the buffered sync rules, or validating `limit`.
- Traefik Hub semantics (`denyOnError`, `limit 0 = unlimited`, `sourceCriterion`).

## Decisions

### D1. One `Status` type per package, in its own file

`tokenbucket/status.go` and `windowcounter/status.go`, each with the type, its comment, and the one function that builds it. Common fields use the same names and types in both packages:

| Field | Type | tokenbucket | windowcounter |
|---|---|---|---|
| `Allowed` | `bool` | wait ≤ maxDelay (µs) | estimate ≤ limit |
| `Limit` | `int64` | `burst` | `limit` |
| `Remaining` | `int64` | whole tokens left | Takes that still fit |
| `Period` | `time.Duration` | `burst / rate` | `window` |
| `Reset` | `time.Duration` | until bucket full | until fixed window end |
| `RetryAfter` | `time.Duration` | wait, when denied | first admitting second, when denied |
| `Delay` | `time.Duration` | wait, when admitted | — |
| `Estimated` | `float64` | — | sliding estimate (today's second return) |

Methods return `(Status, error)` by value; error paths return `Status{}`.

Rejected (explore): a shared type in a common package (new cross-package dependency, one more GOPATH copy per Yaegi test); more loose return values (the requester chose a struct); `AllowStatus`-style parallel methods (the requester ruled them out).

### D2. `tokenbucket`: one status builder for both stores

`newStatus(clock clockConfig, tokens, waitMicro float64) Status` in `status.go`. `Memory.Allow` and `Redis.Allow` call it with the tokens kept after `consumeOne` / the script and the wait. It replaces the duplicated `allowed`/`wait` mapping in `memory.go` and `redis.go`, so both stores compute every field the same way.

- `Allowed = waitMicro <= float64(clock.maxDelay.Microseconds())` — the same expression as today.
- Admitted: `Delay` = wait, `RetryAfter` = 0. Denied: `RetryAfter` = wait, `Delay` = 0. `Delay + RetryAfter` is today's `wait`.
- `Remaining = min(floor(max(tokens, 0)), burst)`. The `min` keeps `Remaining ≤ Limit` when `float64(burst)` rounds up for a huge burst.
- `Period` = `burst × 1s / rate`, computed once per limiter (`refillPeriod`, stored in `clockConfig.period`); multiplying before dividing keeps a whole result exact (5 tokens at 10/s is 500ms, where `burst / limitPerMicro` gives 499.999999ms). `Reset` = `max(burst − tokens, 0) × 1s / rate`.

`RetryAfter = wait`, not `wait − maxDelay`: that is the delay Traefik's own limiter puts in `Retry-After` (`knowledge/research/ext_traefik_ratelimiter_token-bucket/`), and retrying then finds the token without waiting. `wait − maxDelay` is the earliest admitting instant; it is documented, not exposed.

`Period` is derived because the library never receives Traefik's `average`/`period`. Asking the caller for them was rejected in explore (new configuration nobody asked for). With this mapping `Limit / Period = rate`.

### D3. Saturating microseconds-to-duration conversion (`tokenbucket`)

`durationFromNanos(nanos float64) time.Duration` in `status.go`: not positive (including NaN) → 0; at or above `math.MaxInt64` nanoseconds → `math.MaxInt64`; otherwise `time.Duration(nanos)`. The wait converts as today (`waitMicro × 1µs`), so in-range values do not move. `refillPeriod` calls it for `Period`; `newStatus` writes the same rule inline for `Reset` and the wait (`Delay` / `RetryAfter`), because each interpreted call costs about a microsecond under Yaegi. The admit decision keeps comparing the float `waitMicro`, so saturation cannot change `Allowed` (`allow_wait_duration_overflow_test.go`, `allow_maxdelay0_wait_trunc_test.go` stay green).

`windowcounter` needs no helper: `Period` is the caller's `window`, and `Reset` / `RetryAfter` come from `time.Time.Sub`, which already saturates.

### D4. `tokenbucket.Redis` reads the tokens field

After the existing wait parse, parse `values[2]` with the same rule (`strconv.ParseFloat`, reject NaN and ±Inf) and return `errEvalTokens` (`tokenbucket: eval tokens is not a number`, declared next to `errEvalWait` in `clock.go`) on failure. Wait is checked first, so a reply with both fields bad still returns `errEvalWait` as today. `allowScript` and its digest do not change (rejected: returning more fields or `%.17g` formatting — not needed per the probe, and it would change the script digest and `std_go_tokenbucket_lua-eval`). The fake's existing `setEvalReply` cases already put `"0"` in the third field.

Memory/Redis parity: `Allowed`, `Limit`, `Period` equal; `Delay`/`RetryAfter` same class (as today's wait); `Remaining`/`Reset` equal while tokens stay whole numbers. Redis's 14-digit text can move `Remaining` across an integer only when the fraction is ≥ 0.99999999999995; parity tests use whole-number or far-from-edge sequences.

### D5. `windowcounter`: the sliding window carries the clock reading and the window end

`slidingWindow` gains `now time.Time`, `windowSec int64`, `windowStart int64` (Unix seconds). The weight expression moves into one function, `weightAt(elapsedSec, windowSec int64) float64`, with the exact expression `slidingAt` uses today; `slidingAt` and the retry-after search both call it, so the retry predicate is the admit predicate.

The four paths (`takeExact`, `takeBuffered`, `peekExact`, `peekBuffered`) receive the `slidingWindow` value and `limit` instead of the loose `currentKey`/`previousKey`/`ttlSec`/`expireAt`/`weight` parameters (one cluster of facts that always travel together), keep reading Redis and the buffer exactly as today, and end with `newStatus(sliding, limit, current, previous)` (`sliding` carries the window length). `Estimated` stays `float64(current) + float64(previous)*sliding.weight` — the same expression and operand order, so `Allowed` does not move.

`newStatus` in `windowcounter/status.go`:

- `Limit = limit`, `Period = window`, `Remaining = floor(max(limit − Estimated, 0))`.
- `Reset = time.Unix(windowStart + windowSec, 0).Sub(now)`, with `now` from the same `slidingAt` reading. Not rounded.
- `RetryAfter = 0` when `Allowed`. When denied and `limit < 1`: `RetryAfter = Reset`. Otherwise the closed form below.

### D6. `windowcounter` retry-after: closed form, checked with the admit predicate

With `C` = current count as this call saw it (Take/Allow include their own hit; Peek does not), `P` = previous count, `W` = `windowSec`, `L` = `limit`, the next Take is admitted at elapsed second `e` when `(C + 1) + P × weightAt(e, W) ≤ L`.

1. Same window, if `C + 1 ≤ L` and `P > 0`: `e = ceil(W × (1 − (L − C − 1) / P))`. If `e < W`, the retry second is `windowStart + e`.
2. Otherwise the next window (previous becomes `C`, current 0): `e = ceil(W × (1 − (L − 1) / C))` clamped to `[0, W]` (`e = 0` when `C = 0`); retry second `windowStart + W + e`.
3. Check the candidate with the same predicate and `weightAt`, stepping `e` by one second while the float rounding of the closed form disagrees (a bounded handful of steps).
4. `RetryAfter = time.Unix(retrySecond, 0).Sub(now)`. A denied call at elapsed second `s` cannot be admitted at `s` (the next Take adds one hit), so `retrySecond > s` and `RetryAfter > 0`; the next-window bound gives `RetryAfter ≤ 2W`.

Computed only on a deny, O(1). Rejected (explore): a per-second scan (O(window) per deny, expensive under Yaegi); truncating `Reset`/`RetryAfter` to whole seconds (the window end is already a whole second; truncating `now` overstates by up to 1 s — formatting is the middleware's job).

Denied Takes keep incrementing (`std_go_windowcounter_sliding-take`), so a client that keeps hammering pushes its own retry second further; that is documented in the devdocs gotchas, not compensated.

### D7. Peek, buffered mode, and outage

Peek keeps its occupancy meaning: `Allowed` is `Estimated ≤ limit`, `Remaining` uses Peek's `Estimated`, `RetryAfter` is 0 whenever `Allowed` (at occupancy exactly `limit`: `Allowed` true, `Remaining` 0, `RetryAfter` 0; the next Take denies, as today). In buffered mode every field comes from `redis_known + local_delta` and the previous term the path already used, including during an outage (per-node cap, nil error). No GET or EVAL is added to improve the numbers.

### D8. Migrating callers in this repo

Mechanical: `allowed, wait, err :=` → `status, err :=` and `allowed, estimated, err :=` → `status, err :=`, then `status.Allowed`, `status.Estimated`, and `status.Delay + status.RetryAfter` where a test asserted today's raw wait. That covers compiled tests, the interpreted probe sources inside `*_yaegi_test.go` and `bench_yaegi_test.go`, live tests, benchmarks (keep a package-level sink so the call is not optimized away), `README.md`, and the devdocs snippets. `windowcounter.Allow` stays an alias of `Take`.

### D9. Measure compiled and Yaegi

Before (dest `ee9f5c11`) and after, same machine and session: `go test -run '^$' -bench . -benchtime 1s -count 5 ./tokenbucket/ ./windowcounter/` (includes `BenchmarkYaegi*`). Report ns/op, allocs/op, and `redis-cmds/op` (must not change). If the medians move, update `README.md` `## Rate limiters compared` with the new numbers and the commit they came from.

## Risks / Trade-offs

- [The middleware breaks at compile time] → Requested (Desired 4). The PR and the release notes name the new signatures and the `Delay + RetryAfter` equivalence for today's `wait`.
- [Yaegi cost of building a larger struct per call] → D9 measures it; the fields are O(1) arithmetic and `RetryAfter` in `windowcounter` runs only on deny.
- [Float rounding in the retry closed form] → D6 step 3 checks with the admit predicate itself; tests pin the same-window and next-window cases.
- [Redis tokens text has 14 significant digits] → Documented parity rule (D4); `Allowed` does not depend on it.
- [`windowcounter.Reset` is not "full quota back"] → Requester's decision (Desired 5); stated in the spec and the devdocs.
- [Buffered values are this instance's view] → Same view that decides `Allowed` today; documented next to the per-node cap.
- [`tokenbucket.Period` is derived, not Traefik's configured `period`] → Documented on the field; `Limit / Period = rate` holds.

## Migration Plan

Single breaking change; no data or Redis key format changes, so rollback is reverting the commit and the callers. `allowScript` is unchanged, so mixed versions sharing a Redis bucket keep the same meaning.

## Open Questions

- Release tag. The module is at `v1` (latest tag `v1.0.9`) and this change breaks exported signatures; under Go module semver a breaking change normally needs a `/v2` module path. The requester accepted the break but did not choose a version. The release step decides the tag; it does not change the specs or the tasks. Resolved at archive: `v1.0.10`, the repo convention for breaking changes (patch tag, no `/v2` path, as #101); the conductor tags after merge.
