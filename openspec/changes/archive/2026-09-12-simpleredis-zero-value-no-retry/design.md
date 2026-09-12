## Context

Dest `borrow` returns `errUnreachable` when `inUseTurns` is nil. `exec` retries that identity unless `isClosed()`. `errPoolWait` already prints `redis:unreachable` with a distinct identity so pool wait is not retried. `New` is the only caller of `ensureInUseTurns`. See proposal.md for why. Proceed policies: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Nil in-use-turn borrow fails fast with a distinct identity.
- Unit proof of latency, identity, exported-method smoke, and `cap(inUseTurns) == 0`.

**Non-Goals:**
- Changing `shouldRetry` predicates.
- Calling `ensureInUseTurns` outside `New`.
- Panic, compile-time unusable zero value, empty-Host validation.

## Decisions

1. **Unexported sentinel beside `errPoolWait`.** `errors.New(RedisUnreachable)` as `errNotFromNew`. `borrow` returns it when `inUseTurns` is nil. `shouldRetry` stays identity on `errUnreachable`, so `commands_exec.go` is unchanged. Alternative: map nil-channel to `errPoolWait` — rejected; pool wait is a different job. Alternative: panic — out of scope.

2. **Latency ceiling is default `MinRetryBackoff` (8 ms).** Explore assumed elapsed `< 8ms` plus `shouldRetry(errNotFromNew) == false`. Same-package tests can see the sentinel.

3. **Smoke exported methods in `simpleredis/` unit tests.** `Get`, `MGet` (one name), `Set`, `Del`, `Incr`, `IncrBy`, `Expire`, `ExpireAt`, `Eval`, `MSetEX`, `MSetEXAt`, `Close` twice. Accessors are not commands. Empty `MGet` / invalid `MSetEX` stay existing pre-dial returns.

## Risks / Trade-offs

- [8 ms wall-clock assertion flakes on a loaded CI runner] → Mitigation: identity `shouldRetry` false is the non-flaky proof; 8 ms is well under DestBranch's ~59 ms ladder.
- [Out-of-repo callers match `errors.Is` against `errUnreachable`] → Mitigation: they still match `Error()` text; the ask keeps that token. Same-package tests use identity.

## Migration Plan

Library behavior only for a construction bug. Rollback is revert. No deploy contract.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
