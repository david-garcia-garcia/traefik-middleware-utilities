## 1. Compiled fake peer-close

- [x] 1.1 Add a peer-close listener helper (do not change `startFakeRedis`): accept, reply to the first command, close the accepted socket, optionally accept a second connection
- [x] 1.2 Add a success test: first Get ok, second Get ok on a new dial, two accepts, dead conn not in `idle`; name it for peer-close / `io.EOF`
- [x] 1.3 Add a fail test: close the listener after the first accept so retry `borrow` returns `redis:unreachable`
- [x] 1.4 Rename `TestStaleConnectionIsRetried` to name the client-fd / `SetDeadline` / `os.ErrClosed` arm; keep client `conn.close()`, do not peer-close there
- [x] 1.5 Run `go test ./simpleredis/` until the renamed test and both peer-close tests pass without changing `exec` unless they fail on dest

## 2. Live engines

- [x] 2.1 Add same-package `test`-named helper that dials a sidecar and sends RESP `CLIENT KILL ADDR` of `sr.idle[0].netConn.LocalAddr()`; fail if the kill count is 0; do not ship a production `ClientKill`
- [x] 2.2 Add `simpleredis/live_test.go` (skip `-short` / both env unset) for Redis and Dragonfly: Get, kill that pooled ADDR, Get succeeds, killed socket not reused; env `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY`
- [x] 2.3 Set those env vars on the CI `test` job next to the windowcounter live addrs (Redis `:6379`, Dragonfly `:6380`)
- [x] 2.4 Run `go test ./simpleredis/` (and with live env when engines are up) until the live tests pass or skip as specified

## 3. Traefik probe and Pester

- [x] 3.1 Keep `Init` in `New`. On `recover=1` run Set+Get only, set `X-SimpleRedis-Recover: ok`, do not Eval. Default path keeps every verb header (existing EVAL stays Lua 5.1-safe with keys in `KEYS`)
- [x] 3.2 Add Pester Its in the SimpleRedis Describe: warmup GET `/redis` and `/dragonfly`, `CLIENT KILL` by Traefik `ADDR`/`ID` via `docker compose exec redis redis-cli` (`-h dragonfly` for Dragonfly; no `TYPE`/`SKIPME`), then GET `?recover=1` is 200 with `X-SimpleRedis-Recover: ok`. Do not stop `whoami-a`/`whoami-b`. Compose `timeout` stays 0
- [x] 3.3 Run `./Test-Integration.ps1` until SimpleRedis Redis and Dragonfly recovery Its pass and reclaim stays green

## 4. Specs

- [x] 4.1 Confirm the change delta `std_go_simpleredis_tcp-session` matches the landed tests and probe
- [x] 4.2 Run `openspec validate simpleredis-peer-close-eof-redial --type change --strict`
