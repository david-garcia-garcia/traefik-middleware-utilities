## 1. Package and sliding Take

- [x] 1.1 Add `windowcounter/` with `New(redis *simpleredis.SimpleRedis, syncRate time.Duration)`, `Take`/`Allow`, opaque key + limit + whole-second window, sliding estimate, increment-then-compare
- [x] 1.2 Exact mode: `Incr` current, `Expire` two windows when result is 1, `Get` previous (`redis:miss` → 0); propagate `redis:unreachable` / `redis:timeout`
- [x] 1.3 `SetNowForTest` for window-boundary tests; production uses `time.Now`
- [x] 1.4 Unit tests with in-package fake TCP: N Takes then deny; sliding dump-at-boundary does not double; Redis error path

## 2. Buffered sync and reclaim hooks

- [x] 2.1 Buffered mode: per-key `redis_known` + `local_delta`; EVAL INCRBY + EXPIREAT-if-new with `KEYS[1]`; 20 ms floor; negative sync_rate fails New
- [x] 2.2 `Sleep` (flush then stop ticker), `Wake` (start ticker when sync_rate > 0), `Close` after Sleep without closing SimpleRedis; no `time.Tick`
- [x] 2.3 Unit tests: two fake-backed clients share without last-write-wins; Close stops the flush goroutine

## 3. Live Redis/Dragonfly and Yaegi

- [x] 3.1 Live tests table-driven on `WINDOWCOUNTER_LIVE_REDIS` / `WINDOWCOUNTER_LIVE_DRAGONFLY`; skip on `testing.Short` or missing addrs; exact, two-client share, sliding boundary on both engines
- [x] 3.2 Yaegi live: GOPATH copy of non-test `windowcounter` and `simpleredis` sources; stdlib only; `useunsafe` false; compiled test owns start/skip; probe calls Take
- [x] 3.3 CI `test` job: Redis 7 alpine + Dragonfly `v1.40.2` service containers, both env vars set, no `-short`
- [x] 3.4 Run `go test ./windowcounter/...` on this host until passing (short/unit always; live when engines are up)

## 4. Docs and specs

- [x] 4.1 README Libraries/Layout/Tests: this primitive instead of leaky `bucket/`
- [x] 4.2 Usage packet `knowledge/devdocs/std_go_windowcounter.md` and `index_std_go.md` row
- [x] 4.3 Confirm change specs `std_go_windowcounter_sliding-take` and `std_go_windowcounter_sync-flush` match the API
- [x] 4.4 Run `openspec validate --change add-ratelimit-sliding-window --strict`
