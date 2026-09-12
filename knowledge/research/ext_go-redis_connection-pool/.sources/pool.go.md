---
url: https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/pool/pool.go
title: go-redis ConnPool Get/Put/waitTurn
fetched: 2026-09-11
authority: source
ref: github.com/redis/go-redis@7f3b3dffde59329db9fa7a71d650ef373affb599:internal/pool/pool.go
---

ErrPoolTimeout = `"redis: connection pool timeout"`.
ErrPoolExhausted = `"redis: connection pool exhausted"`.
ErrPoolTryFull = `"redis: connection pool has no free turn"` (TryGet only; not a timeout).
errConnEvictedIdle: Put evicted because idle pool at MaxIdleConns.

NewConnPool: `semaphore: internal.NewFastSemaphore(opt.PoolSize)`. Size() returns PoolSize (capacity).

Get → getConn(wait=true):
1. waitTurn: TryAcquire; else Acquire(ctx, PoolTimeout, ErrPoolTimeout). Timeout → ErrPoolTimeout and Stats.Timeouts++. ctx cancel → ctx.Err().
2. popIdle. Hit: return existing conn. Miss: queuedNewConn (dial under MaxConcurrentDials).
Idle connections do not hold a semaphore token. Put calls freeTurn after returning or closing.

newConn MaxActiveConns > 0 && poolSize >= MaxActiveConns → ErrPoolExhausted (no wait).
If pooled and poolSize already >= PoolSize, cn.pooled=false (closed on next Put).

Put idle path: if MaxIdleConns == 0 OR idleConnsLen < MaxIdleConns, append to idleConns. Else shouldCloseConn=true, ProcessOnRemove(..., errConnEvictedIdle), removeConn, closeConn. That eviction does not wait for poolSize == PoolSize.
