# Explore
IssueKey: 2026-09-13-reclaim-bug-hook-panic-busy

## Concepts

**Table / slot / ready / slotBusy**: `reclaim/table.go` maps a key `slotBusy` with `ready` open before `create`, Sleep, or Wake. Waiters block on `ready`. The table owns that protocol; hooks do not.

**put / drop / reclaimLocked / dispose / Reset**: the four protocol owners plus tests-only Reset. `runSleep` / `runWake` / `runClose` only invoke the stored funcs. `dropWhenDone` uses `context.AfterFunc` for cancellable holders.

**Incarnation**: one value from create to close. Sleep panic must not park it asleep. Wake panic is a broken hook, not a resume failure — this Open returns an error; later Open creates.

**Public Open**: already `(any, error)`. `reclaimLocked` today returns only `any` and Open always pairs it with `nil` error (`reclaim/table.go`). Production caller: `e2e/reclaimprobe/plugin.go` (already checks `err`). Pass-through: `reclaim/default.go` `Open`. Tests: `reclaim/table_test.go`, `reclaim/yaegi_test.go`.

## Decisions

- Recover in `put`, `drop`, `reclaimLocked`, `dispose`, and `Reset`. Do not recover inside `runSleep` / `runWake` / `runClose` (would swallow as success). Do not re-panic after unsticking.
- `put`: nil `create` or panic in `create` matches the existing create-error branch: `createErr`, `slotGone`, unmap, `close(ready)`, return error. No Close.
- `drop` (Sleep panic): Close, unmap, `slotGone`, `close(ready)`. Do not set `createErr` — waiters create a new incarnation.
- `reclaimLocked` (Wake panic): Close, unmap, `slotGone`, `close(ready)`, set `createErr` so this Open and concurrent waiters get the wrapped error. Change unexported `reclaimLocked` to return `(any, error)`.
- `dispose` (Close panic): recover so AfterFunc cannot kill the process. `close(ready)` already happened before Close.
- Tests first in `reclaim/table_test.go`: post-fix assertions (second/third Open returns; no leftover busy key; Close ran where agreed; AfterFunc child survives). They fail on current master by hang / leftover busy / process crash. Existing reclaim tests stay green. Do not land tests that assert leftover busy as success.
- AfterFunc process-crash proof: parent `go test` spawns `exec.Command(os.Args[0], "-test.run=^Name$")` with a child env flag. Child runs Sleep panic on a cancellable ctx. Parent asserts the child exit. Reproduced: child `exit status 2` today.
- Out of scope stays out: canceled-ctx Open returns closed value; unmap-before-Close overlap.
- Specs/docs: fold panic/nil-create and panicking-hook end into existing `std_go_reclaim_value-lifecycle` and `std_go_reclaim_context-lease`; update `knowledge/devdocs/std_go_reclaim.md` Wake/Sleep language. No new spec family.
- No identity reconstruction in this work.

## Reproduction (origin/master, throwaway then deleted)

Worktree HEAD `8751428` (prepare only; table matches master). Throwaway `TestThrowaway_*` in `reclaim/`, then deleted.

- 2a create panic: recover; key leftover `slotBusy`; second Open with 200ms timeout hung.
- 2b nil `create`: nil deref panic; leftover mapped; second Open hung.
- 2c Sleep panic on AfterFunc: child `exit status 2` (unrecovered goroutine panic).
- 2d Wake panic after Sleep: recover; third Open hung; Close had not run.

## Open questions

- Q: What wrap string for Sleep / Wake / Close panics? Ticket specified only `fmt.Errorf("reclaim: create %q: panic: %v", key, recovered)`.
  Rank: additive asked — new error text on paths this change owns; criterion names recover and the create wrap
  Decision: assumed — same shape: `reclaim: sleep %q: panic: %v`, `reclaim: wake %q: panic: %v`. Close panic has no Open to return to; recover only (no wrap to a waiter).
  By: explore

- Q: What error for nil `create`?
  Rank: additive asked — nil create is the same as create returning an error; wrap text not given
  Decision: assumed — `fmt.Errorf("reclaim: create %q: nil create", key)` so waiters replay `createErr` and a later Open can create.
  By: explore

- Q: Do concurrent waiters on a panicking Wake receive `createErr`, or only the Open inside `reclaimLocked`?
  Rank: additive asked — ticket says this Open returns the wrapped panic and later Open can create
  Decision: assumed — set `createErr` like create failure so everyone parked on this `ready` gets the same error; key unmapped so a subsequent Open creates.
  By: explore

- Q: After a Sleep panic, emit `reclaim_orphan` / `reclaim_dispose`?
  Rank: additive incidental — log lines are a means; ticket lists Close/unmap/slotGone/close(ready), not msgs
  Decision: assumed — do not emit `reclaim_orphan` (Sleep did not return). Still Close, then `reclaim_dispose` after Close returns or after a recovered Close panic. Abort is not a successful sleep.
  By: explore

- Q: How is the AfterFunc process-crash test isolated so `go test` of the package does not die with the child?
  Rank: additive asked — product tests must fail on current master for process panic on AfterFunc Sleep
  Decision: resolved — subprocess via `exec.Command(os.Args[0], "-test.run=^…$")` and a child env flag. Parent asserts non-zero exit before the fix and zero after. Measured: child `exit status 2` on master.
  By: explore
