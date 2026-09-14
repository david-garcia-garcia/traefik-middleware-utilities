# Sequential SimpleRedis requests fail with redis:unreachable after every pooled socket is dropped

Fix ONE production defect in the `simpleredis` package.

### THE SIMPLICITY GATE — this overrides the workflow's "Done when"

Avoiding complexity matters more than landing a fix. If the simplest correct fix is not small, coherent, and elegant, **do not implement it**. Stop after propose, write up the options with their costs, and hand the decision back to the human. Stopping with a clear written recommendation is a SUCCESS outcome, not a failure. Never ship a speculative abstraction, a config knob nobody asked for, or a mechanism whose moving parts outnumber the bug.

### The defect

After every pooled socket is dropped server-side (a Redis restart, a failover, `CLIENT KILL`, or an idle-timeout reap by the server), sequential requests fail with `redis:unreachable` against a peer that is healthy and accepting the whole time.

Measured: the burst is exactly `PoolSize / (MaxRetries+1)` consecutive failed requests. At defaults (PoolSize 8, MaxRetries 1) that is 4. At PoolSize 32 it is 16.

Root cause: `borrow` in `simpleredis/pool.go` (around line 117-135) returns a socket with no indication of whether it came off the idle list or was freshly dialled:

```go
	if reused != nil {
		handedOff = true
		return reused, nil, false
	}
	// Idle miss: dial while still holding the turn.
	conn, err, handshakeFailed = sr.dial(ctx)
```

So `exec` in `simpleredis/commands_exec.go` (around line 37-52) cannot distinguish "the socket I reused was already dead" from "the peer is down". Both surface as `errUnreachable`, the retry calls `borrow` again, and it pops the *next* corpse off the same idle list. Each request destroys exactly `MaxRetries+1` dead sockets and then gives up. Nothing invalidates the rest of the stale vintage.

Important nuance: concurrency hides this completely. With 12 parallel workers there are zero failures, because parallel callers drain the corpses simultaneously and every retry then dials fresh. It only bites on low-traffic paths — which for a Traefik rate-limiter is the common case after a failover.

### Reproduction (read these; they are untracked in the main checkout, so read them by absolute path)

- `D:\repositories\traefik-middleware-utilities\simpleredis\PRODUCTION-BUGS.md` — full report, this is BUG-1
- `D:\repositories\traefik-middleware-utilities\simpleredis\bugs_production_test.go` — reproductions behind `//go:build simpleredis_bugs`

Run: `go test -tags simpleredis_bugs ./simpleredis/ -run 'TestBugDeadIdle' -v`

### Fix direction (evaluate, do not follow blindly)

The cheapest shape is probably: have `borrow` report whether the socket was reused, and make `exec` treat an I/O failure on a *reused* socket as "not evidence about the peer" — so it does not consume the retry budget and the next attempt is forced onto a fresh dial. It must be hard-bounded so it can never loop.

A full pool-generation / epoch mechanism is the textbook fix but is very likely over-engineered for this codebase. If that is what it takes, that is a strong signal to stop at the gate and ask.

### Constraints

- Only `simpleredis`. Do not touch other packages.
- Go 1.21 target, stdlib only, must stay Yaegi-interpretable (this runs as a Traefik plugin): no generics tricks, no reflection, no new imports beyond stdlib already used in the package.
- Comment style is dense and explanatory — comments state the constraint or the reason, never restate the code. Match it exactly.
- Preserve the existing invariants: the in-use-turn semaphore must stay sound (no lost turns, `OverFrees() == 0`), no fd leaks, no goroutine leaks. These are currently correct; verify they still are.
- Five sibling agents are fixing other `simpleredis` bugs in parallel. BUG-6 (idle socket reaper) also touches `pool.go` borrow paths. Keep your diff surgical.
- A branch `2026-09-13-simpleredis-clamp-maxidleconns-to-poolsize` is already pushed and unmerged; ignore it.

### Regression test

Add a permanent test to the **default** (untagged) suite that fails before your fix and passes after. It must drive real TCP against an in-process RESP fake, warm the pool with genuinely simultaneous in-flight commands, drop every server socket, then assert sequential requests still succeed. Prefix any new fake or helper type with something bug-specific so it cannot collide with the sibling branches. Reuse `simpleredis/fake_redis_test.go` helpers where they fit.

### Verify and report

Run `go vet ./simpleredis/`, `go build ./...`, `go test ./simpleredis/ -count=1`, and `go test ./... -count=1 -short`. Also re-run the tagged reproduction to show it now passes.
