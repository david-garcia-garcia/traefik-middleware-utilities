# Performance

1. [hard] Unbounded collection or cache — `ratelimit/limiter.go:31,166,180` — `windows map[string]*windowState` inserts on every new Redis window key in buffered mode and never deletes; grows with unique opaque rate-limit keys (and stale window suffixes) for the process lifetime
   → Cap the map or evict entries after window TTL / flush when `localDelta == 0` and `expireAt` has passed
   Status: done
   Argument: flushPending deletes localDelta==0 when expireAt is 0 or past (`1979b40`).

2. [hard] I/O in a loop — `ratelimit/limiter.go:274-283` — `flushPending` walks `l.windows` and issues one sequential `redis.Eval` per entry with `localDelta > 0` each tick; dirty key count grows with unique keys that `Take` between flushes
   → Pipeline or batch flushes; prune stale map entries so each tick does not scan unbounded history
   Status: skipped
   Argument: SimpleRedis has no pipeline API; sequential Eval is the client contract. Prune (finding 1) bounds the scan.
