## 1. In-process fake and compiled handshake tests

- [x] 1.1 Add optional AUTH/SELECT reply fields on `fakeRedis` defaulting to `+OK`; increment a hangup counter when `serve` exits on read error
- [x] 1.2 Add handshake AUTH-failure tests: `Init(addr, "wrong-password", "")` for each AUTH-class prefix; assert `redis:noauth`, empty idle, hangup, one accept
- [x] 1.3 Add handshake SELECT-failure test: `Init(addr, "secret", "99")`, AUTH `+OK` then SELECT `-ERR DB index is out of range`; assert AUTH before SELECT, that error text, empty idle, hangup, one accept
- [x] 1.4 Keep `TestRejectedAuthIsReturned` and `TestAuthAndSelectOncePerDial`; run `go test ./simpleredis/` until new tests pass and cover profile counts on `dial` AUTH/SELECT error blocks are non-zero

## 2. Live Go tests and CI engines

- [x] 2.1 Add skip-if-unset live tests for SELECT 99 against `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY` (empty password, database `99`); assert `ERR DB index is out of range` and empty idle
- [x] 2.2 Add skip-if-unset live tests for wrong password against `SIMPLEREDIS_LIVE_REDIS_AUTH` / `SIMPLEREDIS_LIVE_DRAGONFLY_AUTH`; assert `redis:noauth` and empty idle
- [x] 2.3 Wire CI `test` job: set `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY` to existing 6379/6380; add passworded dest-image siblings on 6381/6382 (`services` if process args work, else `docker run` with `--requirepass` and Dragonfly memlock); set AUTH env vars; do not requirepass 6379/6380

## 3. Traefik probe, compose, and Pester

- [x] 3.1 Add probe `Config.Password` and `Config.Database` (empty default); `New` passes them to `Init`; success labels stay host-only; keep the existing Lua 5.1-safe KEYS Eval snippet
- [x] 3.2 Add compose `redis-auth` / `dragonfly-auth` (`redis-server --requirepass secret`, Dragonfly `--requirepass=secret`, memlock `-1`) and whoami routes for wrong password and `database=99`; do not requirepass existing `redis` / `dragonfly`
- [x] 3.3 Pester: assert 502 `redis:noauth` on wrong-password routes and 502 `ERR DB index is out of range` on database-99 routes for Redis and Dragonfly; do not stop `whoami-a` or `whoami-b`; keep `/redis` `/dragonfly` success Its

## 4. Specs

- [x] 4.1 Confirm the change deltas `std_go_simpleredis_tcp-session` and `std_go_simpleredis_resp-commands` match the landed tests and harness
- [x] 4.2 Run `openspec validate --change simpleredis-test-03-auth-select --strict`
