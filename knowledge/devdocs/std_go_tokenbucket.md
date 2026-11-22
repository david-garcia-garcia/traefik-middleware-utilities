# Token bucket

## Language

**Token bucket**:
A Traefik RateLimit clock. Package `tokenbucket`. Refill at `rate` tokens per second, cap at `burst`, consume 1 per Allow. It is not a sliding-window hit counter and not Kong `sync_rate`.
_Avoid_: naming the package `ratelimit` or `bucket`; mixing this clock with `windowcounter`

**Allow**:
One consume against an opaque key. Returns a `Status`. The library does not sleep and does not choose HTTP.
_Avoid_: HTTP 429 inside the library; sourceCriterion; prefixing `rate:` inside the package; deny-on-error inside Allow

**Status**:
The outcome of one Allow: `Allowed`, the wait split into `Delay` (allowed) and `RetryAfter` (denied), and `Limit` (burst), `Remaining`, `Period`, `Reset` for rate-limit headers. A value; the library writes no header.
_Avoid_: rate-limit headers inside the library; a single wait return; reading `Period` as Traefik's configured `period`

**Remaining**:
The whole tokens the bucket keeps after this Allow (after the refund when denied), never above `Limit`, and 0 while the bucket is in debt.
_Avoid_: tokens before the consume; a negative count

**Memory**:
The in-process token-bucket store (`NewMemory`). One map entry per opaque source, cap 65536, expiry-ordered list. It is not Traefik's `ttlmap` and not the Redis store.
_Avoid_: ttlmap; treating Redis keys as Memory entries

## Overview

