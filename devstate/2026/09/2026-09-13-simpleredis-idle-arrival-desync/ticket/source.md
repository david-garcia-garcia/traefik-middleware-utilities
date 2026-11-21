Fix ONE production defect in the `simpleredis` package.

### THE SIMPLICITY GATE (overrides workflow Done when)

Avoiding complexity matters more than landing a fix. If the simplest correct fix is not small, coherent, and elegant, do not implement it. Stop after propose, write up the options with their costs, and hand the decision back to the human. Stopping with a clear written recommendation is a SUCCESS outcome. This particular bug has a real chance of having no elegant fix; be honest about that rather than forcing one.

### The defect

This is the worst class of failure in the package: silently wrong data, no error.

A reply that arrives while a socket is parked on the idle list leaves that socket one reply ahead, permanently. Later commands then return the previous command's payload with err == nil. Measured: Get(k2) returned "POISONED", and across 59 commands 6 returned another key's value with no error. Other runs measured 3, 4, and 17 wrong; one run had 4 out of 4 consecutive commands wrong with no eviction at all.

For a rate limiter this means counters read from the wrong window key, so requests get admitted or denied on another key's number.

Root cause: both boundary checks in `do` (`simpleredis/resp.go`, one before the write around line 39-41 and one after the read around line 58) use `bufio.Reader.Buffered()`, which sees only the bufio buffer and never the kernel receive queue. The code comment already admits this:

```
	// Buffered() covers leftover already in the reader, not a stray that arrives while the socket is idle: it does not see the kernel receive buffer.
	if conn.reader.Buffered() != 0 {
		return nil, false, errUnreachable
```

A stray that lands after the post-read check and before the next pre-write check is invisible to both, so the socket is parked clean and reused clean while sitting one reply ahead. Whether it ever self-heals is pure TCP luck: it is only evicted if the stray happens to coalesce into the same segment as a later reply.

Trigger: a peer that emits an unsolicited or late reply — a proxy such as Envoy or twemproxy, a cluster front-end, or a non-Redis engine. A single well-behaved Redis will not do it. An earlier hunt filed this as debt for exactly that reason (see `simpleredis/BUGS.md`, and note the `knowledge/debt/2026-09-13-simpleredis-idle-arrival-desync.md` it references does not exist in the tree). Given the blast radius, "needs a proxy" is not a good enough reason to leave it open — but it does mean a costly fix is hard to justify.

### Reproduction (untracked in the main checkout; read by absolute path)

- `D:\repositories\traefik-middleware-utilities\simpleredis\PRODUCTION-BUGS.md` — full report, this is BUG-2
- `D:\repositories\traefik-middleware-utilities\simpleredis\bugs_production_test.go` — reproductions behind `//go:build simpleredis_bugs`

Run: `go test -tags simpleredis_bugs ./simpleredis/ -run 'TestBugIdleArrivalDesync' -v`

### Fix directions (evaluate all, pick at most one, or none)

1. Establish the boundary by reading instead of by inspecting. Before reusing a parked socket, set a zero read deadline and attempt a one-byte read: a timeout means the socket is clean, anything else (data or EOF) means it is dirty and must be destroyed. Consuming that byte is fine because the socket is discarded either way. Costs one extra syscall per reuse. Must restore the deadline afterwards, and must be stdlib-only and Yaegi-safe — `syscall.Conn` / `RawConn` / poll tricks are very likely NOT available under Yaegi, so verify before relying on them.
2. Per-command correlation: verify the reply shape against the command actually sent. Catches the desync at the point it produces wrong data without preventing it.
3. Conclude there is no elegant fix, and instead make the failure loud rather than silent.

Option 1 is the most likely to be both simple and correct, but measure its cost before committing to it.

### Constraints

- Only `simpleredis`. Do not touch other packages.
- Go 1.21 target, stdlib only, must stay Yaegi-interpretable (this runs as a Traefik plugin). Confirm any API you add is actually available under Yaegi v0.16.1 — the package already has `TestYaegi_*` tests, use them.
- Comment style is dense and explanatory — comments state the constraint or the reason, never restate the code. Match it exactly.
- Do not regress the hot path. An extra syscall on every single reuse is a real cost; measure it with the existing `simpleredis/bench_test.go` and report the number.
- Preserve existing invariants: in-use-turn semaphore sound, `OverFrees() == 0`, no fd or goroutine leaks.
- Five sibling agents are fixing other `simpleredis` bugs in parallel; two of them also touch `resp.go`. Keep your diff surgical.
- Regression test: Add a permanent test to the default (untagged) suite that fails before the fix and passes after: a fake peer that emits one unsolicited bulk while the client has the socket parked, then assert every later Get returns its own key's value or a clean error — never another key's payload with err == nil. Prefix any new fake or helper with something bug-specific so it cannot collide with sibling branches.
