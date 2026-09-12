# Standards

1. [hard] Leave a trail — `scripts/integration-tests.Tests.ps1:23` — new function `Stop-TraefikEngineClientForTest` has no job comment, and the inspect-IP / CLIENT LIST / KILL blocks have no intros
   ```
   function Stop-TraefikEngineClientForTest {
       param(
           [Parameter(Mandatory)]
           [ValidateSet("redis", "dragonfly")]
           [string]$Engine
       )
       $traefikIp = (docker inspect ...
       $list = docker @cli CLIENT LIST
       ...
       docker @cli CLIENT KILL ADDR $addr
   ```
   → Comment that it CLIENT KILLs Traefik ADDR on the engine (not TYPE/SKIPME); intro each block
   Status: done
   Argument: Job comment plus intros on inspect-IP, CLIENT LIST, and CLIENT KILL ADDR blocks (not TYPE/SKIPME).
2. [judgement] Duplicated Code — `scripts/integration-tests.Tests.ps1:150` — Redis and Dragonfly recover Its are the same warmup / kill / `?recover=1` / header shape with only engine and URL swapped
   ```
   It "GET /redis recovers after CLIENT KILL of the Traefik client" {
       $warmup = Invoke-WebRequest -Uri "$script:BaseUrl/redis" ...
       Stop-TraefikEngineClientForTest -Engine redis
       $response = Invoke-WebRequest -Uri "$script:BaseUrl/redis?recover=1" ...
   }
   It "GET /dragonfly recovers after CLIENT KILL of the Traefik client" {
       ...
       Stop-TraefikEngineClientForTest -Engine dragonfly
       ...
   }
   ```
   → Extract one helper that takes engine and path; call it from both Its
   Status: skipped
   Argument: judgement; two Its keep Redis and Dragonfly visible as dest backends.
