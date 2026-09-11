# Performance

1. [hard] Unbounded collection or cache — `tokenbucket/memory.go:21` — `buckets map[string]*memEntry` keyed by live `Allow(key)`; `expireAt` only resets that key’s tokens on a later Allow and never `delete`s; no max size. Unique keys grow with distinct identities on the request path (usage keys as `"ip:" + ip`). Redis `EXPIRE` bounds the other store; this map does not.
   → Cap the map and evict idle keys (`delete` expired entries; Traefik’s in-memory limiter bounds sources at 65536)
   Status: done
   Argument: delete expired keys; cap at 65536 with dropOne.
