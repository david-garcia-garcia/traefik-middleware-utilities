# Nitpicks

1. [hard] Name for the scope — `reclaim/table.go:126` — `sleepValue(hooks Hooks)` still names a value; the body only calls `hooks.Sleep`
   → Rename to the job (`runSleep`)
   Status: done
   Argument: Renamed to runSleep; callers pass stored Hooks.
2. [hard] Name for the scope — `reclaim/table.go:133` — `wakeValue(hooks Hooks)` still names a value; the body only calls `hooks.Wake`
   → Rename to the job (`runWake`)
   Status: done
   Argument: Renamed to runWake.
3. [hard] Name for the scope — `reclaim/table.go:140` — `closeValue(hooks Hooks)` still names a value; the body only calls `hooks.Close`
   → Rename to the job (`runClose`)
   Status: done
   Argument: Renamed to runClose.
4. [hard] Name for the scope — `e2e/reclaimprobe/plugin.go:54` — `storedValue` and `stored` are two stems for create capture vs Open’s `any`; the body only needs the created probe’s id
   → Name the capture for that role (`created` / `probe`); keep `stored` for Open’s return
   Status: done
   Argument: Capture is `created`; Open return stays `stored`.
5. [hard] Name for the scope — `reclaim/yaegi_test.go:47` — `evalHookprobe` locals `i` and `v` are placeholders for the interpreter and the Eval result
   → `interpreter` and `result` (or `evaluated`)
   Status: done
   Argument: Locals are `interpreter` and `evaluated`.
6. [hard] Name for the scope — `reclaim/yaegi_test.go:179` — `waitCount(n *atomic.Int32)` names the counter `n`
   → Name the role (`count` / `hookCount`)
   Status: done
   Argument: Parameter is `hookCount`.
