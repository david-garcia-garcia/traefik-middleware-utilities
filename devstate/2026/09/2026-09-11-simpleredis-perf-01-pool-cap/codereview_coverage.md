# Test coverage

**Ticket job (source: `devstate/.../requirement.md`, proposal Why):** Cap live TCP sockets with `poolSize` wait + `poolTimeout` (`redis:unreachable`, no extra dial), prove on fake and live Redis/Dragonfly; `New(Config)` freezes pool knobs (deviation).

1. [hard] Critical path untested — `simpleredis/simpleredis.go:22-23` (`errPoolWait` distinct from `errUnreachable`) and `simpleredis/commands.go:140-146` (`exec` retries when `shouldRetry(borrow err)`) — pool-wait timeout must not multiply `MaxRetries`; `(none)` asserts attempt count or that `errPoolWait` is not retried (`TestPoolWaitTimesOutWithoutExtraDial` only checks `Error()` text and `fake.connections() <= 2`, which stays green if pool-wait failures were `errUnreachable` and retried without dialing)
   → Hold all turns, issue one waiter with default `MaxRetries`, assert a single pool-wait window (e.g. elapsed `< 2*PoolTimeout` and no extra TCP dials), or unit-test `shouldRetry(errPoolWait) == false`
   Status: done
   Argument: TestShouldRetryPoolWaitIsFalse; TestPoolWaitTimesOutWithoutExtraDial now fails if wait >= 2*PoolTimeout.

2. [judgement] Happy path only — `scripts/integration-tests.Tests.ps1:152-153` — live cap check uses `Should -BeLessOrEqual 10` while default `poolSize` is 8; an off-by-one cap regression could stay green
   → Tighten to `Should -BeLessOrEqual 8` (or `poolSize` from probe config) once tcp sidecar noise is understood
   Status: skipped
   Argument: same as Spec 1; /proc ESTABLISHED slack 10 is compose/sidecar noise, not plugin-only.
