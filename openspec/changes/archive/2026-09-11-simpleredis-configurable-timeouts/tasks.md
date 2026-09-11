## 1. Options and session fields

- [x] 1.1 Add exported `Options` (`DialTimeout`, `IoTimeout`, `IdleTimeout` `time.Duration`; `MaxIdleConns` int) and `InitWithOptions(host, pass, database, Options)` in `simpleredis/simpleredis.go`; resolve `<= 0` durations and `MaxIdleConns <= 0` into unexported session fields from the package constants; `Init` delegates with `Options{}`; do not export host/pass/database; do not add `poolSize`/`poolTimeout` or functional options
- [x] 1.2 Point `dial`, `do` (`SetDeadline(now+ioTimeout)` only), `borrow`, and `release` at those session fields; do not add a live-socket count or wait channel

## 2. Fake-server tests

- [x] 2.1 Keep `TestIoTimeout` on `Init` defaults; add a never-reply fake with configured `IoTimeout` tens of milliseconds that returns `redis:timeout` well under one second; `InitWithOptions` to a refusing host does not dial
- [x] 2.2 Add configured `IdleTimeout` (idle older than that duration dials again) and `MaxIdleConns=1` (idle list length at most one; extra socket closed)
- [x] 2.3 After `InitWithOptions`, assert the stored dial duration is the configured value; hanging-SYN to `192.0.2.1` returns `redis:unreachable` well under two seconds
- [x] 2.4 Run `go test ./simpleredis/ -short` until the new units pass (existing `TestIoTimeout` still green)

## 3. Live Redis and Dragonfly

- [x] 3.1 Add `simpleredis/live_test.go` table-driven on `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY`; skip on `testing.Short` or missing addrs; unique empty key; same-package `exec("BLPOP", key, "10")` with `IoTimeout` ~50ms asserts `redis:timeout` well before 10s; do not export BLPOP; do not Eval or CLIENT PAUSE
- [x] 3.2 Set both env vars on CI `test` job (`127.0.0.1:6379` / `127.0.0.1:6380`); do not add `-short`; name the vars in README Tests beside the limiter vars
- [x] 3.3 Do not change `docker-compose.yml`, Pester `/redis` `/dragonfly`, or `e2e/simpleredisprobe` Config; probe `New` keeps `Init`

## 4. Usage packet and validate

- [x] 4.1 Update `knowledge/devdocs/std_go_simpleredis.md` How to use / Pattern snippet: `InitWithOptions`; zero or negative = default; `Init` unchanged; idle cap is the idle list (live-socket wait queue is not this type). Language stays
- [x] 4.2 Run `openspec validate simpleredis-configurable-timeouts --strict --type change`
