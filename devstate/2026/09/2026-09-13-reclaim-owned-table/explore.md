# Explore
IssueKey: 2026-09-13-reclaim-owned-table

## Concepts

DestBranch `reclaim` owns two construction stories: a process singleton (`Default`, package `Open`, `Reset` / `ResetWith` in `reclaim/default.go`) and an instance constructor `NewTable(grace)` (`reclaim/table.go:98`). Production-shaped usage (`README.md`, `knowledge/devdocs/std_go_reclaim.md`, `e2e/reclaimprobe/plugin.go:55`) goes through the singleton. Tests mostly already hold a `*Table`.

The ticket removes the singleton. Sharing across Traefik `New` (cancel old ctx, open the same key inside grace) stays a **caller** job: one `*Table` the plugin package keeps, not a table `reclaim` keeps for the process.

```
DestBranch                         After
─────────                         ─────
plugin New ──► reclaim.Open       plugin pkg table = New(Config{…})
                 │                plugin New ──► table.Open
                 ▼
            Default() *Table
```

Table-level config on dest is only `grace` (negative → `DefaultGrace`, zero stays zero). `Hooks.EnforceCloseBeforeOpen` is per-incarnation, not table config (`knowledge/devdocs/std_go_reclaim.md` Language **Hooks**).

Neighbor constructors already take a value `Config` copied at `New`: `simpleredis.New(cfg Config)`, `backendbackoff.New(cfg Config)`.

## Decisions

- Remove `Default`, package `Open`, `Reset`, `ResetWith`. Delete `reclaim/default.go`.
- Replace `NewTable(grace)` with `New(Config)` by value. `New` copies `Config.Grace` onto the table. No setter. Later writes to the caller's `Config` do not change an existing table.
- `Config` has one field, `Grace` — dest's only table knob. Do not add knobs. Negative `Grace` → `DefaultGrace`. Zero `Grace` stays 0.
- `Config` sits next to `Table` in `reclaim/table.go` (DTO that feeds one type).
- `e2e/reclaimprobe` holds a package-level table created once with `New(Config{Grace: DefaultGrace})`. Plugin `New` calls that table's `Open`. A table per plugin constructor would drop reload sharing.
- Drop `TestDefault_*` that exist only for the lazy singleton. Sharing stays covered by two `Open`s on one `New` table. `Table.Reset` stays.
- Spec `std_go_reclaim_context-lease` drops the process-table SHALL; instance `Open` on one table still shares an incarnation. Usage/README/`std_go_backendbackoff.md` say "store in a table the caller owns", not `reclaim.Open`.
- No third-party clone: this is an in-tree constructor move. Yaegi constraint (non-generic `Table`, `create` with no args) is unchanged.

## Open questions

- Q: Is `New` `New(grace time.Duration)` or `New(Config)` with `Grace` as a field?
  Rank: bounded asked — changes existing `NewTable`; searched `*.go` for `NewTable(`: `reclaim/table.go` 1, `reclaim/default.go` 2, `reclaim/table_test.go` 49, `reclaim/yaegi_test.go` 1 (53 sites); Desired #3 names `New()` that takes config parameters
  Decision: assumed — `New(Config)` by value; `Grace` only; copy at `New`; no setter; negative → `DefaultGrace`; zero stays 0. Matches `simpleredis.New` / `backendbackoff.New`. Delete `NewTable`.
  By: explore

- Q: Is `reclaim/default.go` deleted or left without the singleton?
  Rank: bounded asked — Desired #1 names remove `Default` / package `Open` / `Reset` / `ResetWith` in that file; product Go callers of the singleton searched `*.go` for `reclaim.(Default|Open|Reset|ResetWith)(`: `e2e/reclaimprobe/plugin.go:55` only; tests: three `TestDefault_*` in `reclaim/table_test.go`
  Decision: assumed — delete `reclaim/default.go`.
  By: explore

- Q: How does `e2e/reclaimprobe` hold the table so two Traefik `New`s still share an incarnation within grace?
  Rank: bounded asked — Desired #2 names callers manage instance and lifetime; Unknowns names this probe; 1 call site `e2e/reclaimprobe/plugin.go:55`
  Decision: assumed — package-level `var` table in `reclaimprobe`, `New(Config{Grace: DefaultGrace})` once; plugin `New` calls `table.Open`. Not a table per constructor.
  By: explore

- Q: Does `Config` live in `table.go` or a new `config.go`?
  Rank: additive asked — new type this change creates; Desired #3 names constructor config; DTO-next-to-owner is allowed (`skill:opd-commandments:One job, one owner`)
  Decision: assumed — `Config` next to `Table` in `reclaim/table.go`. No `reclaim/config.go`.
  By: explore
