## 1. Failing repro first

- [ ] 1.1 Port `failNextExpireCommands` / `expireCommandCount` onto dest `windowcounter/fake_redis_test.go`
- [ ] 1.2 Create `windowcounter/repro_expire_not_retried_test.go`: fail the first EXPIRE, second Take MUST send EXPIRE (or later Take sets TTL on a leftover no-TTL key)
- [ ] 1.3 Run that test on unfixed `takeExact` and confirm FAIL

## 2. Exact Take EVAL

- [ ] 2.1 Add exact-Take EVAL: `INCR` `KEYS[1]`, `EXPIRE` `ARGV[1]` if `PTTL < 0`, return the increment; digest at package init; do not rewrite `flushScript`
- [ ] 2.2 Switch `takeExact` to `Eval` of that script then previous-window GET; do not refresh TTL when PTTL is set; do not DEL
- [ ] 2.3 Fake: store per-key expire, serve `PTTL`, apply exact EVAL expire-if-no-ttl, record Lua `EXPIRE` as `lastExpire` / expire count

## 3. Proofs

- [ ] 3.1 Confirm `TestRepro_ExactExpireNotRetriedAfterFailure` PASSES (later Take sets TTL, or EVAL never leaves a no-TTL key)
- [ ] 3.2 Keep `TestTake_ExpireOnFirstHit` passing (`EXPIRE … 20`)
- [ ] 3.3 Run `go test -short -count=1 -timeout 60s ./windowcounter` until passing
- [ ] 3.4 Update `knowledge/devdocs/std_go_windowcounter.md` exact path to EVAL expire-if-no-TTL

## 4. Validate

- [ ] 4.1 Run `openspec validate --change windowcounter-exact-expire-if-no-ttl --strict`
