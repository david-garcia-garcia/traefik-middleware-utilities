# Standards

1. [hard] Leave a trail — `e2e/simpleredisprobe/plugin.go:62` — `New` comment says it constructs SimpleRedis and returns a handler (and does not dial) but omits that responses are terminal; it still errors with `missing next handler` while `ServeHTTP` never calls `next`
   → Document terminal probe behavior on `New` and align the nil-`next` check or message with that contract
   Status: done
   Argument: New comment and nil-next error say Traefik still requires next; ServeHTTP never forwards.

2. [hard] Leave a trail — `Test-Integration.ps1:89` — `Test-EngineStackHealth` has no succinct job comment (waits Traefik-side engine, drop, auth, and whoami health for one `-Engine`)
   → Add a one-line comment stating what readiness it proves
   Status: done
   Argument: one-line comment on Test-EngineStackHealth.

3. [hard] Leave a trail — `Test-Integration.ps1:100` — `Invoke-IntegrationPester` has no succinct job comment (runs Pester with shared exit-on-fail config)
   → Add a one-line comment stating what it runs and how failures surface
   Status: done
   Argument: one-line comment on Invoke-IntegrationPester.

4. [judgement] Duplicated Code — `.github/workflows/ci.yml:43` — `integration-redis` and `integration-dragonfly` are near-copy jobs (checkout, PowerShell/Pester install, run, failure logs) differing mainly by engine flag and log service names
   → Extract a reusable workflow or composite so engine-specific steps stay one list
   Status: skipped
   Argument: judgement; Go E2E already lists engine jobs the same way; not applied unattended.
