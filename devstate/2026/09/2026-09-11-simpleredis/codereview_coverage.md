# Test coverage

1. [hard] Critical path untested — `simpleredis/simpleredis.go:81-85` Init and `e2e/simpleredisprobe/plugin.go:46-48` New must not dial (Traefik still starts if Redis is late); `(none)` asserts zero sockets after Init. `TestConnectionIsReused` still sees 1 conn if Init dialed; Pester `/redis` runs after Redis is up, so New that SET+GETs stays green
   → Assert `connections()==0` after Init, or that New succeeds against a refusing host
   Status: done
   Argument: TestConnectionIsReused asserts 0 conns after Init; TestNewDoesNotDial Inits against 127.0.0.1:1.
2. [hard] Assertion does not prove the job — `simpleredis/simpleredis.go:120-122` Set sends `EX` and duration; `TestValueWithNewlinesSurvives`, Yaegi `RoundTrip`, and Pester `X-SimpleRedis-Value` only assert the value returns. Fake Redis `SET` stores `args[2]` and ignores EX — reverting EX leaves those tests green
   → Assert the SET command includes EX and the duration (or that the key expires)
   Status: done
   Argument: TestSetSendsExpire asserts SET argv includes EX 60.
3. [hard] Edge case untested — `simpleredis/simpleredis.go:344-351` maps WRONGPASS, NOPERM, and `ERR Client sent AUTH` to `redis:noauth`; `TestRejectedAuthIsReturned` only sends `-NOAUTH`
   → Assert each AUTH-class prefix returns `redis:noauth`
   Status: done
   Argument: TestRejectedAuthIsReturned covers NOAUTH, WRONGPASS, NOPERM, ERR Client sent AUTH.
4. [hard] Edge case untested — `simpleredis/simpleredis.go:276-277` treats `:` integer replies as success; `TestDelSucceeds` only replies `+OK` (fake Redis DEL also uses the default `+OK`)
   → Assert Del against `:1` returns no error
   Status: done
   Argument: TestDelIntegerReplySucceeds.
5. [judgement] Happy path only — `e2e/simpleredisprobe/plugin.go:35-37` nil next and `55-62` Set/Get 502; Pester only asserts `/redis` 200 and `X-SimpleRedis-Value=ok`. `(none)` unit test
   → Assert nil next errors and Redis errors return 502, or skip if the probe is e2e-only
   Status: skipped
   Argument: probe is e2e-only; Pester owns the happy path. Judgement.
