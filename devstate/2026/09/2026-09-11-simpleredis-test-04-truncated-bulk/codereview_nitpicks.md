# Nitpicks

1. [hard] Name for the scope — `simpleredis/simpleredis_test.go:313` — `n` is a letter placeholder for the idle-list length this body returns
   → Name it `idleCount` (the role under the mutex)
   Status: done
   Argument: renamed `n` to `idleCount` in `pooledIdle` (f36aed0).
