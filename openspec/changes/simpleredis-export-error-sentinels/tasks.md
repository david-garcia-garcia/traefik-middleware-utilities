## 1. Export sentinels

- [ ] 1.1 Export `ErrUnreachable`, `ErrMiss`, `ErrTimeout`, `ErrNoAuth`, `ErrIssue`, `ErrPoolWait` in `simpleredis/simpleredis.go`. Keep the string consts. `ErrPoolWait` wraps `ErrUnreachable`. Unexported names alias the exported values
- [ ] 1.2 Add `IsMiss`, `IsUnreachable`, `IsPoolWait` via `errors.Is`. Do not add other predicates
- [ ] 1.3 Keep `shouldRetry` / `isUnreachable` / `isCommandTimeout` on identity so `shouldRetry(ErrPoolWait)` is false and `shouldRetry(ErrUnreachable)` is true

## 2. Tests in simpleredis

- [ ] 2.1 Wrap each sentinel with `%w` and assert `errors.Is` plus the matching predicate still match while `err.Error() ==` the token does not
- [ ] 2.2 Pin `errors.Is(ErrPoolWait, ErrUnreachable)`, `IsPoolWait` distinction, `shouldRetry(ErrPoolWait)==false`, `shouldRetry(ErrUnreachable)==true`
- [ ] 2.3 Add `errors.Is` on an exported sentinel and one predicate to `clientprobe` in `simpleredis/yaegi_test.go`. If interpreted `errors.Is` on package vars fails, keep the predicate and pin that outcome
- [ ] 2.4 Run `go test -short ./simpleredis/...`

## 3. Window counter

- [ ] 3.1 Extract miss-as-zero classification `getCount` owns (`countFromRedisGet` or equivalent). Convert `windowcounter/limiter.go` to `simpleredis.IsMiss`. Do not add a Redis interface
- [ ] 3.2 Test wrapped `ErrMiss` through that helper: counter is zero, no hard error. Do not convert `e2e/simpleredisprobe` or `parseEvalInt`
- [ ] 3.3 Run `go test -short ./windowcounter/...`

## 4. Specs

- [ ] 4.1 Confirm deltas `std_go_simpleredis_resp-commands`, `std_go_simpleredis_tcp-session`, and `std_go_windowcounter_sync-flush` match the landed code
- [ ] 4.2 Run `openspec validate simpleredis-export-error-sentinels --strict`
