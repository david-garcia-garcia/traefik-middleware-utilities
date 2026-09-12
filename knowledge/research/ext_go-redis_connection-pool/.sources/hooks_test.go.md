---
url: https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/internal/pool/hooks_test.go
title: Put evicts when idle is full under PoolSize
fetched: 2026-09-11
authority: source
ref: github.com/redis/go-redis@7f3b3dffde59329db9fa7a71d650ef373affb599:internal/pool/hooks_test.go
---

TestPutConnEvictsOnIdleFullInvokesOnRemove: PoolSize=2, MaxIdleConns=1. Two newConn (pooled). First putConnWithoutTurn pools (idle=1). Second Put evicts because idle is full. OnRemove fires once; StaleConns=1.

Idle overflow therefore closes a live socket while PoolSize still has room for a second idle.
