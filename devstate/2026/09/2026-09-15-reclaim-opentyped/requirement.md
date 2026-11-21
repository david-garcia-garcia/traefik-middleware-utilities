# Requirement
IssueKey: 2026-09-15-reclaim-opentyped

## Problem
Callers that need `Hooks` funcs to close over the value from `create` must declare a variable, assign inside `create`, and wire hooks externally; that pattern was duplicated at three consumer sites. Hooks are evaluated before `Open` runs, so hooks cannot be returned from `create` through today's API without boilerplate.

## Current (code)
- `reclaim/table.go` — `Table.Open(ctx, key, logger, create func() (any, error), hooks Hooks)` runs the lookup loop and passes a separate `hooks` argument into `put` (`reclaim/table.go:369-403`, `407-423`).
- `reclaim/table.go` — `put` calls `create()` for `(any, error)` only, then `publishPut(..., hooks)` with the caller-supplied hooks (`reclaim/table.go:407-418`, `426-437`).
- `reclaim/opentyped.go` — not found.
- `reclaim/table_test.go` — nil table, nil logger, nil create error strings and ordering for `Open` (`reclaim/table_test.go` ~1580, ~1596, ~1948).
- `reclaim/yaegi_test.go` — Yaegi v0.16.1 harness for hooks via current `Open` signature (`reclaim/yaegi_test.go`).
- `openspec/specs/std_go_reclaim_value-lifecycle/spec.md` — hooks described as passed to `Open` at create (`openspec/specs/std_go_reclaim_value-lifecycle/spec.md:199-228`).
- `knowledge/devdocs/std_go_reclaim.md` — pattern snippet shows external `Hooks` closing over `v` assigned in `create` (`knowledge/devdocs/std_go_reclaim.md:63-77`).

## Desired
- Add `Table.OpenWithHooks(ctx, key, logger, create func() (any, Hooks, error)) (any, error)`; move the lookup loop from `Open` into `OpenWithHooks`.
- Refactor `put` to accept `create func() (any, Hooks, error)`, drop the separate hooks parameter, capture hooks from `create`, pass them to `publishPut` unchanged.
- Keep `Open` signature and semantics; implement as wrapper that adapts `create` and fixed `hooks` into `OpenWithHooks`; preserve nil-argument error precedence (nil table, nil logger, nil create) exactly as today.
- Add package-level `OpenTyped[T any](ctx, t, key, logger, create func() (any, Hooks, error)) (T, error)` in new `reclaim/opentyped.go`; type mismatch error `reclaim: open %q: want %T, got %T`.
- Later (not prepare): tests, spec/devdocs updates, no tag/version bump in this repo.

## Affected
- `reclaim/table.go` (`Open`, `put`, new `OpenWithHooks`).
- New `reclaim/opentyped.go`.
- Follow-on: `reclaim/*_test.go`, `openspec/specs/std_go_reclaim_value-lifecycle/spec.md`, `knowledge/devdocs/std_go_reclaim.md`; consumer `traefik-geoblock` version/vendor after merge (out of this repo).

## Out of scope
- Applying ready-made patch at `D:\tmp\reclaim-upstream-patch\`.
- Re-running Yaegi validation in traefik-geoblock (already done; document only).
- Git tag or module version bump in traefik-middleware-utilities.
- Implementing tests, OpenSpec deltas, or devdocs edits in prepare.

## Unknowns
- Exact OpenSpec change folder name and spec delta wording (propose phase).
- Whether to extend `reclaim/yaegi_test.go` for `OpenTyped` call-expression constraint in this repo (ticket lists as later obligation).

## Tensions
- None between ticket and tree; consumer validation is ahead of upstream code landing here.
