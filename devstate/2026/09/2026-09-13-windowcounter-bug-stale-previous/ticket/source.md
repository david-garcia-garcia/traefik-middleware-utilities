# Buffered previous window is never GET-refreshed

THIS BUG ONLY.

## Problem
Buffered previous window is never GET-refreshed. `bufferedCountLocked` returns memory if the key is in `l.windows`. `windowLocked` GETs current when `localDelta==0`. After `A.Sleep()` then `B.Sleep()`, Redis holds 2 but A's `redisKnown` stays 1. Next-window Take admits (`estimated=2`) instead of deny at 3.

## Agreed how
On buffered Take, if previous key is in memory and `localDelta==0`, GET and set `redisKnown`. If `localDelta>0`, keep memory. Do not INCR previous. Do not GET previous on every Peek. Do not fold into Redis-down policy. Test must deny at estimated 3.

Bound the ask: only this bug.

Implement order is for later phases, not prepare: failing repro first, then `limiter.go` fix.
