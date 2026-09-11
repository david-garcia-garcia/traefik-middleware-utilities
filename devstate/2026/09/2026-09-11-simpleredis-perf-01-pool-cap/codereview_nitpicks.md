# Nitpicks

1. [hard] Name for the scope — `simpleredis/simpleredis.go:114` — `giveSlot` names a vague `give`; the body puts a token back on `sr.slots` (the inverse of borrow’s `<-sr.slots`), so the identifier reads as allocate
   → `freeSlot` / `returnSlot` (direction this body implements)
   Status: done
   Argument: renamed giveSlot to freeSlot.
2. [hard] Name for the scope — `simpleredis/simpleredis.go:94` — `waitLimit` drops that the duration is the pool-slot wait; sibling `liveCap` names its quantity
   → `slotWait` / `poolWait`
   Status: done
   Argument: renamed waitLimit to slotWait.
3. [hard] Name for the scope — `simpleredis/simpleredis.go:106` — `capSize := sr.liveCap()` renames the live cap to a second stem; the next lines still use that bound as channel capacity and fill count
   → Keep the stem (`liveCap := sr.liveCap()`)
   Status: done
   Argument: capSize restemmed to liveCap.
4. [hard] Name for the scope — `simpleredis/live_test.go:172` — local `n` is a letter placeholder for the ESTABLISHED count
   → `established` / `count`
   Status: done
   Argument: local n renamed to established.
