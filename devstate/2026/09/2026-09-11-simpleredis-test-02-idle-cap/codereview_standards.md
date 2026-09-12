# Standards

1. [hard] Leave a trail — `simpleredis/simpleredis_test.go:296` — `serve` comment says it reads one RESP command; the body loops until `readCommand` fails
   → Comment the connection lifetime (hold each command's reply on this socket, then write a GET hit), matching `fakeRedis.serve`
   Status: done
   Argument: holdFakeRedis.serve comment names the socket loop (436cdd0).
2. [judgement] Duplicated Code — `simpleredis/simpleredis_test.go:410` and `simpleredis/simpleredis_test.go:438` — both tests launch 16 held Gets then assert `idleLen <= maxIdleConns`
   → Extract one overlap helper if a later change must keep both assertions in lockstep; tasks 1.2 and 1.3 asked for both tests
   Status: skipped
   Argument: judgement; tasks 1.2 and 1.3 asked for both tests.
3. [judgement] Duplicated Code — `scripts/integration-tests.Tests.ps1:160` and `scripts/integration-tests.Tests.ps1:175` — Redis and Dragonfly Its copy the same 2s `Get-BackendIdleClients` retry
   → One helper that waits until remaining ≤ 8; leave while the two Its stay the live SHALL pair
   Status: skipped
   Argument: judgement; two Its are the live SHALL pair.
