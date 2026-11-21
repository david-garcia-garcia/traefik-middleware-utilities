# Spec

1. [wrong] `openspec/changes/reclaim-close-before-unmap/specs/std_go_reclaim_context-lease/spec.md` Requirement: Lifecycle events are logged — `reclaim_dispose` for the previous incarnation SHALL precede `reclaim_put` of the next
   `reclaim/table.go:326-333` — zero-grace `drop` unmaps, sets `slotGone`, and `close(ready)` then logs dispose; a `slotBusy` waiter can `put` in between
   → Log `reclaim_dispose` after Close returns and before unmap/`close(ready)`, still outside `t.mu`
   Status: done
   Argument: `unmapAfterClose` logs dispose then closes ready so waiters cannot put first (`80328a9`).
