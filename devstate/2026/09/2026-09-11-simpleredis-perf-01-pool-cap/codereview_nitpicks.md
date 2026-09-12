# Nitpicks

1. [hard] Name for the scope — `scripts/integration-tests.Tests.ps1:145` — `$live` is the `/proc` ESTABLISHED count from `Count-Established6379`, but the same block comment names the bound `liveCap` / `poolSize`, so `$live` reads as the cap quantity rather than the measured socket count
   → `$established` (or `$establishedCount`) for the regex count; keep `liveCap` language for the pool bound only
   Status: done
   Argument: `$established` is the /proc ESTABLISHED count; pool bound stays poolSize/liveCap.
