# Nitpicks

1. [hard] Name for the scope — `reclaim/table.go:159` — `failBusySlot` / `hookErr` name a hook failure; the body stores `createErr` and closes ready, `put` passes create errors, and `drop` passes nil so waiters create
   → Name the method for ending the busy slot (`endBusySlot`); name the parameter `createErr`
   Status: done
   Argument: renamed `failBusySlot`/`hookErr` to `endBusySlot`/`createErr`.
