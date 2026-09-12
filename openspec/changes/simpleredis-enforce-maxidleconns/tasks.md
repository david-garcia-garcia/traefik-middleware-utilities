## 1. Tests that pin the gap

- [ ] 1.1 Replace `TestReleaseKeepsSocketWhenLiveUnderCap` with concurrent Gets (PoolSize 16, default MaxIdleConns 8): after Wait, `len(idleConns) == 8`, `waitOpenSocketsEqual(8)`, then a reuse Get that does not increase `connections()`
- [ ] 1.2 Add a sequential `borrow`/`release` table for PoolSize/MaxIdleConns `8/2`, `16/1`, `2/8`, `8/8`: after each return record idle; after the last, `len(idleConns) == min(MaxIdleConns, PoolSize)` and `waitOpenSocketsEqual` that same count
- [ ] 1.3 Run `go test ./simpleredis -run 'TestReleaseKeepsSocketWhenLiveUnderCap|TestIdleCap'` (or the new table name) and confirm DestBranch fails the new assertions before the `release` edit

## 2. Idle-only release

- [ ] 2.1 In `simpleredis/pool.go` `release`, close when `closed` or `len(idleConns) >= maxIdleConns`. Delete `inUse` / `live`. Keep `liveCap()` for `PoolSize()`. Update the method comment
- [ ] 2.2 Run `go test -short ./simpleredis/...` until pass

## 3. Specs

- [ ] 3.1 Confirm the delta `std_go_simpleredis_tcp-session` matches the landed tests
- [ ] 3.2 Run `openspec validate --change simpleredis-enforce-maxidleconns --strict`
