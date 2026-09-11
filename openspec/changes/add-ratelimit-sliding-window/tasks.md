## 1. Package and sliding Take

- [ ] 1.1 Add `ratelimit/` with `New(redis *simpleredis.SimpleRedis, syncRate time.Duration)`, `Take`/`Allow`, opaque key + limit + whole-second window, sliding estimate, increment-then-compare
- [ ] 1.2 Exact mode: `Incr` current, `Expire` two windows when result is 1, `Get` previous (`redis:miss` → 0); propagate `redis:unreachable` / `redis:timeout`
- [ ] 1.3 `SetNowForTest` for window-boundary tests; production uses `time.Now`
- [ ] 1.4 Unit tests with in-package fake TCP: N Takes then deny; sliding dump-at-boundary does not double; Redis error path

## 2. Buffered sync and reclaim hooks

- [ ] 2.1 Buffered mode: per-key `redis_known` + `local_delta`; EVAL INCRBY + EXPIREAT-if-new with `KEYS[1]`; 20 ms floor; negative sync_rate fails New
- [ ] 2.2 `Sleep` (flush then stop ticker), `Wake` (start ticker when sync_rate > 0), `Close` after Sleep without closing SimpleRedis; no `time.Tick`
- [ ] 2.3 Unit tests: two fake-backed clients share without last-write-wins; Close stops the flush goroutine

## 3. Live Redis/Dragonfly and Yaegi

- [ ] 3.1 Live tests table-driven on `RATELIMIT_LIVE_REDIS` / `RATELIMIT_LIVE_DRAGONFLY`; skip on `testing.Short` or missing addrs; exact, two-client share, sliding boundary on both engines
- [ ] 3.2 Yaegi live: GOPATH copy of non-test `ratelimit` and `simpleredis` sources; stdlib only; `useunsafe` false; compiled test owns start/skip; probe calls Take
- [ ] 3.3 CI `test` job: Redis 7 alpine + Dragonfly `v1.40.2` service containers, both env vars set, no `-short`
- [ ] 3.4 Run `go test ./ratelimit/...` on this host until passing (short/unit always; live when engines are up)

## 4. Docs and specs

- [ ] 4.1 README Libraries/Layout/Tests: this primitive instead of leaky `bucket/`
- [ ] 4.2 Usage packet `knowledge/devdocs/std_go_ratelimit.md` and `index_std_go.md` row
- [ ] 4.3 Confirm change specs `std_go_ratelimit_sliding-take` and `std_go_ratelimit_sync-flush` match the API
- [ ] 4.4 Run `openspec validate --change add-ratelimit-sliding-window --strict`
