# Nitpicks

1. [hard] Name for the scope — `windowcounter/repro_peek_take_boundary_test.go:45` — `reproPeekAllowsWhenNextTakeDenies` names Peek/Take occupancy `allowedP`, `estP`, `allowedT`, `estT`; the body only uses Peek allowed/estimated and the next Take's allowed/estimated
   → `peekAllowed`, `peekEstimated`, `takeAllowed`, `takeEstimated`
   Status: done
   Argument: renamed to peekAllowed, peekEstimated, takeAllowed, takeEstimated (matches TestPeek_AgreesWithTakeBeforeIncrement).
