# A panic between borrow and release permanently bricks the pool and leaks the socket fd

`exec` in `simpleredis/commands_exec.go` releases the pooled connection WITHOUT `defer`. A panic between `borrow` and `release` therefore loses the in-use turn permanently AND leaks the socket's fd. `inUseTurns` is a fixed buffered channel created once in `New` and nothing refills it, so after `PoolSize` recovered panics every command returns `redis:unreachable` against a healthy Redis with zero open sockets — a permanent brick that looks exactly like a Redis outage. Traefik recovers a panicking middleware per request, so the process survives and loses a turn silently each time.

`release` was left non-deferred because PR #29 was closed believing a deferred release could not save the turn under Yaegi. **That belief is measurably false.** The requester probed Yaegi v0.16.1 with an interpreted function that increments a counter, registers `defer func() { turns-- }()`, then panics, with a compiled caller recovering Traefik-style. The deferred restore ran in all three panic classes:

  explicit interpreted panic                -> Eval error "boom"                                            counter 0
  interpreter's own errors.As panic         -> "errors: *target must be interface or implement error"        counter 0
  runtime nil-map write                     -> "assignment to entry in nil map"                              counter 0

Yaegi converts the panic into an `Eval` error rather than propagating a Go panic outward, but it still unwinds the interpreted frames and runs their defers on the way.

A working prototype already exists on branch `origin/proto-defer-net` (based on `master`). It contains exactly two product changes plus two tests:
1. `commands_exec.go` — the inline `do` + `release` in `exec`'s loop body moves into a new `runOnConn` method with a deferred release and `reusable` initialised to `false`. `false` covers both "do said not reusable" and "do never returned", which demand the same action: destroy the socket. `reusable` must be declared BEFORE the `defer` so the closure captures the variable, and the assignment must be `values, reusable, err = sr.do(...)` (plain `=`, with `values` and `err` as named returns) so the closure observes the updated value.
2. `pool.go` — `borrow` gains a `handedOff` flag with a deferred `freeInUseTurn()` covering every path that does not hand a socket to the caller, including a panic in `takeIdleConn` or `dial`. The two explicit `sr.freeInUseTurn()` calls on the error paths are REMOVED because the defer now covers them; do not keep both or you will double-free and inflate `OverFrees`.
3. A panic test using a `bufio.Writer` over a panicking `io.Writer` so the panic happens inside `do` while `conn.netConn` stays a healthy socket that `release` can close.
4. The Yaegi defer-on-panic probe.

Measured on the prototype: `turns=2/2 idle=0 accepts=3 OverFrees=0` after two recovered panics, the client still serving, and `go test -count=1 -timeout 300s ./simpleredis/` green in 11.89s with no changes to any existing test.

Land it properly, do not redesign it:
- Keep the design. If something looks wrong, argue it on the delivery card with a measurement; do not silently change approach.
- Rename the two scratch test files to permanent names that match this package's conventions. The `zz_` prefixes and `proto` / `scratch` words must not survive. Suggested: the panic test into `simpleredis/pool_test.go` or a new `simpleredis/panic_safety_test.go`; the Yaegi probe into a new `simpleredis/yaegi_defer_test.go`. Rename the test functions to drop `Proto` / `Scratch` too.
- The Yaegi probe MUST be committed as a permanent test. It is the evidence that overturns PR #29.
- Do NOT add `heldSockets`, `lostTurns`, `turnRecoverMu`, `LostTurns()`, `recoverLostTurnsLocked`, `borrowAfterPoolWait`, or any leak-detection or turn-refill machinery. PRs #66 and #70 took that route and are now CLOSED.
- Do NOT create `knowledge/debt/2026-09-13-simpleredis-close-panic-leaked-fd.md` or `knowledge/debt/2026-09-13-simpleredis-held-socket-lease.md`. Those existed only on the closed #66 branch and are obsolete.
- Update the stale comment in `resp.go` on `do` that says "Any panic here loses the in-use-turn when this client runs in a Traefik middleware: Traefik recovers the request and release never runs." That is no longer true after this change.
- Update `openspec/specs/std_go_simpleredis_tcp-session/spec.md` for the new guarantee: an in-use turn is returned and the socket closed even when the command panics.
- `knowledge/devdocs/std_go_simpleredis.md` deserves a gotcha entry: interpreted `defer` runs under Yaegi, with the measured panic classes, so future work does not repeat #29's assumption.

Open question the later phases must resolve or record: probe ran against yaegi v0.16.1. Check go.mod / go.sum and which Traefik release this plugin targets. If you cannot confirm Traefik ships v0.16.1 or newer, record it as an assumed decision in explore; do not block prepare on it.

Package constraints: simpleredis source imports ONLY the Go standard library. No unsafe, no cgo, no generics. Retry jitter stays math/rand Int63n. Runs interpreted under Yaegi — keep existing workarounds. Pool/timeout/retry knobs frozen at New on Config.

Do not implement product code in prepare. Ground the ticket: dump, branch, stub PR, requirement.md, comments inventory, qualify.
