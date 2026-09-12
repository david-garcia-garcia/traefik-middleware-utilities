---
url: https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/pool/pool_test.go
title: ConnPool wait and PoolTimeout tests
fetched: 2026-09-11
authority: source
ref: github.com/redis/go-redis@7f3b3dffde59329db9fa7a71d650ef373affb599:internal/pool/pool_test.go
---

"wait": PoolSize=1. First Get holds the only turn. Second Get blocks until Put; WaitCount=1.

"timeout": PoolSize=1, PoolTimeout=1s. First Get succeeds. Second Get matches ErrPoolTimeout. Stats.Timeouts becomes 1. After Put, a later Get succeeds.

A later test with PoolTimeout=150ms expects Get while the turn is held to return ErrPoolTimeout in ~150ms.
