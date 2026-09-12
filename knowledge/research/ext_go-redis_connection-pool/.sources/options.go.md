---
url: https://github.com/redis/go-redis/blob/7f3b3dffde59329db9fa7a71d650ef373affb599/options.go
title: go-redis Options pool fields and init defaults
fetched: 2026-09-11
authority: source
ref: github.com/redis/go-redis@7f3b3dffde59329db9fa7a71d650ef373affb599:options.go
---

Pin Version() is `9.23.0-beta.1`.

Godoc:
- PoolSize: "base number of socket connections". Default `10 * runtime.GOMAXPROCS(0)`. Also claims new connections "will be allocated in excess of PoolSize", limited through MaxActiveConns.
- PoolTimeout: "amount of time client waits for connection if all connections are busy before returning an error". Default `ReadTimeout + 1 second`.
- MaxIdleConns: "maximum number of idle connections". "The idle connections are not closed by default." Default 0.
- MinIdleConns default 0. MaxActiveConns default 0 ("no limit").
- ConnMaxIdleTime default 30 minutes.

init():
- PoolSize == 0 → `10 * runtime.GOMAXPROCS(0)`.
- ReadTimeout 0 → `5 * time.Second` (not 3s). -1 disables (stored as 0).
- PoolTimeout == 0 → if ReadTimeout > 0 then ReadTimeout+1s else 30s. With default ReadTimeout 5s that is 6s.
- MaxConcurrentDials <= 0 → PoolSize; if > PoolSize, capped at PoolSize.

ClusterOptions (osscluster.go, not this file) defaults PoolSize to `5 * GOMAXPROCS` per node.
