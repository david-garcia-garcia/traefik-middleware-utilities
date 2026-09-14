# Standards

1. [hard] Leave a trail — `reclaim/repro_enforce_panic_close_test.go:23` — the new Sleep-panic test still says `drop` "recovers that panic through endBusySlot, which unmaps the key and closes ready before dispose"; this change routes that path through `endBusyAfterPanic`
   → Comment the job the test locks (Sleep panic plus stored `EnforceCloseBeforeOpen` waits for Close), not dest's helper
   Status: done
   Argument: Sleep-panic comment now names the Close-before-create job.
2. [hard] Leave a trail — `reclaim/repro_enforce_panic_close_test.go:89` — the new Wake-panic test still says `reclaimLocked` "recovers the panic through endBusySlot (unmap, close ready) and only then runs dispose"; after this change it calls `endBusyAfterPanic`
   → Comment the job the test locks (Wake panic plus stored flag waits for Close; the reclaiming Open still returns the wrapping error), not dest's helper
   Status: done
   Argument: Wake-panic comment now names the Close-before-create job and the reclaiming Open error.
