---
url: https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/pool/pool.go
title: go-redis isHealthyConn and idle Get
fetched: 2026-09-13
authority: source
ref: github.com/redis/go-redis@7f3b3dffde59329db9fa7a71d650ef373affb599:internal/pool/pool.go
---

`getConn` after `popIdle`: if `!p.isHealthyConn(cn, nowNs)` then `CloseConn(..., CloseReasonStale, ...)` and retry another idle (`retryIdle`).

`isHealthyConn(cn *Conn, nowNs int64) bool`, cheapest first:
1. `ConnMaxLifetime > 0` and `cn.expiresAt` before `nowNs` → false.
2. `ConnMaxIdleTime > 0` and `nowNs-cn.UsedAtNs() >= ConnMaxIdleTime` → false.
3. `connCheck(cn.getNetConn())`.
   - `err == nil` → `SetUsedAtNs(nowNs)`, return true.
   - `PushNotificationsEnabled && err == errUnexpectedRead`: `cn.PeekReplyTypeForCheck()`; if `replyType == proto.RespPush` → true (debug log only). Else false.
   - any other err → false.

`Close` health-checks idle conns with `isHealthyConn` before `closeConn` (fd invalid after close would make `connCheck` fail with EBADF).
