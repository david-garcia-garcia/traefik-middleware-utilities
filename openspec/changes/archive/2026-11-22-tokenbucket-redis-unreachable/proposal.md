# Proposal

## Why

Upstream Traefik's `rateLimit` answers HTTP 500 `Could not insert/update bucket` when Redis is down and offers no fail-open or fail-closed option ([traefik/traefik#12091](https://github.com/traefik/traefik/issues/12091)). This library already returns the Eval error with a zero `Status` (`Allowed` false) and leaves HTTP policy to the caller; the gap is a dedicated `errors.Is` proof, GoDoc that names the sentinels, and a live contract that still describes the outage as the display string `redis:unreachable`.

## What Changes

- No behavior change: `Redis.Allow` keeps `return Status{}, err` on Eval failure. Fail-open or fail-closed stays the middleware's decision. The library does not add an HTTP status or a deny-on-error path.
- GoDoc on `Redis.Allow`: on an outage it returns `simpleredis.ErrUnreachable` (or `ErrTimeout`) with `allowed=false`; cite [traefik/traefik#12091](https://github.com/traefik/traefik/issues/12091).
- New self-contained test `tokenbucket/traefik_issue12091_test.go`, `TestTraefik12091_RedisUnreachable`: Redis limiter pointed at `127.0.0.1:1` with `MaxRetries: -1` returns `errors.Is(err, simpleredis.ErrUnreachable)` and `!status.Allowed`. Dest signature is `(Status, error)`. Helpers, if any, live in that file only and are suffixed `12091`.
- Align `knowledge/devdocs/std_go_tokenbucket.md` How to use to `errors.Is` / `IsUnreachable`. Do not touch `windowcounter` or `std_go_windowcounter.md`.
- MODIFIED `std_go_tokenbucket_allow`: name `errors.Is` / `ErrUnreachable` / `ErrTimeout` and `Allowed=false` on the zero status, without deleting "does not return allowed false as a substitute".
- Do not rewrite existing `Error()` tests (`TestRedis_Unreachable`, `TestStatus_ErrorReturnsTheZeroStatus`). No live 12091 test. No `ErrTimeout` test (GoDoc / SHALL only). No `simpleredis` sentinel changes.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_tokenbucket_allow`: Redis outage matching is `errors.Is` against `simpleredis.ErrUnreachable` / `ErrTimeout`; the zero `Status` has `Allowed=false`; the error is still the outage, not a silent deny.

## Impact

- `tokenbucket/redis.go`: GoDoc on `Allow` only. Body must not change behavior.
- `tokenbucket/traefik_issue12091_test.go`: new unit file (loopback refuse, no live engine).
- `knowledge/devdocs/std_go_tokenbucket.md`: How to use (match Redis errors with `errors.Is` / `IsUnreachable`). The fail-closed snippet that propagates `err != nil` may stay.
- `openspec/specs/std_go_tokenbucket_allow/spec.md`: after archive, the Unreachable / zero-status contract names the sentinels.
- Unchanged: `simpleredis` sentinels and `ioError`, `allowScript`, `windowcounter/`, existing `Error()` tests, HTTP policy, the reference worktree `tokenbucket-traefik-upstream-repros`.
