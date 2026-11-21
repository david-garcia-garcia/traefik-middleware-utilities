# Issues

- [ ] note large  `knowledge/debt/2026-09-14-reclaim-unreachable-defensive-paths.md`
  Why: waitCtx Done branch and drop busy-wait are unreachable from the table; the waitCtx poll fast path is redundant.
- [ ] note large  `knowledge/debt/2026-09-14-reclaim-whitebox-test-lock-coupling.md`
  Why: table_gaps_test.go takes tab.mu and slot internals; a sibling locking rewrite can break it at merge.
