# Windowcounter buffered Take on Redis outage is per-node fallback (accepted)

THIS IS ACCEPTED BEHAVIOR, not a fail-closed fix. Buffered Take when Redis is down keeps per-node limit, err=nil. Do NOT GET/INCR every Take to probe Redis. Do NOT return redis:unreachable because localDelta>0 skipped GET. Exact mode still returns Redis errors. Spec “Redis errors propagate” on buffered Take is a deviation: rewrite/lock tests for nil error + local deny after limit. Document the deviation (devdocs + spec delta).

Implement order (required):
1. CREATE tests first that prove the contract (copy/adapt example, then rewrite assertions to the agreed contract so they pass as the lock — they currently FAIL wanting unreachable). Example: `d:\repositories\traefik-middleware-utilities\windowcounter\repro_buffered_outage_test.go`. Fake helpers: `closeListenerAndConns` / trackConn in `windowcounter\fake_redis_test.go` in that same parent tree — port only what this ticket needs into the worktree.
2. Then apply the agreed docs/spec/test rewrite. Do not change takeBuffered to fail-closed.
3. Confirm `go test -short -count=1 -timeout 60s ./windowcounter` (and the new tests) PASS.

Bound the ask: only this bug. Do not fix bugs 2–7.
