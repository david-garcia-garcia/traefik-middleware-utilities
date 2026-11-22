# Design

## Context

See `proposal.md` — Why. Explore measured that `Redis.Allow` already returns `Status{}, err` on Eval failure (`tokenbucket/redis.go:67-69`); a refuse on `127.0.0.1:1` is `errors.Is(err, simpleredis.ErrUnreachable)` with a zero status. The live spec still names the display strings `redis:unreachable` / `redis:timeout` (`std_go_tokenbucket_allow` Requirement: Redis errors do not become deny). Usage still says "Match Redis errors by `Error()` text" (`knowledge/devdocs/std_go_tokenbucket.md:28`). SimpleRedis already documents `errors.Is` / `IsUnreachable` (`std_go_simpleredis.md`). Dest signature is `(Status, error)` (#106); the reference repro still uses `(bool, wait, error)`.

## Goals / Non-Goals

**Goals:**

- Pin the existing outage return with `errors.Is` and `!status.Allowed` in a dedicated unit file.
- Name `ErrUnreachable` / `ErrTimeout` on `Redis.Allow` GoDoc and in the live allow contract, including that HTTP policy is the middleware's.
- Align token-bucket usage matching with SimpleRedis.

**Non-Goals:**

- Changing the `Allow` body, wrapping the Eval error, or adding fail-open / fail-closed / an HTTP code in this library.
- A live 12091 test, an `ErrTimeout` test, rewriting existing `Error()` tests, or touching `windowcounter` / `simpleredis` sentinels.

## Decisions

### D1. No change to `Redis.Allow` body

Keep `return Status{}, err`. Wrapping would break both `errors.Is` and the exact `Error()` text that slog/tcp-session and the existing tests pin. Fail-open or fail-closed is not a library knob; upstream Traefik answers 500 with no option ([traefik/traefik#12091](https://github.com/traefik/traefik/issues/12091)).

Rejected: deny-on-error inside Allow; mapping outage to `Allowed` false with a nil error.

### D2. Dedicated unit file, dest signature

`tokenbucket/traefik_issue12091_test.go`, `TestTraefik12091_RedisUnreachable`. Self-contained: `simpleredis.New` with `Host: "127.0.0.1:1"` and `MaxRetries: -1` (no extra retry on a refuse), `NewRedis`, `status, err := limiter.Allow(ctx, key)`. Assert `errors.Is(err, simpleredis.ErrUnreachable)` and `!status.Allowed`. Do not assert `wait` / `Delay` / `RetryAfter` (zero status). Helpers, if any, live in that file only and are suffixed `12091`; do not import `newSimpleRedisForTest` from `limiter_test.go`. Close the client.

Rejected: copying other-issue tests; a shared helper file; reintroducing `(bool, wait, error)`.

### D3. Keep existing `Error()` tests; no live; no timeout peer

`TestRedis_Unreachable` and the Redis arm of `TestStatus_ErrorReturnsTheZeroStatus` stay as display-text control. Unreachable does not need a live engine; `TestLive_Limiter` already covers the happy path (`runForEachLiveEngine` / `waitLiveClient`). `ErrTimeout` is named in GoDoc and the SHALL; `simpleredis` already classifies deadline → `ErrTimeout`. A hung peer would be new machinery.

### D4. Usage How to use matches SimpleRedis

Replace "Match Redis errors by `Error()` text" with `errors.Is` / `IsUnreachable` against `ErrUnreachable` and `ErrTimeout`. `Error()` remains display (`redis:unreachable`). The pattern snippet that returns `err != nil` is a fail-closed example and may stay. Do not edit `std_go_windowcounter.md`.

### D5. Fold the live contract, do not invent a leaf

MODIFIED `std_go_tokenbucket_allow` Requirement: Redis errors do not become deny. Name `errors.Is` / `ErrUnreachable` / `ErrTimeout` and `Allowed=false` on the zero status. Keep "does not return allowed false as a substitute": the error is the outage, not a silent deny with a nil error. No delta on `std_go_simpleredis_tcp-session` (sentinels unchanged) or `std_go_tokenbucket_lua-eval` (script unchanged).

## Risks / Trade-offs

- [Existing tests match `err.Error() == redis:unreachable`] → Display text does not change; those tests stay as control. The new file is the `errors.Is` proof.
- [Callers that only check `err != nil` keep seeing an error] → Same return path. GoDoc states they choose fail-open or fail-closed.
- [`Allowed=false` with a non-nil error can be misread as deny-on-error] → Spec keeps "does not return allowed false as a substitute"; GoDoc names the zero status.

## Migration Plan

Docs, GoDoc, and one unit test. No Redis key or API change. Rollback is revert.
