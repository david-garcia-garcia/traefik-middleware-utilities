# Standards

1. [hard] Leave a trail — `reclaim/table.go:146` — job comment still says `reclaim_dispose` means Close has returned; the recover this change added emits `MsgDispose` when Close panics
   → State dispose after Close returns or after a recovered Close panic
   Status: done
   Argument: dispose comment names recovered Close panic.
2. [hard] Leave a trail — `reclaim/table.go:324` — `drop` (and `Table` at 52–55) still says sleep, orphan, grace, close, dispose cannot be reordered; Sleep panic skips orphan and grace
   → Name the Sleep-panic abort: Close, unmap, no orphan
   Status: done
   Argument: Table and drop comments name the Sleep-panic abort.
3. [hard] Leave a trail — `reclaim/table.go:409` — `Reset` still says orphan precedes dispose; Sleep panic skips orphan
   → Say orphan precedes dispose only when Sleep returns
   Status: done
   Argument: Reset comment: orphan precedes dispose when Sleep returns.
4. [hard] Leave a trail — `knowledge/devdocs/std_go_reclaim.md:38` — Close still says the table waits until Close has returned before `reclaim_dispose`; Close panic still emits it
   → Match the spec: emit `reclaim_dispose` after Close returns or after a recovered Close panic
   Status: done
   Argument: Close language matches recovered panic emit.
