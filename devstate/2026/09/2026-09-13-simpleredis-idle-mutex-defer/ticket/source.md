# Make simpleredis release idleConnsMu via defer

Deliver ONE pull request that makes `simpleredis` release its pool mutex via `defer`, plus two small cleanups. Repo is a Go 1.21 Traefik plugin library; this code also runs INTERPRETED under Yaegi. PR host is GitHub.

This work depends on PR #71 (`2026-09-13-simpleredis-panic-safe-release`), which rewrites `borrow` in the same file. #71 is NOT merged: branch from `origin/2026-09-13-simpleredis-panic-safe-release`, set GitHub PR base to that branch, state in the PR body that it is stacked on #71 and must merge after it.

Background: The premise that a `defer` does NOT run when a panic happens under Yaegi is FALSE. On Yaegi v0.16.1 an interpreted `defer` runs for an explicit interpreted panic, for the interpreter's own `errors.As` panic, and for a nil-map write. PR #71 acts on that by deferring `release` in `exec` and deferring `freeInUseTurn` in `borrow`. This PR extends the same reasoning to the one remaining hand-unwound resource: the `idleConnsMu` mutex in `release`.

## Task A (the real work): make `release` release `idleConnsMu` via defer

In `simpleredis/pool.go`, `release` currently takes `sr.idleConnsMu.Lock()` and unlocks it manually on two separate exit paths. A panic inside that critical section leaves the mutex locked forever, which is WORSE than the turn leak #71 fixes: every subsequent `borrow` blocks in `takeIdleConn`, every `release` blocks, and `Close` blocks, permanently, while Traefik keeps the process alive accumulating stuck goroutines.

HARD REQUIREMENT: a naive `defer sr.idleConnsMu.Unlock()` at the top is a REGRESSION and you must not do it. The current code unlocks BEFORE calling `conn.close()` and `sr.freeInUseTurn()` on purpose, because you must never hold a mutex across a socket-close syscall or a channel send. Instead, extract the locked decision into its own small method so `defer` owns the unlock, and act on its verdict after it returns. Sketch (adapt names to repo conventions):

```go
func (sr *SimpleRedis) release(conn *pooledConn, reusable bool) {
	if !reusable || !sr.parkIdleConn(conn) {
		conn.close()
	}
	sr.freeInUseTurn()
}
```

where `parkIdleConn` takes the lock, `defer`s the unlock, makes the keep-or-close decision, appends on success, and returns a bool. Net effect: two `conn.close()` call sites collapse to one and three `sr.freeInUseTurn()` call sites collapse to one.

SEMANTICS THAT MUST NOT CHANGE:

1. The close decision is exactly: close when the client is closed, OR when `idleConns` is already at `maxIdleConns` AND `live >= liveCap()`. Otherwise park.
2. `conn.lastUsed` is still stamped before the socket is parked.
3. `freeInUseTurn()` still runs LAST on every path. Load-bearing: the existing comment says "inUse still includes this socket until freeInUseTurn runs", and the `inUse = liveCap() - len(inUseTurns)` computation inside the critical section depends on this socket's turn still being held. If you free the turn before or during the locked section you will corrupt the live-count arithmetic.
4. `OverFrees` behavior is unchanged: exactly one free per release, no double frees.

Do NOT add production hooks or seams purely to unit-test a panic inside the critical section. Add a test only if you can do it without changing production API surface.

## Task B (cosmetic): defer the two trivial mutex unlocks

In `simpleredis/commands_msetex.go`, `cachedGroupWrite` and `storeGroupWrite` each hold `groupWriteMu` across a single field access with a manual unlock. Convert both to `defer`. Nothing there can panic; this is consistency, so keep it to a two-line change.

## Task C: retire the stale guidance that will mislead the next reader

`simpleredis/BUGS.md` section 2 currently instructs: "Do not simply switch to `defer sr.release(...)`; PR 29 rejected that and the reason should be re-read before touching it." That instruction is now DISPROVEN. Replace it with the measured finding: interpreted `defer` DOES run on panic under Yaegi v0.16.1, verified for an explicit interpreted panic, the interpreter's own `errors.As` panic, and a nil-map write. Point at the permanent proof test PR #71 adds (`simpleredis/yaegi_defer_test.go`) rather than restating the evidence.

Also grep `knowledge/devdocs/std_go_simpleredis.md` and the openspec specs for the same stale claim and fix it there if present.

PR #71 already handles the stale comments in `simpleredis/resp.go` (the `do` comment saying "release never runs") and in `simpleredis/commands_exec.go` (the `pull/29` comment). Verify they are gone on your base; do not duplicate that work, but DO fix them if #71 missed one.

## Explicitly DO NOT change

- `Close` in `simpleredis/simpleredis.go`: already uses the correct "swap the slice under the lock, close the sockets outside it" idiom.
- `takeIdleConn` returning `stale` sockets for the caller to close after the lock: deliberate lock hygiene.
- `resp.go` using `errors.Is` instead of a `net.Error` type assertion.
- `resp.go` using `context.AfterFunc` instead of `go` plus `select` on `ctx.Done()`.
- The `math/rand` constraint on retry jitter.
- The `OverFrees` counter.

## Other branches in flight that touch the same files (document as conflicts, do not narrow scope)

- `2026-09-13-simpleredis-close-abandoned-socket` (likely touches `release`)
- `2026-09-13-simpleredis-desync-boundary-check` (touches `resp.go`)
- `2026-09-13-simpleredis-resilience-test-coverage`
- a separate in-flight PR that hardens `.golangci.yml` and gives named results to `dial` and `borrow` in `pool.go` (`2026-09-13-golangci-lint-harden`)

## Validation (for later phases)

Run the full local test suite including Yaegi interpreter tests, run `go vet`, wait for measured CI green. Run the pool and release tests with `-count=5`.
