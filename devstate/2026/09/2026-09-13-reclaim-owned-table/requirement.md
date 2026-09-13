# Requirement
IssueKey: 2026-09-13-reclaim-owned-table

## Problem
The package owns a process-wide table (`Default`, package `Open`, `Reset`, `ResetWith`). Callers cannot choose config per table or own lifetime. The ticket wants that singleton gone: each caller holds a `Table` created through `New()`, with constructor config fixed after create.

## Current (code)
- `reclaim/default.go` — process singleton: `Default()` lazy-creates `NewTable(DefaultGrace)`; package `Open` is `Default().Open`; `Reset` / `ResetWith` tear down and replace that one table. Tests only on Reset.
- `reclaim/table.go` `NewTable(grace)` — only constructor. Negative grace becomes `DefaultGrace`. `Table` stores `grace`; no setter after create.
- `reclaim/table.go` `Table.Reset()` — instance teardown (sleep then close every incarnation). Distinct from package `Reset`.
- `reclaim/table_test.go` `TestDefault_OpenSharesIncarnation`, `TestDefault_ResetWithAppliesGrace`, `TestDefault_ConcurrentFirstUseReturnsOneTable` — require the process table.
- `e2e/reclaimprobe/plugin.go` — production-shaped caller uses package `reclaim.Open`.
- `openspec/specs/std_go_reclaim_context-lease/spec.md` — SHALL expose one process-wide table (`Default` / package `Open`); Default Open shares one incarnation.
- `knowledge/devdocs/std_go_reclaim.md` — production = process table; prefix keys on Default; `NewTable` for tests.
- `knowledge/devdocs/std_go_backendbackoff.md` — store Gate in `reclaim.Open`.
- `README.md` — production uses process table (`reclaim.Open`).
- Dest has no `New()` and no `Config` type. Config today is the `grace` argument to `NewTable`.

## Desired
1. Remove the process-wide default table: `Default`, package `Open`, `Reset`, `ResetWith` in `reclaim/default.go`.
2. Callers who need a table create it and manage its lifetime (instance `Open` / `Table.Reset`, not a package singleton).
3. Construct tables with `New()` that takes the table’s config parameters. Those parameters MUST NOT change after the table exists.
4. Existing dest config (grace, including negative → `DefaultGrace`) stays that constructor input; do not invent extra knobs.

## Affected
- `reclaim/default.go` (remove singleton API)
- `reclaim/table.go` (`NewTable` → `New()`; grace stays constructor-only)
- `reclaim/table_test.go` (Default/ResetWith tests)
- `e2e/reclaimprobe/plugin.go` (must hold a table instance)
- `openspec/specs/std_go_reclaim_context-lease/spec.md` (drop process-table SHALL)
- `knowledge/devdocs/std_go_reclaim.md`, `knowledge/devdocs/std_go_backendbackoff.md`, `README.md`

## Out of scope
- New config fields dest does not already have (only `grace` today)
- Changing `Open` / `Hooks` / sleep-wake-close
- `Table.Reset` as instance teardown (ticket removes package Reset, not this method)
- Other packages’ internals (`simpleredis`, `tokenbucket`, `windowcounter`)

## Unknowns
- Whether `New` is `New(grace time.Duration)` (rename of `NewTable`) or `New(Config)` with grace as a field.
- Whether `default.go` is deleted or left without the singleton.
- How `e2e/reclaimprobe` holds the table so two `New`s in one process still share an incarnation within grace (package-level table vs per-constructor table).

## Tensions
- Ticket: no process-wide table. Spec `std_go_reclaim_context-lease` SHALL expose `Default` / package `Open`. Ticket wins; spec and usage catch up in propose.
- Ticket: `New()`. Dest constructor is `NewTable(grace)`. Rename (and any Config wrapper of existing grace) is the ask, not a second constructor.
- Usage/README/`reclaimprobe` tell callers to use package `Open`. After this change they use an owned `Table`.
