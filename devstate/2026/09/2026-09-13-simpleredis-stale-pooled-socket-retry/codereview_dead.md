# Dead

1. [hard] Leftover production path — `simpleredis/pool.go:72` — `borrow` has no callers outside tests after `exec` moved to `borrowSocket`

```
func (sr *SimpleRedis) borrow(ctx context.Context) (conn *pooledConn, err error, handshakeFailed bool) {
	conn, err, handshakeFailed, _ = sr.borrowSocket(ctx, false)
	return
}
```

   Grep `.borrow(` on the worktree (ignore `*_test.go`, `openspec/`, `devstate/`, `.cursor/`): definition only. Remaining hits are `pool_test.go:566`, `pool_e2e_test.go:58`, `panic_safety_test.go:31`. `borrowSocket` is the production path (`commands_exec.go:39`).
   → Delete `borrow`; retarget those tests to `borrowSocket(ctx, false)` (drop `fromIdle`).
   Status: done
   Argument: deleted `borrow`; tests call `borrowSocket(ctx, false)`.
