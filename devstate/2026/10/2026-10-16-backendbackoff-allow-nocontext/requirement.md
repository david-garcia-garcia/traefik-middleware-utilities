# Requirement
IssueKey: 2026-10-16-backendbackoff-allow-nocontext

## Problem
`Gate.Allow` takes `context.Context` only to reject already-canceled or deadline-exceeded callers via `ctx.Err()`. The ticket removes that coupling so admission logic stays synchronous and callers decide whether to invoke `Allow` after their request context is done.

## Current (code)
- `backendbackoff/allow.go` — `Allow(ctx context.Context, key string)` returns `(false, 0, ctx.Err())` when `ctx` is done, then runs credit/cooldown state machine under `g.mu`.
- `backendbackoff/allow.go` — `Report(key, success bool)` has no context parameter (unchanged contract).
- `backendbackoff/gate_test.go` — `TestAllow_CanceledContext` expects canceled `ctx` to deny without admitting.
- `backendbackoff/gate_test.go` — other tests call `Allow(context.Background(), "k")` or a local `ctx`.
- `backendbackoff/gate_yaegi_test.go` — interpreted probe calls `gate.Allow(ctx, "k")`.
- `backendbackoff/bench_test.go` — benchmark uses `Allow(ctx, "k")`.
- `openspec/specs/std_go_backendbackoff_allow/spec.md` — documents `Allow(ctx, key)`, requires `context.Background()` when no deadline, and scenario “Canceled context does not admit”.
- `knowledge/devdocs/std_go_backendbackoff.md` — usage tells callers to pass `req.Context()` to `Allow`.
- `README.md` — example uses `gate.Allow(req.Context(), "backend:"+host)` (not named in ticket artifacts list).

## Desired
- `Allow(key string)` with no `context.Context`; remove the `ctx.Err()` early return and drop the `context` import from `allow.go` if unused.
- Keep `Report(key, success bool)` signature and behavior unchanged.
- Do not change credit math, cooldown scheduling, in-memory map semantics, or the single mutex guarding the gate.
- Update `backendbackoff` tests (remove or replace canceled-context coverage; call `Allow("k")` elsewhere).
- Update `openspec/specs/std_go_backendbackoff_allow/spec.md` to `Allow(key)` and remove context-cancellation requirements/scenarios; state that request cancellation is handled before calling `Allow`.
- Update `knowledge/devdocs/std_go_backendbackoff.md` to document `Allow(key)` without context guidance tied to `req.Context()`.

## Affected
- `backendbackoff/allow.go`
- `backendbackoff/gate_test.go`
- `backendbackoff/gate_yaegi_test.go`
- `backendbackoff/bench_test.go`
- `openspec/specs/std_go_backendbackoff_allow/spec.md`
- `knowledge/devdocs/std_go_backendbackoff.md`

## Out of scope
- Credit bucket, cooldown duration, TTL/map cap, and mutex design in `backendbackoff/`.
- `Report` API or its context-free contract.
- `tokenbucket`, `windowcounter`, or other packages’ `Allow(ctx, …)` signatures.
- `README.md` and other docs not listed in the ticket (unless implement chooses alignment).

## Unknowns
- Whether downstream Traefik plugin copies outside this repo already call `Allow(ctx, key)` and need a separate migration (not visible in this tree).

## Tensions
- Live spec and devdocs currently require context propagation and canceled-context denial; ticket explicitly reverses that for `Allow` only while `Report` remains without context (symmetric “caller owns lifecycle” for attempts vs outcomes).
