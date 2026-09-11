## 1. Sweep-on-release

- [x] 1.1 In `release`, on every reusable path, peel `idle[0]` while older than `idleTimeout` (stop at the first still-valid head); collect peeled conns under the lock; `close()` after unlock; peel before the append-or-close decision including `closed` and `len(idle) >= maxIdleConns`
- [x] 1.2 Leave `borrow` LIFO `break` and `Close` drain unchanged; no ticker, no `container/list`; keep `idleTimeout` 30s and `maxIdleConns` 8

## 2. Fake-server unit

- [x] 2.1 Add a two-entry idle-head test: two concurrent Gets then wait for `len(idle)==2`; backdate head `lastUsed`; Get; assert the head socket is closed, not in `idle`, and the command succeeds
- [x] 2.2 Keep `TestIdleTimeoutOpensANewConnection`; run `go test ./simpleredis/ -short` until the new unit passes

## 3. Live Redis and Dragonfly

- [x] 3.1 Add `simpleredis/live_test.go` table-driven on `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY`; skip on `testing.Short` or missing addrs; same two-entry recipe with `Get`; optional `waitLiveClient` via Get/Set/Incr (no Ping, no production Reset/Dump)
- [x] 3.2 Set both env vars on CI `test` job (reuse `127.0.0.1:6379` / `127.0.0.1:6380`); do not add `-short`; name the vars in README Tests beside the sibling vars
- [x] 3.3 Do not change `docker-compose.yml`, Pester `/redis` `/dragonfly`, or `e2e/simpleredisprobe`; do not set server `timeout`

## 4. Usage packet and validate

- [x] 4.1 After the peel lands, add a usage gotcha on `knowledge/devdocs/std_go_simpleredis.md` that `release` peels stale heads
- [x] 4.2 Run `openspec validate --change simpleredis-idle-head-sweep --strict`
