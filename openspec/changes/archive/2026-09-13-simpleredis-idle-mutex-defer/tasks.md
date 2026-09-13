## 1. Product

- [x] 1.1 Extract `parkIdleConn` from `release`: lock, `defer` unlock, keep-or-close, append on success, return bool. Do not close or `freeInUseTurn` inside it.
- [x] 1.2 Rewrite `release`: stamp `lastUsed` on the reusable path, close when `!reusable || !parkIdleConn(conn)`, then `freeInUseTurn()` last on every path. No `defer Unlock()` on `release` itself.
- [x] 1.3 Convert `cachedGroupWrite` and `storeGroupWrite` manual `groupWriteMu` unlocks to `defer`.
- [x] 1.4 Leave `Close`, `takeIdleConn`, `resp.go` `errors.Is` / AfterFunc, `math/rand`, and `OverFrees` unchanged.

## 2. Docs

- [x] 2.1 Rewrite `simpleredis/BUGS.md` section 2 to the Yaegi-defer finding and point at `simpleredis/yaegi_defer_test.go`.
- [x] 2.2 Grep `knowledge/devdocs/std_go_simpleredis.md` and openspec specs for a stale "defer does not run" claim; leave them unless one remains. Confirm dest already dropped the `resp.go` / `commands_exec.go` #29 comments.

## 3. Tests

- [x] 3.1 `go vet ./simpleredis/`
- [x] 3.2 `go test -count=1 -timeout 300s ./simpleredis/` including Yaegi interpreter tests. No production hooks for a panic inside the locked section.
- [x] 3.3 `go test -count=5 -timeout 600s -run "Pool|Release" ./simpleredis/`
- [x] 3.4 Run `openspec validate --change simpleredis-idle-mutex-defer --strict`
