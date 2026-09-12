# Nitpicks

1. [hard] Name for the scope — `simpleredis/interpretedcost_test.go:188` — `fn` hides that the value is the convertprobe loop to interpret (`CopyLoop` / `CopyCallLoop` / `UnsafeLoop`)
   → Rename to `loopName`
   Status: done
   Argument: f0460da `fn` → `loopName` on benchmarkYaegiConvert.
2. [hard] Name for the scope — `simpleredis/interpretedcost_test.go:271` — `fn` hides that the value is the encodeprobe loop to interpret (`BufioLoop` / `SingleWriteLoop`)
   → Rename to `loopName`
   Status: done
   Argument: f0460da `fn` → `loopName` on benchmarkYaegiEncode.
