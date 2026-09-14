# Explore
IssueKey: 2026-09-14-reclaim-ending-path-test-coverage

## Concepts

`reclaim.Table` is a keyed store of `any` plus holder contexts. One incarnation is `create -> (sleep -> wake)* -> sleep -> close`. Ending paths (last-holder drop, grace expire, Sleep/Wake panic, tests-only `Reset`) must sleep before close. `Hooks.EnforceCloseBeforeOpen` keeps the key mapped across Close so a later `Open` waits; the documented tests-only exception is that `Reset` unmaps first regardless of that flag.

Coverage on this worktree (HEAD = empty start commit on `origin/master` e9ac73c) was measured, not taken from the ticket:

```
go test -count=1 -covermode=atomic -coverprofile=<tmp>.cover -timeout 10m ./reclaim/
coverage: 94.4% of statements
```

Nine profile rows have a third-column count of 0:

| Profile row | Function | Why nothing in `./reclaim/` reached it |
| --- | --- | --- |
| `table.go:144.37`, `:146.15`, `:148.19` (3 blocks) | `waitCtx` | `dropWhenDone` starts `watch` only when `ctx.Done()` is nil (`reclaim/table.go` L410–415). `watch` is `waitCtx`'s only caller. The `ctx.Done() != nil` select is unreachable from the table. |
| `table.go:322.17`, `:324.35`, `:327.4` (3 blocks) | `Open` `case slotGone` | Every ending path unmaps or replaces the key inside the same `t.mu` hold except `Reset`, which swaps `t.items` and only then ends each slot. A `put` that maps its slot back in between leaves a mapped, gone incarnation. That window is not reliably schedulable. |
| `table.go:437.36` (4 statements) | `drop` busy-wait | A busy slot only ever has holders that have not bound yet, so no pending drop can meet one under an ordinary schedule. |
| `table.go:506.64` (2 statements) | `expire` early return | The armed `AfterFunc` arriving after an `Open` already woke the value, or after another path claimed it. Winning that timer race on purpose is not reliable. |
| `table.go:559.65` (1 statement) | `Reset` Sleep panic | No test panics `Sleep` on `Reset`. |

Two more behaviours the ticket names are **not** zero-count statements:

- `waitCtx`'s non-blocking `select` on `finished` at the top of the polling loop (`table.go:156.19`) already has count 2. It still only wins when both channels are ready and the select picks the ticker; the dedicated test pins that without waiting out a tick.
- `Reset` + `EnforceCloseBeforeOpen` shares the same unmap-first statements as every other `Reset`. The gap is a missing assertion, not a missing statement: nothing combines `Reset` with the flag and a later `Open` while Close is still in flight. Devdocs (`knowledge/devdocs/std_go_reclaim.md` Gotchas) and `std_go_reclaim_value-lifecycle` (`Tests-only Reset MAY unmap first regardless of the field`) already describe it.

Usage packet `knowledge/devdocs/std_go_reclaim.md` is enough to call the subsystem. Language has no gap. Spec hosts already exist: `std_go_reclaim_value-lifecycle` (Reset Sleep-panic skip-orphan; Reset unmap-before-enforce) and `std_go_reclaim_context-lease` (orphan/dispose log order). This run does not invent a third family.

```
                  waitCtx
                     ▲
                     │ only caller
                   watch
                     ▲
                     │ only when Done() == nil
                dropWhenDone ── Done() != nil ──► AfterFunc(drop)
```

## Decisions

- Land `reclaim/table_gaps_test.go` verbatim from the caller. Do not rewrite the seven tests. Do not edit `reclaim/table.go`.
- Keep the three white-box tests (`Open` mapped-gone, `drop` busy-wait, `expire` claimed). They take `tab.mu` and construct `slot` state the same way `mustSlot` / `readState` already do. Do not try to win those races on the scheduler.
- Do not delete the unreachable `waitCtx` Done branch, the `drop` busy-wait, or the redundant poll fast path in this change. Note them as follow-ups (`devstate/issues.md` + `knowledge/debt/`).
- Propose ADDED scenarios on the existing reclaim spec leaves for the two documented Reset contracts that had no test. Do not add a new spec folder.
- Do not coordinate with `2026-09-14-reclaim-bug-finished-race-lock-defer`. Flag the white-box coupling (`tab.mu`, `slot` internals) as a merge-time note.
- CI has no statement-coverage gate (`openspec/specs/std_go_ci_test-suites/spec.md`, `knowledge/devdocs/std_go_test-suites.md`). Acceptance stays: `go test ./reclaim/` green, Docker `go test -race ./reclaim/` green, cover profile 100.0% with zero count-0 rows.

## Open questions

- Q: Should this change delete the unreachable `waitCtx` Done branch and the `drop` busy-wait, or leave a comment that they are unreachable?
  Rank: bounded incidental — two enumerated sites in `reclaim/table.go` (L144–150, L437–443); requirement Out of scope
  Decision: assumed — do not delete or comment them here; note as `knowledge/debt/2026-09-14-reclaim-unreachable-defensive-paths.md`.
  By: explore

- Q: Should this change delete the `waitCtx` polling-loop non-blocking `finished` check (already count 2 on master) as a redundant fast path?
  Rank: bounded incidental — one enumerated site in `reclaim/table.go` (L154–159); requirement Out of scope
  Decision: assumed — keep the statements and land `TestWaitCtx_PollingSeesAnEndedIncarnationFirst` verbatim; same debt file as the unreachable pair.
  By: explore

- Q: Which spec leaf gets the two Reset contracts that had no test (Sleep panic skips orphan and still disposes; Reset unmaps first regardless of `EnforceCloseBeforeOpen`)?
  Rank: additive asked — Desired names spec/devdocs deltas for those two documented contracts; new scenarios on leaves this change does not create
  Decision: assumed — ADDED scenarios on `std_go_reclaim_value-lifecycle` (behaviour) and, if log order needs an explicit Reset-Sleep-panic exception, `std_go_reclaim_context-lease`. FindSpecHost in propose confirms. No new folder.
  By: explore

- Q: Will the parallel locking ticket rename `tab.mu` or `slot` fields before merge and break the white-box tests in `table_gaps_test.go`?
  Rank: additive incidental — no DestBranch rename exists today; the sibling branch is out of this run's scope
  Decision: assumed — do not coordinate; do not touch `table.go`; note `knowledge/debt/2026-09-14-reclaim-whitebox-test-lock-coupling.md`. The sibling PR owns any rename follow-up.
  By: explore
