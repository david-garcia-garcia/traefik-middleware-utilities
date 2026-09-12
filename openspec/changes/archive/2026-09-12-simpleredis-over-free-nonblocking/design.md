## Context

Dest `freeInUseTurn` sends on `inUseTurns` with no `default`. `ensureInUseTurns` pre-fills the buffer to `liveCap`. Same-package tests can call the unexported send. Yaegi already interprets `closed atomic.Bool` on `SimpleRedis`. Proceed policies: `devstate/explore.md`. See proposal.md for why.

## Goals / Non-Goals

**Goals:**
- Non-blocking extra return plus `atomic.Int64` `OverFrees()` readable from compiled tests.
- Guard and invariant in `simpleredis/pool_test.go`. Run the invariant locally with `-race`.

**Non-Goals:**
- Panic on over-free, mutex-counter rewrite, probe/Pester wiring, CI `-race` workflow, sibling `simpleredisfixes2/` findings.

## Decisions

1. **`select` with `default`, not panic.** Dropping a token cannot raise the live cap: a full semaphore already means nothing in use. Alternative: panic — louder, but Out of scope and the how-to-fix names silent drop. Alternative: mutex counter under `idleConnsMu` — also fixes bug-03's `len(inUseTurns)` read; that is a larger pool rewrite and out of scope.

2. **`atomic.Int64` field, exported `OverFrees() int64`.** Matches `closed atomic.Bool` (Yaegi-safe struct field). Place the accessor next to `PoolSize()` / `MaxIdleConns()`. Alternative: unexported test-only helper — Desired asks an operator who already holds `*SimpleRedis` can read it; do not wire the Traefik probe (0 call sites of the sibling accessors in `e2e/simpleredisprobe`).

3. **Tests live in `pool_test.go`.** Same-package owner of the semaphore. Guard: `New` then one extra `freeInUseTurn` with a short timer so a hang fails the test. Invariant: many goroutines × four client shapes (healthy fake, `127.0.0.1:1`, AUTH-rejecting fake, `PoolSize: 1` + short `PoolTimeout` against a delayed fake). After `Wait`, `len(inUseTurns)==cap(inUseTurns)` and `OverFrees()==0`. Both halves: length alone cannot distinguish balanced from one leak plus one over-free.

4. **Local `-race` only.** Run `go test -race` on the invariant in this change. Do not edit GitHub workflows (ci-01).

## Risks / Trade-offs

- [Silent drop hides a leak until someone reads `OverFrees()`] → Mitigation: guard test plus invariant; usage-packet gotcha names the counter.
- [Yaegi `atomic.Int64` field] → Mitigation: dest already uses `atomic.Bool` on the same struct; keep the same field pattern.
- [Invariant flakes on AUTH/dead-address retries] → Mitigation: `MaxRetries: -1` where retries would multiply wait; assert finish, not success.

## Migration Plan

Library behavior change only. Rollback is revert. No deploy key.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
