## 1. Live pool

- [x] 1.1 Add default `poolSize` 8 and `poolTimeout` 200ms; unexported overrides when non-zero; buffered `chan struct{}` for in-use turns; acquire before pop-idle or dial; return the turn on release
- [x] 1.2 Change `borrow` so idle-miss waits for a slot (timer + `select`) instead of dialing past the live cap; pool wait elapsed → `redis:unreachable` and no extra dial
- [x] 1.3 Change `release` so a reusable socket is not closed only because idle is full while live sockets are under `poolSize`; still trim idle to eight; Close still drains idle and blocks redial
- [x] 1.4 Compiled fake tests: overlapping callers above eight stay at most `poolSize` dials; burst dials stay near `poolSize`; all slots busy → `redis:unreachable` and no extra accept; existing sequential-reuse and eight-concurrent tests still pass
- [x] 1.5 Run `go test ./simpleredis/...` until compiled and Yaegi tests pass

## 2. Live Redis and Dragonfly

- [x] 2.1 Add `simpleredis` live tests gated on `SIMPLEREDIS_LIVE_REDIS` / `SIMPLEREDIS_LIVE_DRAGONFLY` (skip when unset or `-short`): live sockets ≤ 8 and pool-timeout `redis:unreachable` on both engines
- [x] 2.2 Set those env vars on CI `test` job next to the existing Redis 6379 / Dragonfly 6380 services
- [x] 2.3 Probe `?hold=` EVAL TIME-wait (`numkeys` 0, Lua 5.1-safe, no `table.maxn`); keep existing verb headers and `/a` `/b`
- [x] 2.4 Pester on `/redis` and `/dragonfly`: concurrent holds, `/proc/net/tcp` ESTABLISHED on :6379 ≤ 8, extra waiter 502 `redis:unreachable` when eight are held; do not stop whoami-a/b
- [x] 2.5 Run `./Test-Integration.ps1` until Redis, Dragonfly, and reclaim Describes pass

## 3. Docs

- [x] 3.1 Update `knowledge/devdocs/std_go_simpleredis.md` gotcha: live cap eight, wait, `redis:unreachable` on pool timeout
- [x] 3.2 Confirm delta `std_go_simpleredis_tcp-session` matches the landed behaviour; `openspec validate simpleredis-live-pool-cap --type change --strict`
