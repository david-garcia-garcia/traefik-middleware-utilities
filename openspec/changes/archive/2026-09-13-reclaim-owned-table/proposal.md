## Why

The process-wide reclaim table (`Default`, package `Open`) is born at `DefaultGrace` and is the only table production callers can use. Callers that need their own grace, or that must not share keys, cannot own a table. Constructor config already exists on `NewTable`; the singleton makes it useless for production.

## What Changes

- **BREAKING.** Remove `Default`, package `Open`, `Reset`, and `ResetWith`. Delete `reclaim/default.go`.
- **BREAKING.** Replace `NewTable(grace)` with `New(Config)` by value. `Config.Grace` is copied at `New` and MUST NOT change on that table afterwards. Negative grace still becomes `DefaultGrace`. Zero grace stays 0. No new knobs.
- Callers who need a table create it and hold it. Sharing across Traefik `New` is a package-level table in the caller, not a singleton in `reclaim`.
- `e2e/reclaimprobe` holds one table and calls `table.Open` from plugin `New`.
- Fold `std_go_reclaim_context-lease` off the process-table SHALL. Usage, README, and `std_go_backendbackoff.md` follow in implement.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_reclaim_context-lease`: Drop the process-wide `Default` / package `Open` SHALL. A table is created with `New(Config)`; grace is constructor-only. Instance `Open` on one table still shares an incarnation. The Traefik probe opens through a caller-owned table.

## Impact

- `reclaim/default.go` (delete), `reclaim/table.go` (`NewTable` → `New(Config)`), `reclaim/table_test.go` (`TestDefault_*` and every `NewTable`), `reclaim/yaegi_test.go`.
- `e2e/reclaimprobe/plugin.go`, `README.md`, `knowledge/devdocs/std_go_reclaim.md`, `knowledge/devdocs/std_go_backendbackoff.md`.
- No Traefik/Yaegi upgrade. No change to `Open` / `Hooks` / sleep-wake-close. `Table.Reset` stays. Other packages' internals stay out of scope.
