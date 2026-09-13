## Context

See proposal.md for why. The table already parks a key `slotBusy` with `ready` open, then runs `create` / Sleep / Wake outside `t.mu`. Create that *returns* an error already sets `createErr`, `slotGone`, unmaps, and closes `ready`. Panic and nil `create` skip that branch. `reclaimLocked` returns only `any`; `Open` always pairs it with a nil error. `dropWhenDone` uses `context.AfterFunc` with no recover. `runSleep` / `runWake` / `runClose` must stay thin (a recover there would look like success).

## Goals / Non-Goals

**Goals:**
- Recover at the protocol owners so the slot is never left half-finished.
- Tests that fail on current master (hang / leftover busy / AfterFunc child crash), then the recover, then they pass.

**Non-Goals:**
- Silent swallow inside `runSleep` / `runWake` / `runClose`.
- Re-panic after unstick.
- Canceled-ctx Open returns a closed value.
- Unmap-before-Close overlap (`expire`).

## Decisions

- Recover in `put`, `drop`, `reclaimLocked`, `dispose`, and `Reset` — not in the runners. Alternative: wrap only `AfterFunc`. Rejected: create/Wake panics are on the Open goroutine and still leave the slot busy.
- Nil `create` and create panic share the existing create-error branch. Wrap panic as `fmt.Errorf("reclaim: create %q: panic: %v", key, recovered)`. Nil create: `fmt.Errorf("reclaim: create %q: nil create", key)`.
- Sleep panic: Close, unmap, `slotGone`, `close(ready)`. Do not set `createErr` (waiters create). Skip `reclaim_orphan`. Still Close then `reclaim_dispose`. Sleep wrap exists for logs/tests if returned; AfterFunc has no Open to return to.
- Wake panic: set `createErr`, Close, unmap, `slotGone`, `close(ready)`. Change unexported `reclaimLocked` to `(any, error)`. Alternative: keep returning the pointer. Rejected: a panicking hook is broken, not a resume.
- Close panic: recover in `dispose` only. `ready` already closed.
- AfterFunc crash test: parent `go test` spawns `exec.Command(os.Args[0], "-test.run=^Name$")` with a child env flag. Parent asserts the child's exit. Alternative: `t.Setenv` plus in-process recover — that would not prove AfterFunc can kill the process.
- Land failing tests first, then the recover, as two commits.

## Risks / Trade-offs

- [Wake now returns an error] → Public `Open` already returns `(any, error)`. Callers that ignored create errors still ignore this one. Spec and usage packet stop saying wake cannot fail in any case.
- [Sleep panic skips orphan log] → `reclaim_orphan` always preceding dispose no longer holds on this abort path. Spec the exception. Mitigation: tests assert Close ran and later Open creates.
- [Subprocess test is slower / Windows-sensitive] → Use a tight `-test.run` regexp and a short child sleep after cancel. Measured on this machine: child `exit status 2` in ~50ms.

## Migration Plan

Library change. No stored data. Callers that already handle `Open` errors need no migration. Rollback is revert.

## Open Questions

None. Assumed wrap strings and waiter/`createErr` policy live on `devstate/explore.md`.
