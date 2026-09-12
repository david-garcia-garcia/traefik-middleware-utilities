# Performance

1. [hard] Unbounded collection or cache — `leakybucket/memory.go:21` — `buckets map[string]*memEntry` keyed by live `Add`/`Take` identity; `expireAt` delete plus `dropExpired` drop idle keys only; no max size (sibling `tokenbucket.Memory` caps at `maxMemorySources` 65536). Unique keys grow with distinct identities on the request path (usage keys as `"ip:" + ip`).
   → Cap the map and evict when at cap (reuse `tokenbucket`’s 65536 + `dropOne`)
   Status: done
   Argument: `maxMemorySources` 65536 + `dropOne` like tokenbucket; `c8855c3`.
2. [hard] I/O in a loop — `leakybucket/redis.go:239` — `flushPending` walks `r.keys` and issues one sequential `redis.Eval` per entry with `localPours > 0` each tick (under `r.mu`); dirty key count grows with unique keys that `Add` between flushes
   → Pipeline or batch flushes; cap the buffer so each tick does not Eval unbounded unique keys
   Status: skipped
   Argument: sequential EVAL is the windowcounter Kong flush; `simpleredis` has no pipeline in v1.
3. [hard] Hot-path full scan — `leakybucket/memory.go:80` — `pourLocked` calls `dropExpired`, which walks every `buckets` entry, on each new or expired key under the request mutex; unique-key arrivals grow both the map and the per-request walk
   → Bound the map (same cap as finding 1) so the walk cannot grow with unique identities
   Status: done
   Argument: same 65536 cap as finding 1; `c8855c3`.
