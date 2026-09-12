## 1. Non-blocking over-free

- [x] 1.1 Add `overFrees atomic.Int64` on `SimpleRedis` and export `OverFrees() int64` next to `PoolSize()` / `MaxIdleConns()`
- [x] 1.2 Change `freeInUseTurn` to `select` with `default`; on the default branch `overFrees.Add(1)`
- [x] 1.3 Keep the nil `inUseTurns` no-op

## 2. Tests

- [x] 2.1 Guard: same-package test in `pool_test.go` that `New` then one extra `freeInUseTurn` returns before a short timer and `OverFrees()` is 1
- [x] 2.2 Invariant: many goroutines × healthy fake, dead address, AUTH-rejecting fake, starved pool with short `PoolTimeout`; after wait, `len(inUseTurns)==cap(inUseTurns)` and `OverFrees()==0`
- [x] 2.3 Run `go test -short ./simpleredis/...` until the new tests pass
- [x] 2.4 Run `go test -race -count=1 -timeout 60s -run TestOverFree ./simpleredis` (or the invariant test name) until it passes; do not edit CI workflows

## 3. Usage packet and spec

- [x] 3.1 Add a gotcha on `knowledge/devdocs/std_go_simpleredis.md` that dest used to hang on over-free and now drops and counts via `OverFrees()`
- [x] 3.2 Confirm the change delta `std_go_simpleredis_tcp-session` matches the landed tests
- [x] 3.3 Run `openspec validate --change simpleredis-over-free-nonblocking --strict`
