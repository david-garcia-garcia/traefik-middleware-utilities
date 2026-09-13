## 1. Tests first (dest FAIL)

- [x] 1.1 Add `tokenbucket/ttl_truncation_test.go` adapted from the caller repro: dest `Allow(ctx, key) (bool, time.Duration, error)`. NewMemory and NewRedis with ttl 1500ms succeed. After one Redis Allow, fake EVAL ARGV ttl (index 6) is `"1"`. Memory depleted at t0 is not expired at t0+1200ms (still not a fresh burst).
- [x] 1.2 Run `go test -short -count=1 ./tokenbucket/ -run TTLTruncation`. Confirm FAIL on dest (New accepted 1500ms / ARGV 1 / Memory live at +1200ms). Do not edit `validateClock` until that FAIL is measured.

## 2. Gate

- [x] 2.1 In `validateClock`, reject `ttl` that is not a whole number of seconds (still `>= 1s`). Same `errTTL` sentinel. Text: `tokenbucket: ttl must be a whole number of seconds (at least 1s)`. Do not PEXPIRE, rewrite Lua, or floor Memory.
- [x] 2.2 Reshape `ttl_truncation_test.go`: NewMemory/NewRedis 1500ms return `errors.Is(err, errTTL)`; NewMemory/NewRedis 2s succeed. Drop ARGV=`1` / Memory-at-+1200ms as the post-fix contract.
- [x] 2.3 Run `go test -short -count=1 ./tokenbucket/`. Confirm PASS. Update usage gotcha in `knowledge/devdocs/std_go_tokenbucket.md` (`ttl < 1s` → whole seconds).

## 3. Specs

- [x] 3.1 Confirm the change deltas `std_go_tokenbucket_allow` and `std_go_tokenbucket_lua-eval` match the landed tests
- [x] 3.2 Run `openspec validate --change tokenbucket-ttl-whole-seconds --strict`
