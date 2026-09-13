## 1. Tests first (must fail on dest)

- [ ] 1.1 Add `tokenbucket/repro_epoch_idle_burst_test.go` `TestRepro_NewKeyFillsToBurstAtEpoch` with dest `Allow(ctx, key)`: `epoch_clock` burst 5; `huge_burst_elapsed_below_burst` burst `1e12`
- [ ] 1.2 Add Redis/fake cases for those same clocks plus an `allowScript` assertion that empty `#rl_source ~= 4` seeds `tokens = burst` and `last = t`
- [ ] 1.3 Run `go test -short -count=1 -timeout 60s -run TestRepro_NewKeyFillsToBurstAtEpoch ./tokenbucket` and confirm FAIL (do not fix yet)

## 2. Fill missing state

- [ ] 2.1 Lua: empty `HGETALL` (`#rl_source ~= 4`) sets `tokens = burst` and `last = t`; keep MIT notice
- [ ] 2.2 Memory: new `memEntry` (miss and TTL delete) starts `tokens = burst`, `last = nowMicro`; do not change `consumeOne` for `last=0`
- [ ] 2.3 Fake Redis missing hash seeds `tokens=burst`, `last=now` like Lua, then `consumeOne`

## 3. Pass and agree

- [ ] 3.1 Re-run `go test -short -count=1 -timeout 60s -run TestRepro_NewKeyFillsToBurstAtEpoch ./tokenbucket` and confirm PASS
- [ ] 3.2 Run `go test -short -count=1 -timeout 60s ./tokenbucket` and confirm `TestMemoryAndRedis_Agree` and existing burst/TTL tests pass
- [ ] 3.3 Run `openspec validate --change fill-new-key-to-burst --strict`
