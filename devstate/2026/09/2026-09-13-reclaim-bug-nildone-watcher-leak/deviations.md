# Deviations

- [x] taken  test-only `NewTable` wrapper instead of restoring production `NewTable` or rewriting the reproducer
  Asked: copy `reclaim/repro_nildone_watcher_test.go` verbatim (it calls `NewTable(time.Millisecond)`).
  Instead: leave that file byte-identical and add `NewTable` in `reclaim/table_test.go` as `New(Config{Grace: grace})`.
  Owner: `reclaim/table.go` `New(Config)` (the dest constructor; production `NewTable` was removed in `c960bfe`)
  Why: honouring the letter by putting `NewTable` back on `table.go` would reintroduce a constructor the spec forbids; rewriting the one call in the reproducer would violate the verbatim-file ask. The leak assertion does not depend on the constructor name.
  By: explore
  Requester: not asked
