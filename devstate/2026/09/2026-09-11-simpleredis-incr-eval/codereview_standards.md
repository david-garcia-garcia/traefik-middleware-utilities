# Standards

1. [hard] Symmetry and consistency — `simpleredis/yaegi_test.go:132` — the Kong incrby+expireat Lua snippet is `kongScript` here but `kongIncrbyExpireatScript` in `e2e/simpleredisprobe/plugin.go:17` and `simpleredis/simpleredis_test.go:191`
   ```
   const kongScript = `local exists = redis.call("exists", KEYS[1])
   local value = redis.call("incrby", KEYS[1], ARGV[1])
   ...
   return value`
   ```
   → Rename to `kongIncrbyExpireatScript` in the yaegi clientprobe source so the same role uses the same identifier across sibling probe files
   Status: done
   Argument: renamed yaegi clientprobe const to kongIncrbyExpireatScript (27d70e5).

2. [judgement] Duplicated Code — `e2e/simpleredisprobe/plugin.go:17` — the Kong incrby+expireat Lua script is copied verbatim in three hunks (`plugin.go`, `simpleredis/simpleredis_test.go:191`, `simpleredis/yaegi_test.go:132`)
   ```
   const kongIncrbyExpireatScript = `local exists = redis.call("exists", KEYS[1])
   local value = redis.call("incrby", KEYS[1], ARGV[1])
   if exists == 0 then
     redis.call("expireat", KEYS[1], ARGV[2])
   end
   return value`
   ```
   → Extract one shared owner for the script (e.g. a small fixture both e2e and tests import) instead of three near-identical literals
   Status: skipped
   Argument: judgement; probe cannot import _test; a shared production fixture would export limiter Lua from SimpleRedis.

3. [judgement] Duplicated Code — `scripts/integration-tests.Tests.ps1:93` — the eight `X-SimpleRedis-*` header assertions are duplicated between the `/redis` and `/dragonfly` Its
   ```
   $response.Headers["X-SimpleRedis-Value"] | Should -Be "ok"
   $response.Headers["X-SimpleRedis-MGet"] | Should -Be "ok"
   ...
   $response.Headers["X-SimpleRedis-Eval"] | Should -Be "3"
   ```
   → Extract a helper that asserts all verb headers on a response; call it from both backend cases with only the URL differing
   Status: skipped
   Argument: judgement; two Its keep failure output on the route name.
