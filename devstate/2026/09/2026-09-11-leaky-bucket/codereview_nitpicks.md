# Nitpicks

1. [hard] Name for the scope — `leakybucket/lua.go:12` — `local rl_source = redis.call('hgetall', key)` then `#rl_source == 4` is Traefik RateLimit's HGETALL noun; this script only reads `last`/`water`
   → `local hash = redis.call('hgetall', key)` (and `#hash == 4`)
   Status: done
   Argument: `lua.go` HGETALL is `hash`; `c8855c3`.
2. [hard] Name for the scope — `leakybucket/memory.go:93` — `for source, entry := range m.buckets` names the opaque key `source`; `pourLocked`/`Add` in this file use `key`
   → `for key, entry := range m.buckets`
   Status: done
   Argument: `dropExpired` loop uses `key`; `c8855c3`.
3. [hard] Symmetry and consistency — `leakybucket/redis.go:239` — `for redisKey, state := range r.keys` while `Add`/`evalPour`/`addBuffered`/`keyLocked` in this file use `key` for that role
   → `for key, state := range r.keys`
   Status: done
   Argument: `flushPending` loop uses `key`; `c8855c3`.
