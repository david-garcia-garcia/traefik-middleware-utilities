---
url: https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/semaphore.go
title: go-redis FastSemaphore wait queue
fetched: 2026-09-11
authority: source
ref: github.com/redis/go-redis@7f3b3dffde59329db9fa7a71d650ef373affb599:internal/semaphore.go
---

FastSemaphore is a buffered `chan struct{}` pre-filled to `capacity` (PoolSize). Acquire = receive; Release = send.

Available() == 0 means every turn is taken by an in-use connection or an in-flight dial.

Acquire(ctx, timeout, timeoutErr):
- ctx already done → ctx.Err()
- fast path: non-blocking receive
- slow path: timer.Reset(timeout); select token / ctx.Done() / timer.C → timeoutErr

Fairness: "eventual fairness (no starvation) but not strict FIFO."
Shape SimpleRedis can copy: `make(chan struct{}, poolSize)` plus `select` with a timer.
