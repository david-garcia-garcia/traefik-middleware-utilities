## 1. Memory pour

- [ ] 1.1 Add `leakybucket/` with `NewMemory(leak, capacity, ttl)`, `Add(key, n)`, `Take(key)`, `Level(key)`, water clock, lazy ttl expire, `SetNowForTest`
- [ ] 1.2 Reject `leak <= 0`, `capacity <= 0`, `ttl < 1s` at New; `n < 1` on Add
- [ ] 1.3 Unit tests: pour to cap then deny; idle drain; Level after Δt; until-not-full; invalid New; no HTTP/source reads

## 2. Redis Eval and sync_rate

- [ ] 2.1 Add `NewRedis` on `*simpleredis.SimpleRedis`; EVAL leak-then-add HASH `water`/`last`; KEYS[1]; no `table.maxn`; no Go SET/HSET of the hash; propagate `redis:unreachable` / `redis:timeout`
- [ ] 2.2 `sync_rate=0` EVAL every Add/Take/Level; `>0` local_pours + timer EVAL; floor 20 ms; negative fails; `Sleep`/`Wake`/`Close` on Redis only
- [ ] 2.3 Fake TCP unit tests: Eval encoding; two instances both pours count (no last-write-wins); over-allow bounded by sync interval
- [ ] 2.4 Memory vs Redis agreement on the same leak/capacity sequence when `sync_rate=0`

## 3. Live Redis/Dragonfly and Yaegi

- [ ] 3.1 Live tests table-driven on `LEAKYBUCKET_LIVE_REDIS` / `LEAKYBUCKET_LIVE_DRAGONFLY`; skip on `testing.Short` or missing addrs; exact pour-to-cap then leak; buffered two-instance; memory/Redis agreement; both engines
- [ ] 3.2 Yaegi: GOPATH copy of non-test `leakybucket` and `simpleredis` sources; stdlib only; `useunsafe` false; compiled test owns start/skip; probe calls Take (fake + live)
- [ ] 3.3 CI `test` job: set both `LEAKYBUCKET_LIVE_*` to the existing Redis/Dragonfly services; no `-short`
- [ ] 3.4 Run `go test ./leakybucket/...` on this host until passing (short/unit always; live when engines are up)

## 4. Docs and specs

- [ ] 4.1 README Libraries/Layout/Tests: add Leaky bucket row beside Token bucket and Window counter
- [ ] 4.2 Usage packet `knowledge/devdocs/std_go_leakybucket.md` and `index_std_go.md` row
- [ ] 4.3 Confirm change specs `std_go_leakybucket_pour` and `std_go_leakybucket_sync-flush` match the API
- [ ] 4.4 Run `openspec validate --change add-leakybucket --strict`
