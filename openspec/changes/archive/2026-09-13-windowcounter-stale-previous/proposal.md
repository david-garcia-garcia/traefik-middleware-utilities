## Why

On DestBranch, buffered Take GET-refreshes the current window after a flush (`localDelta == 0`) but never GET-refreshes the previous-window key once it is in `l.windows`. After A.Sleep then B.Sleep, Redis holds 2 while A's `redisKnown` for that key stays 1. Next-window Take (weight 1) admits at estimated 2 instead of deny at 3.

## What Changes

- Failing repro first (`windowcounter/repro_stale_previous_test.go`): two buffered clients, one Take each, A.Sleep then B.Sleep, next-window Take must deny at estimated 3.
- On buffered Take, if the previous key is in memory and `localDelta == 0`, GET and set `redisKnown`. If `localDelta > 0`, keep memory.
- Do not INCR previous. Do not GET previous on every Peek. Do not fold this GET into Redis-down / pending-delta outage policy.
- Failed previous GET on Take returns like current `windowLocked` GET.
- Spec: later Take after a window roll still sees the shared previous count. Peek may lag previous until Take refreshes.

## Capabilities

### New Capabilities

- None. This is a delta on the existing windowcounter leaves, not a new package or spec family.

### Modified Capabilities

- `std_go_windowcounter_sync-flush`: After both instances flush, a later Take on either instance sees the shared count including when that key has rolled to previous. Buffered Take GETs previous when that key is in memory and `local_delta` is 0; Peek MUST NOT GET previous every call because `local_delta` is 0.
- `std_go_windowcounter_sliding-take`: Dump at the window boundary does not double for buffered two-client Sleep order (deny at estimated 3 when current=1 and previous Redis=2 at weight 1). Callers still own the opaque key.

## Impact

- `windowcounter/limiter.go` (Take-only previous GET; Peek stays on `bufferedCountLocked`).
- `windowcounter/repro_stale_previous_test.go` (unit fake Redis).
- `openspec/specs/std_go_windowcounter_sync-flush/spec.md` and `std_go_windowcounter_sliding-take/spec.md` (after archive).
- `knowledge/devdocs/std_go_windowcounter.md` (Take GET previous when `localDelta == 0`).
- No exact-mode change. No INCR previous. No live e2e case. No Redis-down policy change. No other windowcounter bugs.
