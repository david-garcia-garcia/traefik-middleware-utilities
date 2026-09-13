# Buffered Take holds l.mu across Redis GET

THIS BUG ONLY.

takeBuffered/peekBuffered/flushPending hold l.mu across Redis GET/Eval. A 400ms GET on key slow blocked Take on unrelated key fast for 400ms.

Agreed how: Do not call Redis while holding l.mu. windowLocked / first-sight Peek GET: snapshot under lock, unlock, getCount, re-lock, apply redisKnown (merge if another Take inserted). flushPending: copy (key,delta,expireAt) under lock, unlock, Eval, re-lock: redisKnown=n, localDelta -= flushedDelta (do not set localDelta=0 blindly). Exact mode unchanged. Fast Take must finish well under the GET delay.

Implement order (required):
1. CREATE the failing repro FIRST. Example: `d:\repositories\traefik-middleware-utilities\windowcounter\repro_lock_during_get_test.go`. Port blockGetPrefix / waitGetBlocked / unblockGet from parent `windowcounter\fake_redis_test.go`. Confirm FAIL (~400ms wait).
2. Then implement unlock-around-I/O.
3. Confirm that test PASSES and `go test -short -count=1 -timeout 60s ./windowcounter` passes.

Bound the ask: only this bug. Do not implement bug 2's previous GET in this PR unless required for compile.
