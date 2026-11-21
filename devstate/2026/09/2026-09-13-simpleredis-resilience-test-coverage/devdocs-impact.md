# Devdocs impact
change: simpleredis-resilience-test-coverage

## Units
- SimpleRedis — subsystem — `simpleredis/` (`std_go_simpleredis.md`)
- RESP decode — subsystem — `simpleredis/resp.go` (`std_go_simpleredis_resp-decode.md`)
- Test suites — pattern — CI unit/`-short`/`-race` (`std_go_test-suites.md`)

## Findings
- [x] stale-usage  SimpleRedis — `std_go_simpleredis` Key files / prove-with omit the healthy-probe tests and BUGS.md; Gotchas omit the live-socket sampler pitfall
- [x] stale-usage  RESP decode — `std_go_simpleredis_resp-decode` Key files omit `resp_fuzz_test.go`; Gotchas omit that a `readReply` panic leaks a pool turn
- [x] stale-usage  Test suites — `std_go_test-suites` How-to does not say SimpleRedis chaos/lifecycle skip under `-short`, or that Unit race skips Yaegi error paths and concurrent MSetEX
