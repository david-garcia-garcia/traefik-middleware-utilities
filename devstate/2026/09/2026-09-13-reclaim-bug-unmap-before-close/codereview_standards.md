# Standards

1. [hard] One job, one owner — `reclaim/table.go:322` — `drop`'s zero-grace path copies `expire`'s Close, unmap, `slotGone`, `close(ready)`, `MsgDispose` tail; two writers now own the production end
   ```
   runClose(storedHooks)
   t.mu.Lock()
   if t.items[key] == incarnation {
   	delete(t.items, key)
   }
   incarnation.state = slotGone
   close(incarnation.ready)
   t.mu.Unlock()
   logger.Debug(MsgDispose, "key", key)
   ```
   → Extract one owner and call it from `drop` (already `slotBusy`) and from `expire` after the asleep-to-busy claim; leave tests-only `Reset` on `dispose`
   Status: done
   Argument: extracted `unmapAfterClose` for drop and expire; Reset still inlines Close-then-log (`80328a9`).

2. [hard] Leave a trail — `reclaim/table_test.go:816` — both overlap tests are multi-block (hold Close, park second Open, release, assert create) with only a function comment; sibling `TestTable_ZeroGraceRacingOpenIsPlainBind` intros the park window
   ```
   cancel()
   <-closeEntered
   ...
   case <-time.After(200 * time.Millisecond):
   ...
   close(releaseClose)
   ```
   → Intro hold-Close, park second Open, release, and create-after-Close on each test
   Status: done
   Argument: intro comments on hold-Close, park, and create-after-Close in both overlap tests (`80328a9`).
