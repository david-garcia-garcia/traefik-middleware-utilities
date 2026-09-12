## Context

Dest `exec` loops `attempt <= maxRetries` with `time.Sleep` and no context (`simpleredis/commands_exec.go`). `dial` uses `DialTimeout` then `do` AUTH/SELECT each with a fresh `IOTimeout`. Zero Config is `DialTimeout` 2s, `IOTimeout` 1s, `MaxRetries` 0 → 3 extra. Yaegi: stdlib only; no `net.Error` assert (`resp.go`). See proposal.md for why. Proceed policies: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Proxy-shaped zero-Config defaults and a derived overall command deadline.
- Every public verb takes `context.Context` first; cancel closes the socket and frees the turn.
- Compiled black-hole, handshake-stall, and cancel tests.

**Non-Goals:**
- Circuit breaker (debt note).
- Passing a request context through `windowcounter.Take` or `tokenbucket.Allow`.
- A new `CommandTimeout` Config field.
- Changing go-redis sentinel meaning of leftover `0` inside `retryLimits` except via `applyDefaults` at `New`.
- Importing go-redis or miniredis.

## Decisions

1. **`applyDefaults` maps `MaxRetries` 0 → 1; `retryLimits` keeps `0` → 3 and `-1` off.** After `New`, zero Config is one extra retry. Callers who want four attempts set `MaxRetries: 3`. Alternative: change `retryLimits` 0 → 1 — rejected; that would also change any path that skipped `applyDefaults` and would collapse the documented `-1` vs `0` story. Alternative: leave four attempts with only shorter timeouts (~800ms) — rejected; Desired names MaxRetries 1.

2. **Derived overall budget `(maxRetries+1)*(DialTimeout+IOTimeout)` at `exec` entry.** `min` with `ctx` deadline. Remaining time is `SetDeadline` for dial and each `do`. No `Config.CommandTimeout`. Alternative: new knob — rejected; operators already have Dial/IO/retries; context is the caller-side cap.

3. **`exec(ctx, args)` and every public verb takes `ctx` first.** `borrow`/`dial`/`do` take `ctx`. Backoff is `select` on timer and `ctx.Done()`, not `time.Sleep`. Cancel watcher closes `netConn` on `ctx.Done()`. Alternative: `*Context` twins with unadorned `Background()` wrappers — rejected; that lets request-path callers skip cancel.

4. **Error tokens.** `ctx.Err()` for cancel / caller deadline. Derived-budget expiry is `redis:timeout`. `shouldRetry` retries neither. Alternative: collapse cancel to `redis:unreachable` — rejected; that hides which failure it was.

5. **Proof is compiled tests.** Black-hole `203.0.113.1:6379` asserts elapsed ≤ budget + 50ms slack (this host measured 8.047s before the fix). Handshake: fake that accepts TCP and never replies, with `Pass` and `Database`. Cancel: cancel mid-command against that stall. Do not assert elapsed ≈ 8s. The probe passes `req.Context()`. windowcounter and tokenbucket pass `context.Background()`.

## Risks / Trade-offs

- [TEST-NET-3 fails fast on some CI] → Budget assertion still passes; handshake-stall fake does not depend on black-hole routing.
- [Eval EVALSHA then EVAL can pay two derived budgets] → Accepted; existing two-`exec` shape; shared `ctx` still cancels the second.
- [Shorter IOTimeout makes slow live engines flaky] → Live tests that need more time set `Config.IOTimeout` explicitly (existing pattern).
- [Cancel watcher goroutine leak] → Stop the watcher when `do` returns; tests assert the turn is free.

## Migration Plan

Unadorned methods no longer compile. Callers pass `context.Background()` or `req.Context()`. Rollback is revert. No production deploy key.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
