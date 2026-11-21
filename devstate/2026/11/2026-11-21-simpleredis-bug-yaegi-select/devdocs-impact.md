# Devdocs impact
change: simpleredis-drop-select-under-yaegi

Pin: `origin/master...cfd6b564` (after `git_sync`, master already merged; 31 reviewable paths).

## Units
- In-use-turn semaphore — subsystem — `simpleredis/pool.go` (`borrowSocket`, `freeInUseTurn`, `ensureInUseTurns`), `simpleredis/simpleredis.go` (`inUseMu`, `freeTurns`, `turnCap`, `turnWaiters`); spec `std_go_simpleredis_tcp-session`
- Retry backoff wait — subsystem — `simpleredis/commands_exec.go` `waitUntil`; spec `std_go_simpleredis_resp-commands`
- Select-free session source under Yaegi — pattern — `simpleredis/simpleredis_test.go` `TestSessionSourceHasNoSelect`; `knowledge/research/ext_traefik_plugins_yaegi-select/`
- Unit race skip list — pattern — `simpleredis/repro_yaegi_select_crosstalk_test.go` `skipYaegiUnderRace`, `simpleredis/yaegi_errorpath_test.go`; packet `std_go_test-suites`

## Findings
- [x] language-gap  In-use turn — `std_go_simpleredis` describes the semaphore in Gotchas but had no Language term for the permit it hands out
  Produced: `**In-use turn**` (PoolSize permit held from borrow to release, none for idle; semaphore = mutex, free count, frozen cap, FIFO of waiters). _Avoid_: token, slot, in-use-turn channel.
- [x] stale-usage  At-rest turn helpers — `std_go_simpleredis` Gotcha sent both at-rest checks to `repro_yaegi_select_probe_test.go`; the queued-waiter check is `queuedTurnWaiters` in `simpleredis/pool_test.go`
- [x] stale-usage  Key files — `std_go_simpleredis` had no line for the semaphore tests (`simpleredis/pool_test.go`) or the select repro pair (`simpleredis/repro_yaegi_select_*_test.go`)
- [x] stale-usage  Unit race — `std_go_test-suites`: Language said "in-use-turn length" (channel length is gone); lint note named `sr.borrow`, which does not exist (`sr.borrowSocket`); the `-race` "skip only" list omitted reclaim `TestYaegi_OpenHooksRunSleepWakeClose` and `TestYaegi_WatchPublishedDeliveredToFuncAny`

## Verified, no finding
- No `inUseTurns` channel left in either packet (only the `ensureInUseTurns` method name, which still exists).
- No-`select` gotcha matches HEAD: no `select` in non-test `simpleredis/*.go`; `TestSessionSourceHasNoSelect` in `simpleredis/simpleredis_test.go`; `time.AfterFunc(PoolTimeout)` + `context.AfterFunc(ctx)` abandon under `inUseMu`; `waitUntil` on a `context.WithTimeout` child.
- SimpleRedis `-race` skips at HEAD are exactly `TestYaegiErrorpath_PoolWait` and the four `TestReproSelect_*_Yaegi` (`skipYaegiUnderRace`); `TestAlloc*` skip via `skipAllocCeilingIfRace`; MSetEX fallback runs 4×8 under race.
- Master's TLS / ACL-user lines (merged in 317770ba) match HEAD: `Config.TLS` stored as `TLS.Clone()`, `tls.Dialer{NetDialer: &net.Dialer{Timeout: DialTimeout}}`, `AUTH <User> <Pass>` when `User` is set (also with empty `Pass`), `ErrTLS` = `redis:tls` not wrapping `ErrUnreachable`, `tls_error` on `dial:`/`do:` Error lines, `isTLSRejection` (`CertificateVerificationError`, `RecordHeaderError`, `*net.OpError` `remote error`), `startTLSFakeRedis`, `TestWriteTLSFixtureFiles` / `SIMPLEREDIS_TLS_FIXTURE_OUT`, `SIMPLEREDIS_LIVE_{REDIS,DRAGONFLY}_TLS` / `SIMPLEREDIS_LIVE_TLS_DIR`, user `app` / `apppass`, CI ports 6383/6384, containers `simpleredis-{redis,dragonfly}-tls`, `docker cp` to `/tls`, `ACL SETUSER app`.
