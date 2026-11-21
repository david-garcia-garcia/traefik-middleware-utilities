# Unmap before Close — next create runs while the previous Close is still in flight

Bug: unmap before Close — next create runs while the previous Close is still in flight. Zero grace (and expire after positive grace) deletes the map entry then Close runs outside t.mu. Concurrent Open sees the key absent and create()s while old Close is blocked.
Reproduced 5/5: NewTable(0), Close hook signals then blocks; cancel last holder; Open same key; create of incarnation 2 started while Close of 1 still blocked.

Agreed how (implement this):
Close of an incarnation must finish before the key is absent for a new create.
Keep the key mapped in a non-reclaimable state (slotBusy) for the whole Close, then unmap and close(ready). Open already waits on slotBusy; after ready it sees the key gone and creates.
- Zero grace: after Sleep, do not publish slotAsleep and do not unmap. Close as part of that same busy transition, then unmap / close(ready).
- expire: do not delete until Close returns. Switch to slotBusy (not asleep) for the Close window so a racing Open cannot reclaim.
- Do not run Close under t.mu.
- Do not leave the slot asleep during Close.
- Reset is tests-only and already must not race Open; same Close-before-unmap ordering only if it is cheap and keeps existing Reset tests.

Tests first, then fix:
1. Land a product test that FAILS on current master (create of #2 starts while Close of #1 is blocked). Then implement. Then it PASSES (second Open waits until Close returns; then create). Existing reclaim tests stay green.
2. Do not fix the other two reclaim bugs (canceled-ctx disposed return; hook panic bricks key).

Example (today this PASSES because it logs CONFIRMED when overlap happens — rewrite so it FAILS on overlap / PASSES when Open waits for Close):
A compiled table test with NewTable(0), Close hook that signals then blocks, cancel last holder, Open the same key. Fail if create of incarnation 2 ran while Close of 1 was blocked. After the fix, Open still waits for Close (timeout path), then release Close. Also cover expire after a positive grace if cheap (same unmap-then-Close in expire).
