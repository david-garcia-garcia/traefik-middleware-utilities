## 1. Memory Allow

- [x] 1.1 Add `tokenbucket/` with `NewMemory(rate, burst, maxDelay, ttl)`, `Allow(key)`, Lua-formula refill/burst/consume/wait/refund, lazy ttl expire, `SetNowForTest`
- [x] 1.2 Reject `rate <= 0`, `burst < 1`, `maxDelay < 0`, `ttl < 1s` at New
- [x] 1.3 Unit tests: burst after idle; refund when wait > maxDelay; invalid New; no HTTP/source reads

## 2. Redis Eval

- [x] 2.1 Add `NewRedis` on `*simpleredis.SimpleRedis`; copy Traefik Lua with MIT notice; `KEYS[1]`; `#rl_source == 4`; parse wait microseconds; propagate `redis:unreachable` / `redis:timeout`
- [x] 2.2 Fake TCP unit tests: Eval encoding; two instances share one key (no double burst)
- [x] 2.3 Memory vs Redis agreement on the same rate/burst/maxDelay sequence

## 3. Live Redis/Dragonfly and Yaegi

- [x] 3.1 Live tests table-driven on `TOKENBUCKET_LIVE_REDIS` / `TOKENBUCKET_LIVE_DRAGONFLY`; skip on `testing.Short` or missing addrs; burst, two-instance share, memory/Redis agreement on both engines
- [x] 3.2 Yaegi: GOPATH copy of non-test `tokenbucket` and `simpleredis` sources; stdlib only; `useunsafe` false; compiled test owns start/skip; probe calls Allow (fake + live)
- [x] 3.3 CI `test` job: set both `TOKENBUCKET_LIVE_*` to the existing Redis/Dragonfly services; no `-short`
- [x] 3.4 Run `go test ./tokenbucket/...` on this host until passing (short/unit always; live when engines are up)

## 4. Docs and specs

- [x] 4.1 README Libraries/Layout/Tests: add Token bucket row beside Window counter
- [x] 4.2 Usage packet `knowledge/devdocs/std_go_tokenbucket.md` and `index_std_go.md` row
- [x] 4.3 Confirm change specs `std_go_tokenbucket_allow` and `std_go_tokenbucket_lua-eval` match the API
- [x] 4.4 Run `openspec validate --change add-tokenbucket --strict`