Import `github.com/david-garcia-garcia/traefik-middleware-utilities/tokenbucket`. For Redis, inject an already-`Init`ed `*simpleredis.SimpleRedis`. Prefix keys in the caller. Compute `rate` from Traefik `average/period`. Pass `maxDelay` and `ttl`. `ttl` is a minimum idle lifetime: the library raises it to the whole seconds needed to refill `burst` at `rate`, including `maxDelay`, so an expired bucket is already full (traefik/traefik#13957). Traefik's own formula (`2s` when rate ≥ 1, else `1 + int(1/rate)`) may be shorter; pass it anyway. Do not close the Redis client from the limiter.

## How to use

- `NewMemory(rate, burst, maxDelay, ttl)` or `NewRedis(client, rate, burst, maxDelay, ttl)` once.
- Call `Allow(ctx, key)` per request. Pass `req.Context()` on the request path; pass `context.Background()` when there is no deadline.
- Match a Redis outage with `errors.Is` against `simpleredis.ErrUnreachable` or `simpleredis.ErrTimeout` (`IsUnreachable` is the unreachable family only; it does not match `ErrTimeout`). Display text stays `redis:unreachable` / `redis:timeout`.
- On a Redis outage `Allow` returns that error and the zero `Status`. `Allowed` is false because Status is zero, not because Allow denied. Match `err` before reading `Allowed`. Fail-open or fail-closed is the middleware's decision; this library does not pick an HTTP code. The snippet that returns `err != nil` is fail-closed.
- Read the returned `Status`: deny on `!Allowed` and put `RetryAfter` in `Retry-After`; sleep `Delay` when allowed to keep Traefik's delayed admit; take `Limit`, `Remaining`, `Period`, and `Reset` for `X-RateLimit-*`. Formatting (whole seconds, HTTP date) is the middleware's job. On a non-nil error the `Status` is the zero value. Map `context.Canceled` to HTTP 499 in the middleware and skip the wait when wait is 0; this package writes no HTTP (traefik/traefik#14173).
- Prove with `go test -short ./tokenbucket/...`. Live Redis/Dragonfly is `limiter_e2e_test.go` / `limiter_yaegi_e2e_test.go` and the `traefik_issue*_e2e_test.go` files (see `knowledge/devdocs/std_go_test-suites.md`; they need `TOKENBUCKET_LIVE_*` and skip under `-short`). Regressions for defects reported against Traefik are `traefik_issue<N>_test.go` / `traefik_issue<N>_e2e_test.go`, listed under Key files.
- Benchmark with `go test -run '^$' -bench . -benchtime 1s -count 5 ./tokenbucket/`: `bench_test.go` (compiled, fake Redis with a warm script cache, `redis-cmds/op`), `bench_yaegi_test.go` (interpreted: hot key, 10k rotating sources, new key at a full table; it writes a test-only `zz_bench_hooks.go` into the interpreter's GOPATH copy of the package to prefill sources without `Allow`), `bench_e2e_test.go` (`-bench Live`, needs `TOKENBUCKET_LIVE_*`, skips under `-short`). CI does not run benchmarks. Results and machine: `README.md` `## Rate limiters compared`.

## Pattern snippet

```go
limiter, err := tokenbucket.NewRedis(client, 100, 50, 5*time.Millisecond, 2*time.Second)
if err != nil {
	return err
}
ctx := req.Context()
status, err := limiter.Allow(ctx, "ip:"+ip)
if err != nil {
	return err // 499 on context.Canceled; fail-closed on an outage (fail-open would admit). This package writes no HTTP
}
if !status.Allowed {
	return errLimited // 429 with Retry-After from status.RetryAfter
}
time.Sleep(status.Delay)
```

## Key files

- `tokenbucket/` — Traefik token bucket; `status.go` builds the `Status` both stores return; `redis.go` `Allow` returns Eval errors with the zero Status
- `tokenbucket/traefik_issue14328_test.go`, `traefik_issue14328_e2e_test.go` — concurrent first Allows admit at most burst (Memory; live Redis/Dragonfly)
- `tokenbucket/traefik_issue13704_test.go`, `traefik_issue13704_e2e_test.go` — Memory idle-reclaim and live remaining TTL
- `tokenbucket/bench_test.go`, `bench_yaegi_test.go`, `bench_e2e_test.go` — compiled, interpreted, and live benchmarks
- `openspec/specs/std_go_tokenbucket_allow/spec.md`, `openspec/specs/std_go_tokenbucket_lua-eval/spec.md`

## Gotchas

- `Redis.Allow` returning `context.Canceled` or `DeadlineExceeded` does not prove the Eval script did not run; the token may have been charged. Do not refund from this package. `Memory.Allow` on an already-done ctx returns that error and charges nothing.
- A non-nil Redis error with `!status.Allowed` is an outage, not a 429. Match `err` first. Fail-open (admit) or fail-closed (5xx) is the caller's choice; do not 429 from the zero Status.
- `rate <= 0`, NaN, Inf, `burst < 1`, `maxDelay < 0`, or a `ttl` that is not a whole number of seconds of at least 1s fails New. Redis EXPIRE is integer seconds; New must not accept a Duration Redis cannot represent. Passthrough is skipping construction. This package has no unlimited-rate constructor.
- Lua always returns `"true"`; Go maps `Allowed` from wait microseconds vs maxDelay microseconds (Lua refunds on `>`). The `Delay` and `RetryAfter` durations are not the admit gate.
- The wait is `Delay` when allowed and `RetryAfter` when denied; the other is 0, and `Delay + RetryAfter` is the single wait `Allow` used to return. `RetryAfter` is the full wait (the delay Traefik puts in `Retry-After`), not `wait - maxDelay`.
- `Period` is derived: `burst / rate`, the time to refill an empty bucket. The library never sees Traefik's `average`/`period`; `Limit / Period = rate`.
- `Remaining` and `Reset` come from the tokens kept after this Allow, after the refund when denied. Admitted-with-delay leaves tokens negative (debt): `Remaining` is 0 and `Reset` is longer than `Period`.
- Duration fields saturate at the maximum `time.Duration` instead of overflowing, and are never negative. Saturation does not change `Allowed`.
- Redis reads the tokens left from the third Eval field. Redis writes it with 14 significant digits and Dragonfly appends `.0`, so memory and Redis agree on `Remaining`/`Reset` while tokens stay whole and may differ only when that rounding crosses an integer. A tokens field that is not a finite number is `errEvalTokens` (`tokenbucket: eval tokens is not a number`); the wait is checked first, so a reply with both bad is still `errEvalWait`.
- A new or TTL-expired key starts at `burst`, then consume 1. Missing Redis hash does the same; do not assume `last=0` refill from Unix epoch. The idle lifetime is the caller `ttl` raised to that refill floor, never lowered, and it is the same value Memory expires with and Redis passes to `EXPIRE`. A tiny rate clamps the floor to the largest whole-second `time.Duration`.
- Concurrent first Allows on a new opaque key share one bucket: at most `burst` of them return `Allowed`. Memory holds the bucket-map mutex across lookup, create, and consume of one Allow; Redis runs one Eval and Go MUST NOT GET or SET that hash. Do not expect each overlapping first Allow to start from a full `burst`.
- Redis `allowScript` ends every EVAL with `EXPIRE` of `ttl` seconds, including a write to an existing key. After Allow, remaining TTL is in `(0, ttl]`. Idle Redis sources are reclaimed by the engine. Do not set EXPIRE only when the key is new.
- A 3-field Eval wait that is not a finite number (`nan`, `+Inf`, `-Inf`, `inf`) is `errEvalWait` (`tokenbucket: eval wait is not a number`), same as a garbage string. Do not treat it as admit or deny.
- EVAL scripts must list keys in `KEYS` (Dragonfly). Do not use `table.maxn`.
- Two limiter instances on one Redis key share burst. Do not expect a second full burst.
- Two `*Memory` values on the same key admit `2*burst`. Share one `*Memory` for one budget (for example across routers or reloads).
- `NewMemory` holds at most 65536 sources (Traefik `maxSources`). A new source first removes sources whose `ttl` elapsed, then one live source only if the store is still at the cap. Expired sources leave even below the cap: ten idle sources past `ttl` plus one new source leave exactly that new source stored. They do not linger until 65536. Which live source goes is not specified (the code removes the least recently used). A removed source comes back as a full bucket. Sources sit in an expiry-ordered list, so a new source costs O(1), not a map walk; a test clock that moves backwards can leave an expired source behind the head until it is looked up; at the cap that can remove a live source while the expired one stays stored. `time.Now` does not move backwards, so production keeps expired-first.
- Every `Allow` on a stored source that is not the tail moves it to the tail. `moveToBackLocked` relinks in place instead of calling `unlinkLocked` and `pushBackLocked`, because each interpreted method call costs microseconds under Yaegi (`BenchmarkYaegiMemoryAllow/rotating-10k`). Keep that path free of extra calls.
- Persisted `last` is never earlier than the previous `last`. Elapsed still clamps when now is behind `last` (no negative refill). Memory reads now after the mutex so lock order is clock order.
