## 1. Exec release on panic

- [ ] 1.1 Add a named helper next to `exec` that runs `do` then always `release`, with `defer` scoped to that attempt. On panic leave `reusable` false. Do not recover
- [ ] 1.2 Call that helper from `exec` after a successful `borrow` instead of `do` then `release`. Keep the retry loop otherwise unchanged

## 2. Unit proof

- [ ] 2.1 In `simpleredis/pool_test.go`, add a test with `startStaticRedis` reply `*1000000000000000000\r\n`, `PoolSize: 2`, `MaxRetries: -1`. Recover two command panics, assert `len(inUseTurns) == cap(inUseTurns)`, then `borrow` succeeds without waiting `PoolTimeout`
- [ ] 2.2 Run `go test -short ./simpleredis/...`

## 3. Specs

- [ ] 3.1 Confirm the change delta `std_go_simpleredis_tcp-session` matches the landed helper and test
- [ ] 3.2 Run `openspec validate --change simpleredis-exec-panic-releases-token --strict`
