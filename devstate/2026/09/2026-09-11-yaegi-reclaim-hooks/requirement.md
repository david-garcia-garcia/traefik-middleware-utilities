# Requirement
IssueKey: 2026-09-11-yaegi-reclaim-hooks

## Problem

Traefik v3.7.11 loads local plugins through Yaegi v0.16.1 (`docker-compose.yml`: `traefik:v3.7.11`). Values returned from an interpreted `create func() (any, error)` lose their method set when stored as `any`, so optional `Sleep` / `Wake` / `Close` on stored reclaim values never run in production plugins. The four-event lifecycle spec is inert under Yaegi despite compiled tests passing.

## Current (code)

- `reclaim/table.go:180` — `Open(..., create func() (any, error))`; lifecycle via type switches on `sleeper` / `waker` / `closer` (`reclaim/table.go:115-167`).
- `reclaim/table.go:133-143` — documents Yaegi synthesizes `any` returns with no methods; hooks stay inert interpreted.
- `reclaim/default.go:28` — package `Open` forwards the same signature.
- `openspec/specs/std_go_reclaim_value-lifecycle/spec.md:80-85` — requires optional interfaces on the stored value and type-switch lookups.
- `knowledge/debt/2026-09-11-yaegi-drops-methods-on-any.md` — debt file records the gap; Action `note` from prior ticket.
- `knowledge/devdocs/std_go_reclaim.md:48` — documents optional methods do not run under Yaegi.
- `knowledge/research/ext_traefik_plugins_yaegi-generics/notes.md:23-27` — sourced Yaegi behavior for `func() (any, error)` returns.
- `reclaim/table_test.go` — compiled lifecycle tests with `lifecycle` type implementing Sleep/Wake/Close; no Yaegi interpreter import.
- `e2e/reclaimprobe/plugin.go:54-56` — `reclaim.Open` with create returning `*probeValue` (no lifecycle methods).
- `scripts/integration-tests.Tests.ps1:41-44` — Pester asserts `reclaim_put` / `reclaim_bind` only; no orphan/dispose or hook log proof.

## Desired

- Change `Open` to take explicit lifecycle hooks (not optional interface discovery on the stored value) so Sleep/Wake/Close run under Yaegi.
- **Take** (close) debt `knowledge/debt/2026-09-11-yaegi-drops-methods-on-any.md` when fixed.
- Add tests importing the actual Yaegi interpreter for lean Yaegi-behavior coverage.
- Add a test host for the reclaim table that exercises all lifecycle hooks (likely log-only).
- Keep Pester e2e as final wrap-up for key paths under Traefik.

## Affected

- `reclaim/table.go`, `reclaim/default.go`, `reclaim/table_test.go`
- `openspec/specs/std_go_reclaim_value-lifecycle/spec.md` (and related devdocs)
- `knowledge/debt/2026-09-11-yaegi-drops-methods-on-any.md` (delete when taken)
- New Yaegi test package; `e2e/reclaimprobe/` and/or `scripts/integration-tests.Tests.ps1`
- Consumers (e.g. future geoblock import switch) must pass explicit hooks

## Out of scope

- Upgrading Traefik or Yaegi pin
- Generic `Table[T]` or changing grace/context-lease semantics
- Geoblock plugin behavior, Redis, or production middleware beyond test hosts
- Vendoring Yaegi into the library `go.mod` for runtime (test dependency only is in scope)

## Unknowns

- Exact explicit-hook API shape on `Open` (function parameters vs struct) — ticket names direction only; explore must decide.
- Whether compiled callers keep optional methods on the value as a convenience wrapper or migrate fully to explicit hooks.
- Which Yaegi import path/version pin matches Traefik v3.7.11 for unit tests (`go.mod` currently has no Yaegi dep).

## Tensions

- Prior archived design (`openspec/changes/archive/2026-09-11-add-reclaim-table/design.md:12`) listed explicit hooks as a **Non-Goal**; this ticket reverses that deliberately.
- Live spec (`std_go_reclaim_value-lifecycle`) requires optional interfaces on the value; new API will need spec delta.
- Ticket says take the debt; debt file still says Action `note` and cites API fork as why-not-taken — this ticket is that fork.
