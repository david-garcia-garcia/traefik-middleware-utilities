# Reclaim-owned SimpleRedis idle reaper

IssueKey: 2026-09-12-simpleredis-risk-02-no-idle-reaper
Size: large
Action: note

## Why this follow-up
A client that never borrows again never runs `takeIdleConn`, so a full-list sweep cannot close sockets after `IdleTimeout`. Resting Traefik workers then keep up to `PoolSize` established Redis sockets until process death or `Close`.

## Why it was not taken
A background ticker is only safe if something stops it. Dest product code does not call `SimpleRedis.Close` (`windowcounter` limiter `Close` leaves the injected client open; the SimpleRedis Traefik probe never `Close`s). Wiring `reclaim.Hooks{Close}` plus a reaper is a lifecycle change across callers, not this ticket’s sweep.

## Risks
Quiet clients keep holding Redis `maxclients` slots and 8 KiB bufio pairs until reload or process exit. Combining sweep with a later reaper is still valid; the sweep does not block that work.

## Context
Current: `simpleredis/pool.go` `takeIdleConn` (this change: sweep on borrow). `reclaim` already has `Hooks.Close`.
Proposed: `New` starts one reaper goroutine stopped by `Close`, and product `New` paths that own a SimpleRedis pass `reclaim.Hooks{Close: client.Close}`.
