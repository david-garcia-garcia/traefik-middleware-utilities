## 1. Compiled live e2e

- [x] 1.1 Add a KEYS/ARGV Eval case to `TestLive_Eval` using the Kong incrby+expireat script (Lua 5.1-safe, key in KEYS)
- [x] 1.2 Add future MSetEXAt then Get + positive TTL next to `pastExatMiss` in `TestLive_MSetEX`

## 2. Yaegi live

- [x] 2.1 Expand `LiveVerbs` to MGet, IncrBy, Expire, ExpireAt, and MSetEXAt; leave fake-TCP Yaegi at the existing subset

## 3. Traefik path cases

- [x] 3.1 Dispatch `ServeHTTP` on the last path segment: health on one-segment paths; per-verb cases; `/recover`, `/drop`, `/hold`; 404 on unknown remainder
- [x] 3.2 Replace `Assert-SimpleRedisVerbHeaders` with one Pester `It` per case per engine; move EVALSHA, recover, hold, and distinct-token tests onto the new paths
- [x] 3.3 Keep compose health on exact `/redis` and `/dragonfly`; handshake routers unchanged

## 4. Usage

- [x] 4.1 Update `knowledge/devdocs/std_go_simpleredis.md` and `std_go_test-suites.md` so Yaegi live and Traefik Pester match the path-case suite
