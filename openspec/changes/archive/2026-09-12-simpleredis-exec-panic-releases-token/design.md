## Context

Dest `exec` borrows, calls `do`, then `release` with no attempt-scoped defer. `release` is the post-borrow path into `freeInUseTurn`. `readReply` still `make`s from a parsed array length, so `*1000000000000000000\r\n` panics `makeslice`. `startStaticRedis` already exists. Proceed policies: `devstate/explore.md`. See proposal.md for why.

## Goals / Non-Goals

**Goals:**
- Attempt-scoped token return after a successful `borrow` in `exec`, including panic unwind.
- On that panic path, close the socket (`reusable` false).
- Unit proof of token conservation and a later successful `borrow`.

**Non-Goals:**
- Parser allocation cap (bug-02).
- Non-blocking `freeInUseTurn` (risk-05).
- Wrapping `dial` AUTH/SELECT `do`.
- Recovering the panic in `exec`.

## Decisions

1. **Named helper next to `exec`, not a function-scoped defer and not an IIFE.** A helper that runs `do` then always `release` owns one job. Function-scoped `defer` in `exec` would hold the token across retries. Ticket IIFE is the same job; a named owner matches One job, one owner. Alternative: IIFE in the loop — rejected only for naming; behavior is identical.

2. **`reusable` is a named result the deferred close reads after `do` returns.** On panic it stays false, so `release` closes the socket. Alternative: always close on every `do` — rejected; clean replies still pool.

3. **Do not recover in `exec`.** Desired 2. Alternative: recover and return `redis:issue?` — out of scope; hides the panic source.

4. **Proof in `pool_test.go` with `startStaticRedis`.** `PoolSize: 2`, `MaxRetries: -1`, recover `PoolSize` panics, assert `len(inUseTurns) == cap(inUseTurns)`, then `borrow` succeeds. Ticket markdown `test-02` is not on dest. Alternative: Yaegi-only panic case — only if the compiled test plus existing Yaegi `defer` coverage is not enough (explore assumed it is).

5. **Do not wrap `dial`.** Handshake panic remains a debt note.

## Risks / Trade-offs

- [Function-scoped defer holds tokens across retries] → Mitigation: helper scoped to one attempt (decision 1).
- [Parser panic source is removed by bug-02] → Mitigation: assert token cap after recovered panics, not a specific `makeslice` string (spec).
- [Yaegi defer mismatch] → Mitigation: dest already interprets `defer` in `takeIdleConn`; add a Yaegi case only if that suite fails after apply.

## Migration Plan

Library behavior on the panic path only. Rollback is revert. No deploy key.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
