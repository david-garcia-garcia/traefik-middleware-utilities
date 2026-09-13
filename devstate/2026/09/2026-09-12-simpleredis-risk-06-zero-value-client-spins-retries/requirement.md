# Requirement
IssueKey: 2026-09-12-simpleredis-risk-06-zero-value-client-spins-retries

## Problem
A `SimpleRedis` built without `New` (`&SimpleRedis{}`) has a nil in-use-turn channel. `borrow` returns `errUnreachable` for that, and `exec` retries that sentinel for the full default ladder, so a programming error is reported as a transient network failure after ~53 ms.

## Current (code)
- `simpleredis/simpleredis.go` — `SimpleRedis` is an exported struct; unexported fields still allow `&SimpleRedis{}`. `New` is the only caller of `ensureInUseTurns`. Zero-value `closed` is false, so `isClosed()` is false.
- `simpleredis/pool.go` `borrow` — nil `inUseTurns` returns `errUnreachable` (same identity `shouldRetry` treats as retryable). Closed client also returns `errUnreachable`.
- `simpleredis/pool.go` `ensureInUseTurns` — unexported; if called on a zero-value client it builds `liveCap()` (`defaultPoolSize` 8) against empty `host`.
- `simpleredis/commands_exec.go` `exec` — retries while `shouldRetry(err)` and not `isClosed()`. `retryLimits(0,0,0)` is 3 extra retries, 8 ms / 512 ms backoff.
- `simpleredis/commands_exec.go` `isUnreachable` / `shouldRetry` — identity match on `errUnreachable` only. `errPoolWait` shares `Error()` text `redis:unreachable` and is not retried (`commands_exec_test.go` `TestShouldRetryPoolWaitIsFalse`).
- `simpleredis/simpleredis.go` `errPoolWait` — already the distinct-identity, same-text pattern the ticket wants for a not-from-`New` client.
- `simpleredis/commands.go`, `commands_eval.go`, `commands_msetex.go` — exported verbs go through `exec` (empty `MGet`/`MSetEX` return before borrow). `Close` on a zero value is a no-op after CAS.
- Accessors: `PoolSize()` / `PoolTimeout()` / `IdleTimeout()` / `DialTimeout()` / `IOTimeout()` fall back to package defaults; `MaxIdleConns()` and `MaxRetries()` return the zero field (0).
- No test asserts zero-value command latency or `cap(inUseTurns)==0` (`simpleredis/*_test.go` — not found).
- In-repo consumers inject `*simpleredis.SimpleRedis` from `New`: `tokenbucket/redis.go` `NewRedis`, `windowcounter/limiter.go` `New`. They do not construct `&SimpleRedis{}`.

## Desired
1. A client that did not come from `New` fails on the first command, immediately, without the retry ladder.
2. The error stays `redis:unreachable` by `Error()` text so string matchers still work, but a distinct identity (same pattern as `errPoolWait`) so `shouldRetry` does not treat it as transient.
3. Proof: zero-value `Get` returns in well under one backoff interval with `simpleredis.RedisUnreachable`; smoke every exported method on `&SimpleRedis{}` including `Close` twice with no panic; `cap(sr.inUseTurns) == 0` so `New` remains the only semaphore constructor.

## Affected
- `simpleredis/simpleredis.go` (new sentinel beside `errPoolWait`)
- `simpleredis/pool.go` `borrow` nil-`inUseTurns` return
- `simpleredis/commands_exec.go` (no `shouldRetry` change if identity is distinct)
- New or extended unit tests under `simpleredis/`

## Out of scope
- Panic on nil semaphore.
- Making the zero value unusable at compile time (unexported required field, or `New` returning an interface).
- Rejecting empty `Host` in `New`, or documenting that `New` cannot fail.
- Calling `ensureInUseTurns` from anywhere other than `New`, or changing empty-host dial behavior of a client that *did* come from `New`.
- Other `simpleredisfixes2` findings.
- Changing `tokenbucket` / `windowcounter` construction.

## Unknowns
- Exact wall-clock bound for the latency assertion (ticket probe 53.0428 ms; backoff is jittered; ticket asks “well under one backoff interval”).
- Whether any out-of-repo caller constructs `SimpleRedis` without `New`.

## Tensions
- Ticket severity is judgement and both in-repo consumers already inject `New`; the ask is still a fail-fast guardrail, not a live production bug.
- Ticket “accessors fall back to defaults” is true for `PoolSize` / timeouts, not for `MaxIdleConns()` or `MaxRetries()` (they return 0). Accessor polish is not the ask.
- Ticket lists panic / API reshape / Host validation as optional; those contradict the recommended two-line identity split and stay out of scope.
