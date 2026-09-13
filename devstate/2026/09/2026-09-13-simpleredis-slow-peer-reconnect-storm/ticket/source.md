# Investigate simpleredis slow-peer reconnect storm (BUG-5)

Investigate and, only if there is an elegant fix, fix ONE production defect in the `simpleredis` package of the Go repo at `D:\repositories\traefik-middleware-utilities` (module `github.com/david-garcia-garcia/traefik-middleware-utilities`).

### THE SIMPLICITY GATE — this overrides the workflow's "Done when"

Avoiding complexity matters more than landing a fix. If the simplest correct fix is not small, coherent, and elegant, **do not implement it**. Stop after propose, write up the options with their costs, and hand the decision back to the human.

The honest answer here may well be **"do not fix in code"**. The textbook remedy is a circuit breaker or a dial-rate limiter, and both add real state, real configuration, and real failure modes to a package whose whole point is being a small stdlib-only Yaegi-safe client. Concluding "no elegant fix exists, here is why, here is what to document instead" is a **fully successful outcome**. Do not talk yourself into shipping a breaker just to close a ticket.

### The defect

A peer that is merely slower than `IOTimeout` destroys the pooled socket on every single command, so the pool degenerates into connect-per-command.

Measured: 15 commands against a peer answering just above `IOTimeout` opened **15 TCP connections** and left 0 sockets in the idle list. Connection reuse is exactly zero.

Root cause: closing the socket on timeout is correct in isolation — the reply is still outstanding, so the socket genuinely cannot be reused. The problem is that nothing else changes. There is no backoff on the dial rate and no breaker, so a Redis that is merely slow receives a connection storm on top of the load that made it slow, pushing it toward `maxclients`, while the client accumulates TIME_WAIT sockets. With `Pass` or `Database` configured, every one of those reconnects also adds an AUTH and a SELECT round trip.

The default `IOTimeout` is 100 ms, which is inside the range a loaded Redis can exceed at p99. This does not need an outage to start.

The relevant code is `runOnConn` and `exec` in `simpleredis/commands_exec.go`, `release` in `simpleredis/pool.go`, and `do` in `simpleredis/resp.go`.

### Reproduction (read these; they are untracked in the main checkout, so read them by absolute path from the ORIGINAL repo, not only the worktree)

- `D:\repositories\traefik-middleware-utilities\simpleredis\PRODUCTION-BUGS.md` — full report, this is BUG-5
- `D:\repositories\traefik-middleware-utilities\simpleredis\bugs_production_test.go` — reproductions behind `//go:build simpleredis_bugs`

Run: `go test -tags simpleredis_bugs ./simpleredis/ -run 'TestBugSlowPeerDestroysEveryPooledSocket' -v`

### What to work out (this is the ask; prepare records it, does not solve it)

1. Confirm the measurement independently, and quantify the real-world cost: how many extra connections per second a given request rate produces, and what that does with AUTH plus SELECT in the handshake.
2. Establish whether this is genuinely distinct from correct behaviour. Closing a socket with an outstanding reply is not optional. The question is only whether the *dial rate* should be governed, and whether this package is the right place for that.
3. Consider the cheap non-breaker options honestly: is a better default `IOTimeout` the actual answer? Does the existing retry backoff already dampen part of this? Would documenting the interaction between `IOTimeout`, `PoolSize`, and the server's `maxclients` serve the operator better than code?
4. Only if something genuinely small and coherent falls out, implement it.

### Constraints

- Only `simpleredis`. Do not touch other packages.
- Go 1.21 target, stdlib only, must stay Yaegi-interpretable (this runs as a Traefik plugin). Any background goroutine or shared mutable state is a significant complexity cost here — weigh it explicitly.
- Comment style is dense and explanatory — comments state the constraint or the reason, never restate the code. Match it exactly.
- Preserve existing invariants: in-use-turn semaphore sound, `OverFrees() == 0`, no fd or goroutine leaks.
- Five sibling agents are fixing other `simpleredis` bugs in parallel, several in the same files. Keep any diff surgical.
- Do not change the default `IOTimeout` without saying clearly that it is a behaviour change for every existing caller, and treat that as a decision for the human, not for you.

### If we do implement (later, not this phase)

Add a permanent test to the **default** (untagged) suite that fails before the fix and passes after, showing connection churn is bounded under a peer slower than `IOTimeout`. Prefix any new fake or helper with something bug-specific so it cannot collide with sibling branches.

### If we do not implement (later)

Still deliver real work: the measurements, the options with their costs, recommendation, and the documentation change. Write into the run's bus files. Do not open a code PR that changes behaviour — a stub PR for the bus/docs is OK.

### Verify (later, not this phase)

If code changed: `go vet ./simpleredis/`, `go build ./...`, `go test ./simpleredis/ -count=1`, `go test ./... -count=1 -short`.
