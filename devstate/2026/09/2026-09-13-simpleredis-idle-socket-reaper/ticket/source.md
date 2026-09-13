# Fix idle-socket reaping in simpleredis (BUG-6)

Fix ONE production defect in the `simpleredis` package of the Go repo at `D:\repositories\traefik-middleware-utilities` (module `github.com/david-garcia-garcia/traefik-middleware-utilities`).

Treat this as an unattended full run, subject to the simplicity gate below.

## THE SIMPLICITY GATE — this overrides the workflow's "Done when"

Avoiding complexity matters more than landing a fix. If the simplest correct fix is not small, coherent, and elegant, **do not implement it**. Stop after propose, write up the options with their costs, and hand the decision back to the human. Stopping with a clear written recommendation is a SUCCESS outcome, not a failure.

## The defect

`IdleTimeout` is documented as an idle reuse gate but behaves as nothing at all when traffic stops: the sweep only runs inside `takeIdleConn`, which only runs when somebody borrows. With no traffic, nothing is ever reaped.

Measured: with `IdleTimeout` at 50 ms and traffic stopped, after 500 ms of silence the idle list still held 4 sockets and the server still had 4 sockets open.

Root cause, `takeIdleConn` in `simpleredis/pool.go` around line 145:

```go
	now := time.Now()
	// Keep still-young sockets in place; collect stale for close after unlock.
	survivors := sr.idleConns[:0]
	for _, conn := range sr.idleConns {
		if now.Sub(conn.lastUsed) < sr.idleTimeout {
			survivors = append(survivors, append(survivors, conn)
			continue
		}
		stale = append(stale, conn)
	}
```

There is no background reaper, so a client with no traffic holds `PoolSize` sockets and their fds indefinitely.

Why it matters, and be precise about this in your analysis: standalone it is mostly fd and `maxclients` pressure proportional to the number of plugin instances. Its bigger role is as an **amplifier** of a separate, more serious defect — those pinned sockets are exactly the ones Redis will eventually drop on its own server-side `timeout`, and a sibling bug makes a pool full of such corpses fail real requests after a restart. Quiet time therefore converts into guaranteed failures on the next request.

## Reproduction (untracked in the main checkout; read by absolute path)

- `D:\repositories\traefik-middleware-utilities\simpleredis\PRODUCTION-BUGS.md` — full report, this is BUG-6 (the sibling defect is BUG-1)
- `D:\repositories\traefik-middleware-utilities\simpleredis\bugs_production_test.go` — reproductions behind `//go:build simpleredis_bugs`

Run: `go test -tags simpleredis_bugs ./simpleredis/ -run 'TestBugIdleSocketsAreNeverReaped' -v`

## Fix directions (evaluate, pick at most one)

1. A background reaper: a ticker goroutine started in `New` and stopped in `Close`. Straightforward, but it adds a goroutine to every client in a Traefik plugin process, changes `New`'s contract from "allocates nothing live" to "starts a goroutine", and makes forgetting `Close` a leak. The package currently has an explicit test that `New`/use/`Close` cycles keep the goroutine count flat — whatever you do must keep that true.
2. Stamp an absolute expiry on each socket when it is parked, so the reuse gate and the socket lifetime agree, without any background work. This does not release fds during silence, so be honest about whether it actually addresses the defect or only the reuse half of it.
3. Conclude the fd pinning is acceptable and the real fix belongs entirely to the sibling defect (validate on borrow), and document it.

Option 1 is the only one that truly releases fds while idle, and it is also the one with the highest complexity cost. Weigh that honestly rather than defaulting to it.

## Constraints

- Only `simpleredis`. Do not touch other packages.
- Go 1.21 target, stdlib only, must stay Yaegi-interpretable (this runs as a Traefik plugin). If you add a goroutine, verify it behaves under Yaegi — the package has `TestYaegi_*` tests, and there is an existing comment in `resp.go` explaining that Yaegi's `select` handling forced the use of `context.AfterFunc` instead of a goroutine plus `select`. Read that comment before adding any goroutine or `select`.
- Comment style is dense and explanatory — comments state the constraint or the reason, never restate the code. Match it exactly.
- `Close` must stay safe to call more than once, and must not race the reaper. `simpleredis/lifecycle_test.go` has the goroutine and fd leak checks — they must stay green.
- Preserve existing invariants: in-use-turn semaphore sound, `OverFrees() == 0`, no fd or goroutine leaks.
- Five sibling agents are fixing other `simpleredis` bugs in parallel. BUG-1 (stale pooled socket retry) also touches `pool.go` borrow paths, so a conflict there is likely. Keep your diff surgical and confined to reaping — do not also change how `borrow` validates or reports a reused socket.

## Regression test

Add a permanent test to the **default** (untagged) suite that fails before your fix and passes after: park several sockets, stop all traffic, and assert that after a multiple of `IdleTimeout` both the idle list and the server-side open socket count have dropped. Add a companion assertion that the goroutine count returns to its starting value after `Close`. Prefix any new fake or helper with something bug-specific so it cannot collide with sibling branches.

## Verify and report

Run `go vet ./simpleredis/`, `go build ./...`, `go test ./simpleredis/ -count=1`, `go test ./... -count=1 -short`, and the Yaegi tests. Re-run the tagged reproduction to show it now passes.
