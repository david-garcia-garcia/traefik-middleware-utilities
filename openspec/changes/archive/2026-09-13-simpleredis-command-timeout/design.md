## Context

Dest `do` calls `SetDeadline` once for the write plus the whole reply, using `clampTimeout(ctx, IOTimeout)`. The overall budget already exists as `bindCommandDeadline` plus `watchConnClose`, so `IOTimeout` was being spent twice: once shaping the budget, once capping the socket. Caller vs library timeout ownership is `clampTimeout` / `ioOrContext` / `libraryTimeout`. Proceed policies: `devstate/explore.md`.

An earlier revision of this change wrapped the TCP conn in a `stallConn` that refreshed `SetReadDeadline` / `SetWriteDeadline` per kernel call, turning `IOTimeout` into a stall bound. That was rejected on review as too much machinery for the problem: it added a full hand-written `net.Conn` implementation (Yaegi does not accept promoted methods from an embedded `net.Conn`), a second deadline regime to reason about, and a `pooledConn.stall` field, all to keep a cap that the command budget already provides.

## Goals / Non-Goals

**Goals:**
- One bound on a command socket: the command budget.
- A steadily streaming multi-megabyte bulk that outlasts one `IOTimeout` returns intact.
- A drip peer cannot hold an in-use turn past the budget.
- Existing caller-deadline vs `redis:timeout` tests stay green.
- A sane default: 250 ms rather than 100 ms.

**Non-Goals:**
- Changing `readBulk` allocation (sibling BUG-4).
- Shrinking `maxBulkLength` or adding a bandwidth model (deviation).
- A second timeout knob.
- Keeping the sub-`IOTimeout` fail-fast on a quiet peer.
- Other packages, `syscall`, `net.Error` asserts.

## Decisions

1. **`do` stamps `SetDeadline` from `commandBudgetLeft(ctx, IOTimeout)`.** That is `time.Until(ctx.Deadline())`, which is exactly what `exec` bound at entry. One line replaces the wrapper. The `fallback` argument covers a `ctx` with no deadline (direct-`do` tests), where `IOTimeout` remains the bound.

2. **`ioOrContext` loses `ioBound` and `ioTimeout`.** It previously compared them to tell "socket deadline was really the ctx deadline" from "socket deadline was `IOTimeout`". Now the socket deadline always *is* the ctx deadline when one exists, so `ctx.Deadline()` presence is the whole test. `libraryTimeout` still maps library budget to `redis:timeout` and a sooner caller deadline to `Err()`, so `TestGetCallerDeadlineIsDeadlineExceededNotRedisTimeout` and the zero-Config budget tests keep their answers.

3. **`defaultIOTimeout` 100 ms → 250 ms.** The zero-Config budget goes 600 ms → 900 ms. 100 ms left no room for a normal WAN hop plus a non-trivial value.

4. **Leave `maxBulkLength` at `64 << 20`.** With the budget as the only bound, `IOTimeout` is not a size bound. The budget plus `watchConnClose` is the wall-time cap.

5. **Tests in `simpleredis/bug3_command_budget_deadline_test.go` with `bug3` helper prefix.** Avoids colliding with sibling branches' fakes. The silent-peer test asserts elapsed is both `>= IOTimeout` and `<= budget + slack`; the lower bound is what fails if the socket deadline ever drifts back to a per-operation cap.

## Risks / Trade-offs

- [A quiet peer now costs the whole budget instead of `IOTimeout`] → Accepted, and pinned by test. It replaces silent permanent data loss on healthy peers; lowering `IOTimeout` or `DialTimeout` lowers the ceiling.
- [A drip peer could pin a turn] → Mitigation: `watchConnClose` closes on ctx done; the drip test asserts elapsed ≤ budget + slack and `OverFrees()==0`.
- [Raising the default widens the worst case for callers who never set it] → Accepted: 900 ms worst case on a dead peer, and callers with a stricter need pass their own context deadline, which still wins.
- [Yaegi `net.Error` assert] → Mitigation: keep `errors.Is`; no conn wrapper and no type assert remain.
- [BUG-4 conflict in `readBulk`] → Mitigation: do not touch allocation.

## Migration Plan

Library behavior change only. Rollback is revert. No deploy key.

## Open Questions

None. Rows live on `devstate/explore.md`.
