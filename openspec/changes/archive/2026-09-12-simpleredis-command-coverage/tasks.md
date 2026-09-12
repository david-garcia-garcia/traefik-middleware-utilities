## 1. Compiled live e2e

- [x] 1.1 Add a KEYS/ARGV Eval case to `TestLive_Eval` using the Kong incrby+expireat script (Lua 5.1-safe, key in KEYS)
- [x] 1.2 Add future MSetEXAt then Get + positive TTL next to `pastExatMiss` in `TestLive_MSetEX`

## 2. Yaegi live

- [x] 2.1 Expand `LiveVerbs` to MGet, IncrBy, Expire, ExpireAt, and MSetEXAt; leave fake-TCP Yaegi at the existing subset

## 3. Traefik HTTP verb map

- [x] 3.1 Dispatch `ServeHTTP` on the last path segment: health on one-segment paths; one public verb per remainder; `drop=1` selects DropHost; 404 on unknown remainder
- [x] 3.2 Pester composes each proof from status and body (Set then Get, Eval script in the request, drop=1 Incr/Eval, hold via Eval TIME wait)
- [x] 3.3 Keep compose health on exact `/redis` and `/dragonfly`; handshake routers unchanged
- [x] 3.4 Split Pester by domain (`integration-tests.reclaim.Tests.ps1`, `integration-tests.simpleredis.Tests.ps1`) and dot-source `integration-tests.utils/`
- [x] 3.5 CI splits Integration Tests (reclaim), Integration Tests Redis, and Integration Tests Dragonfly; SimpleRedis file switches `INTEGRATION_ENGINE`

## 4. Usage

- [x] 4.1 Update `knowledge/devdocs/std_go_simpleredis.md` and `std_go_test-suites.md` so Yaegi live and Traefik Pester match the HTTP verb map
