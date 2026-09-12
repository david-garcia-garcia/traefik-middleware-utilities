## Context

Dest exports error string consts and keeps sentinel values unexported (`simpleredis/simpleredis.go`). `shouldRetry` / `isUnreachable` identity-compare `errUnreachable` so `errPoolWait` is not retried (`commands_exec.go`). `windowcounter.getCount` string-compares `RedisMiss`. `Limiter.redis` is `*simpleredis.SimpleRedis` (no interface). Proceed policies: `devstate/explore.md`. See proposal.md for why.

## Goals / Non-Goals

**Goals:**
- Exported sentinels and three predicates that survive wrapping.
- Pool wait still not retried after it wraps unreachable.
- Window-counter miss-as-zero via `IsMiss`.
- Yaegi `clientprobe` observes `errors.Is` and one predicate.

**Non-Goals:**
- Converting e2e probe or other `err.Error()` matches.
- Wrapping `parseEvalInt` / `RedisIssue` (bug-05).
- A Redis interface or Get test hook on `Limiter`.
- Rewriting `shouldRetry` onto `errors.Is` except as needed to keep pool wait unretried.

## Decisions

1. **Exported vars, unexported aliases.** `ErrMiss` etc. are the exported values. `errMiss` / `errUnreachable` / `errPoolWait` alias them so in-package sites stay. Alternative: rename every in-package use — rejected; Desired says aliases.

2. **`ErrPoolWait = fmt.Errorf("%w", ErrUnreachable)`.** Same `Error()` text. `errors.Is(pool, unreachable)` true. Keep `isUnreachable` on identity so `shouldRetry(ErrPoolWait)` stays false. Alternative: rewrite `shouldRetry` to `errors.Is` with pool-wait first — out of scope unless identity breaks.

3. **Predicates only for Miss, Unreachable, PoolWait.** Desired names those three. Do not add `IsTimeout` / `IsNoAuth` / `IsIssue`.

4. **Miss-as-zero helper, not a Redis interface.** Extract `countFromRedisGet` (or equivalent) that `getCount` already owns. Test feeds wrapped `ErrMiss`. Fake TCP Peek on an empty store covers unwrapped `$-1`. Alternative: `SetGetForTest` — rejected; commandments forbid a production hook that only tests call unless it says Test, and a Get hook is extra surface. See `devstate/deviations.md`.

5. **Yaegi: both `errors.Is` and one predicate on `clientprobe`.** `ioError` already uses `errors.Is` under Yaegi. If interpreted `errors.Is` on package vars fails, keep the predicate and pin that outcome.

## Risks / Trade-offs

- [`errors.Is(ErrPoolWait, ErrUnreachable)` tempts a later `shouldRetry` rewrite that retries pool wait] → Mitigation: keep identity; pin `TestShouldRetryPoolWaitIsFalse` plus wrapping asserts.
- [Windowcounter wrapped-miss test never hits `SimpleRedis.Get`] → Mitigation: same classification `getCount` calls; Get wrapping is bug-05.
- [Yaegi cannot see exported vars] → Mitigation: predicates are the interpreted surface; pin whichever path works.

## Migration Plan

Additive API. Rollback is revert. Existing `Error()` text and string consts stay. No deploy keys.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
