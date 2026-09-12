## Context

Dest `exec` loops `attempt <= maxRetries` with `time.Sleep` and no context (`simpleredis/commands_exec.go`). `dial` uses `DialTimeout` then `do` AUTH/SELECT each with a fresh `IOTimeout`. Zero Config is `DialTimeout` 2s, `IOTimeout` 1s, `MaxRetries` 0 → 3 extra. Yaegi: stdlib only; no `net.Error` assert (`resp.go`). See proposal.md for why. Proceed policies: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Proxy-shaped zero-Config defaults and a derived overall command deadline.
- `*Context` twins; cancel closes the socket and frees the turn.
- Compiled black-hole, handshake-stall, and cancel tests.

**Non-Goals:**
- Circuit breaker (debt note).
- Passing `req.Context()` from the probe, windowcounter, or tokenbucket.
- A new `CommandTimeout` Config field.
- Changing go-redis sentinel meaning of leftover `0` inside `retryLimits` except via `applyDefaults` at `New`.
- Importing go-redis or miniredis.

## Decisions

1. **`applyDefaults` maps `MaxRetries` 0 → 1; `retryLimits` keeps `0` → 3 and `-1` off.** After `New`, zero Config is one extra retry. Callers who want four attempts set `MaxRetries: 3`. Alternative: change `retryLimits` 0 → 1 — rejected; that would also change any path that skipped `applyDefaults` and would collapse the documented `-1` vs `0` story. Alternative: leave four attempts with only shorter timeouts (~800ms) — rejected; Desired names MaxRetries 1.

2. **Derived overall budget `(maxRetries+1)*(DialTimeout+IOTimeout)` at `exec` entry.** `min` with `ctx` deadline. Remaining time is `SetDeadline` for dial and each `do`. No `Config.CommandTimeout`. Alternative: new knob — rejected; operators already have Dial/IO/retries; context is the caller-side cap.

3. **`exec(ctx, args)` plus `*Context` twins.** Unadorned verbs call `Background()`. `borrow`/`dial`/`do` take `ctx`. Backoff is `select` on timer and `ctx.Done()`, not `time.Sleep`. Cancel watcher closes `netConn` on `ctx.Done()`. Alternative: change every signature to take `ctx` first (go-redis shape) — rejected; Desired keeps wrappers so in-tree callers compile.

4. **Error tokens.** `ctx.Err()` for cancel / caller deadline. Derived-budget expiry is `redis:timeout`. `shouldRetry` retries neither. Alternative: collapse cancel to `redis:unreachable` — rejected; that hides which failure it was.

5. **Proof is compiled tests.** Black-hole `203.0.113.1:6379` asserts elapsed ≤ budget + 50ms slack (this host measured 8.047s before the fix). Handshake: fake that accepts TCP and never replies, with `Pass` and `Database`. Cancel: cancel mid-command against that stall. Do not assert elapsed ≈ 8s. Probe stays on unadorned verbs.

## Risks / Trade-offs

- [TEST-NET-3 fails fast on some CI] → Budget assertion still passes; handshake-stall fake does not depend on black-hole routing.
- [Eval EVALSHA then EVAL can pay two derived budgets] → Accepted; existing two-`exec` shape; shared `ctx` still cancels the second.
- [Shorter IOTimeout makes slow live engines flaky] → Live tests that need more time set `Config.IOTimeout` explicitly (existing pattern).
- [Cancel watcher goroutine leak] → Stop the watcher when `do` returns; tests assert the turn is free.

## Migration Plan

Unadorned methods keep compiling. Rollback is revert. No production deploy key.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
