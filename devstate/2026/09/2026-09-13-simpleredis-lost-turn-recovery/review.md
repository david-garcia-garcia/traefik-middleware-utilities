# Review journal

## prepare (2026-09-13T08:50:29Z)
phase: prepare
findings: none
fixed: none
skipped: explore and later phases (prepare only)

## explore (2026-09-13T09:02:12Z)
phase: explore
findings: reproduced TestBugLostInUseTurnBricksPoolPermanently on DestBranch (turns=0/2, idle=0, accepts=2, next Get redis:unreachable)
fixed: none (think, do not implement)
skipped: refill not applied; PR 29 defer-release not reversed

## implement (2026-09-13T09:28:36Z)
phase: implement
findings: none
fixed: heldSockets + LostTurns refill at pool-wait; doWithHeldSocket without defer-release; default-suite regression tests
skipped: defer sr.release (PR 29); closing leaked TCP fds

## codereview (2026-09-13T09:28:36Z)
phase: codereview
findings: Standards 2, Nitpicks 1, Performance 1, Coverage 3 (1 skipped judgement)
fixed: defer around dial; serialize refill under turnRecoverMu through dial; panic-in-do and hung-dial tests; rename test client redis
skipped: two-waiter LostTurns equality (judgement)

## devdocsimpact (2026-09-13T09:28:36Z)
phase: devdocsimpact
findings: stale-usage SimpleRedis — std_go_simpleredis gotcha already names LostTurns refill
fixed: none this phase (packet already produced)
skipped: none

## archive (2026-09-13T09:28:36Z)
phase: archive
findings: none
fixed: folded LostTurns into std_go_simpleredis_tcp-session; moved change to archive/2026-09-13-simpleredis-lost-turn-recovery
skipped: none

## pullrequest (2026-09-13T09:39:20Z)
phase: pullrequest
findings: none
fixed: title dropped WIP; CI 34749762357 all eight checks succeeded; live pool-wait now holds inside do
skipped: none