# Standards

1. [judgement] Duplicated Code — `scripts/integration-tests.Tests.ps1:136` — overlapping Redis and Dragonfly Its copy the same Value/MGet distinctness assertions, differing only by URL
   → Extract one assertion helper both Its call (keep separate It names so both engines stay visible)
   Status: skipped
   Argument: judgement; Redis and Dragonfly must stay separate Its so both-engine coverage stays visible.
