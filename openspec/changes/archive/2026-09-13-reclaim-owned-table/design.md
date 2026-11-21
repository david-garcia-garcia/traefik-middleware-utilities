## Context

See proposal.md for why. Dest has `NewTable(grace)` (`reclaim/table.go`) which already copies grace onto an unexported field with no setter, and a process singleton in `reclaim/default.go` that always constructs `NewTable(DefaultGrace)`. Production-shaped callers (`e2e/reclaimprobe/plugin.go`, README, `knowledge/devdocs/std_go_reclaim.md`) go through package `Open`. Neighbor constructors already take a value `Config` copied at `New` (`simpleredis`, `backendbackoff`). Open questions and assumed decisions: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- One constructor `New(Config)` that freezes grace.
- No process table in `reclaim`.
- Probe still shares an incarnation across two Traefik `New`s inside grace.

**Non-Goals:**
- Extra table knobs (`EnforceCloseBeforeOpen` stays on `Hooks`).
- Changing `Open` / `Hooks` / sleep-wake-close.
- Removing `Table.Reset`.
- A `reclaim/config.go` file.

## Decisions

- `New(Config)` by value, `Grace` only. Alternative: `New(grace time.Duration)`. Rejected: the ask is constructor config; dest already has the `simpleredis`/`backendbackoff` Config shape.
- Copy `Grace` onto `Table` at `New`. Alternative: store `*Config` on the table. Rejected: a pointer would let the caller mutate after create.
- Delete `reclaim/default.go`. Alternative: leave an empty file or a deprecated `Default`. Rejected: the ticket removes the singleton.
- Probe: package-level `var table = New(Config{Grace: DefaultGrace})`. Alternative: a table per plugin `New`. Rejected: two routes would not share an incarnation.
- `Config` next to `Table` in `table.go`. Alternative: `config.go`. Rejected: DTO that feeds one type may sit next to it; one field does not earn a file.
- Drop `TestDefault_*` that exist only for the lazy singleton. Sharing stays on two `Open`s of one `New` table.

## Risks / Trade-offs

- [External importers still call `reclaim.Open`] → This module's only product caller is `e2e/reclaimprobe`. README and usage change in the same apply. Rollback is revert.
- [Probe `init` table uses `DefaultGrace`] → Matches dest process-table grace so Pester orphan/reclaim timings stay valid. Do not shorten probe grace without changing those waits.

## Migration Plan

Library **BREAKING**. Callers replace `reclaim.Open` with a held `*Table` and `table.Open`. Tests replace `NewTable` with `New(Config{Grace: …})` and package `Reset` with `tab.Reset()`. Rollback is revert.

## Open Questions

None. Assumed constructor and ownership policy live on `devstate/explore.md`.
