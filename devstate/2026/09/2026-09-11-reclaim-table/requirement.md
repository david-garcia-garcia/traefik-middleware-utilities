# Requirement
IssueKey: 2026-09-11-reclaim-table

## Problem
The reclaim table lives inside `traefik-geoblock` (`pkg/reclaim`) and is not reusable as a standalone library. This repo (`traefik-middleware-utilities`) is meant to host shared Traefik middleware primitives, starting with reclaim, with tests that prove Yaegi compatibility — not just compiled `go test`.

## Current (code)
- `reclaim/` package — not found (no product tree on `origin/initial` at `ef7be38`)
- `go.mod` / module `github.com/david-garcia-garcia/traefik-middleware-utilities` — not found
- `README.md` (module path, `reclaim/` layout, Yaegi rules) — not found on `origin/initial` or IssueKey worktree; exists untracked on caller checkout only (`d:\repositories\traefik-middleware-utilities\README.md`)
- OpenSpec specs for reclaim — not found
- Pester / Traefik Yaegi e2e harness — not found
- Source reclaim implementation — `david-garcia-garcia/traefik-geoblock` PR #83 @ `22f09a0`: `pkg/reclaim/table.go`, `default.go`, `table_test.go` (external; see `knowledge/research/ext_geoblock_reclaim-source/`)
- Reclaim specs in source project — `openspec/specs/std_go_reclaim_context-lease/spec.md`, `openspec/specs/std_go_reclaim_value-lifecycle/spec.md` (external; geoblock-specific `core_geoblock_*` specs are out of scope for spin-off)
- Yaegi reclaim constraints — documented in geoblock `knowledge/devdocs/std_go_reclaim.md` and `knowledge/research/ext_traefik_plugins_yaegi-generics/` (external until copied)

## Desired
- Add the reclaim table package under `reclaim/` with module path `github.com/david-garcia-garcia/traefik-middleware-utilities`, ported from geoblock PR #83.
- Extensive unit tests (port/adapt `table_test.go` coverage from source).
- E2e Pester-based tests that load a fake middleware through Yaegi (Traefik local-plugin path) and prove reclaim behavior under interpretation.
- Bring in OpenSpec specs that describe reclaim table behavior (`std_go_reclaim_context-lease`, `std_go_reclaim_value-lifecycle`) and relevant Traefik/Yaegi knowledge (adapt, do not copy whole geoblock tree).

## Affected
- New `reclaim/` package and `go.mod`
- New test harness (Pester + fake middleware + docker/Traefik as needed)
- `openspec/specs/` (reclaim specs, after propose)
- `knowledge/research/` and `knowledge/devdocs/` (Traefik/Yaegi/reclaim usage)
- `README.md` (when committed to product tree)

## Out of scope
- Redis connection or leaky bucket libraries (README lists as planned, not this ticket)
- Geoblock-specific wrappers (`pkg/dbwrappers`), dbsource, plugin wiring
- `core_geoblock_*` OpenSpec specs
- Remote issue tracker or remote PR workflow
- Pushing to origin
- Changing geoblock PR #83 or traefik-modsecurity sync (source debt notes shared copy elsewhere)

## Unknowns
- Exact fake-middleware shape for Yaegi e2e (minimal plugin that imports `reclaim/` and exercises Open/Sleep/Wake/Close)
- Whether e2e runs in CI on this host only via docker (prepare did not run tests)
- How much of geoblock Pester/docker-compose harness to reuse vs rewrite for a library-only repo
- Yaegi optional-interface limitation (`sleeper`/`waker`/`closer` via `any` return) — documented upstream debt; spin-off may inherit same constraint unless explore chooses explicit hooks

## Tensions
- Caller README describes intended layout and Yaegi rules but is not on `origin/initial` or the IssueKey worktree — product tree is empty while the ticket references README as ground truth.
- Ticket asks for specs/knowledge from geoblock; prepare records the gap — copying/adapting is propose/implement work, not done here.
