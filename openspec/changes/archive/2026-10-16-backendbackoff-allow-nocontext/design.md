## Context

See `proposal.md` — Why. Today `allow.go` returns `(false, 0, ctx.Err())` before acquiring `g.mu`. Explore enumerated 36 in-package call sites; no Go importers outside `backendbackoff` in this tree. Live contract is `openspec/specs/std_go_backendbackoff_allow/spec.md`.

## Goals / Non-Goals

**Goals:**

- Synchronous `Allow(key)` with unchanged credit/cooldown/map semantics under the existing mutex.
- Preserve `(bool, time.Duration, error)` for gate errors (`errClosed`, `errUnknown`, etc.).
- Align tests, Yaegi probe, benchmarks, devdocs, and spec delta with caller-owned cancellation.

**Non-Goals:**

- Context on `Report`, README alignment (unless implement records a deviation), external Traefik plugin migrations, performance refactors (sharding, `sync.Map`, atomics).

## Decisions

1. **Remove context from Allow only** — Matches `Report`’s context-free contract and explore’s symmetric “caller owns lifecycle” decision. Alternative: keep context for backward compatibility — rejected (contradicts requirement).

2. **Delete canceled-context test, no in-package middleware substitute** — Cancellation is not observable inside `backendbackoff` without faking HTTP/middleware; spec states pre-call responsibility instead.

3. **Fold into `std_go_backendbackoff_allow`** — Small adjustment to one leaf (signature + one scenario swap + test requirement wording); not a new catalog leaf.

## Risks / Trade-offs

- **[Breaking API]** → Document in devdocs and spec; external copies migrate independently (explore assumed out of scope).
- **[Callers forget pre-cancel check]** → Devdocs show checking `req.Context().Err()` (or equivalent) before `Allow`; same pattern as skipping backend work after cancel.

## Migration Plan

1. Change signature and remove early return in `allow.go`.
2. Mechanical update of test/Yaegi/bench call sites.
3. Update devdocs snippet and Allow bullet.
4. Archive merges spec delta into live `std_go_backendbackoff_allow`.
5. Rollback: revert commit; no persistent storage impact.
