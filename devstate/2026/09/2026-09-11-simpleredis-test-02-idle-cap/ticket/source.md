# test-02 — Pool cap and release-after-Close never execute

issueHost: local
issueRef: none
source: simpleredisfixes/test-02-idle-cap-and-release-after-close.md
index: simpleredisfixes/README.md

## Caller hard requirement

Tests MUST run against both Redis and Dragonfly. Both are supported backends. Cap and Close contracts must be unit-tested as the finding specifies; live overlap against both engines must not leak idle sockets beyond the cap. Extend compose + Pester `/redis` `/dragonfly`. Lua 5.1-safe. Dragonfly KEYS required.

## Index context (simpleredisfixes/README.md)

Review of `simpleredis/` for hot-path efficiency and test coverage. One file per finding. Nothing here is applied to `simpleredis/simpleredis.go` yet.

RESP encode/decode is not the bottleneck. The costs that matter include connection management (measured: 86% of commands paid a fresh TCP handshake under bursty load).

Findings table row:

| # | Axis | Severity | Finding |
|---|---|---|---|
| test-02 | Coverage | hard | Pool cap and release-after-`Close` never execute |

Suggested order: test-01 through test-05 after perf-01/perf-02; these are the failure modes perf-01 exposes.

## Finding

- **Axis**: Test coverage
- **Severity**: hard
- **Where**: `simpleredis/simpleredis.go:245-260` (`release`), `:64-78` (`Close`), `:233-238` (`borrow` second closed check)
- **Status**: not applied

### What I found

The entire "do not pool this connection" branch of `release` has **zero coverage**
(measured block `253.47,257.3`, 3 statements):

```go
sr.mu.Lock()
if sr.closed || len(sr.idle) >= maxIdleConns {
	sr.mu.Unlock()
	conn.close()      // never executed by any test
	return
}
```

So neither half of that condition is proven: not the idle cap, and not the
release-after-`Close` case. The second `closed` check in `borrow` (`:236-238`) is
also uncovered.

`TestCloseDrainsIdleAndDoesNotRepool` appears to cover the `Close` half, and its
final assertion even says so:

```go
if len(redis.idle) != 0 {
	t.Fatalf("release after Close idle = %d, want 0", len(redis.idle))
}
```

That assertion is **vacuous**. The preceding `Get` after `Close` returns
`redis:unreachable` from `borrow`'s first `closed` check at `:210-212`, so no
connection is ever borrowed and `release` is never called. The test asserts that a
function which never ran left the list empty — it would pass even if the
release-after-close branch pooled the connection instead of closing it.

Nothing anywhere asserts that `len(sr.idle)` stays within `maxIdleConns`.
`TestConcurrentCommandsStayWithinPool` looks like the cap test but is a tautology:
it runs 8 goroutines and asserts at most 8 connections, which 8 goroutines cannot
exceed regardless of the pool's behaviour.

### Why it matters

`release` is the only thing standing between this library and unbounded resource
growth, and it is the function whose contract nobody has tested:

- If the cap check regressed, the idle list would grow without limit — every
  connection ever opened retained, each holding a socket and 8 KB of `bufio`
  buffers. That is a file-descriptor leak in a long-lived Traefik process, and the
  kind of bug that only shows up after days of production traffic.
- If the `closed` check regressed, `Close` would no longer be a real shutdown:
  connections returned by in-flight commands after `Close` would be pooled into a
  client that is supposed to be dead, leaking sockets across Traefik configuration
  reloads (which is exactly when `Close` is called).
- The documented contract on `Close` at `:64` states "In-flight commands still
  finish; their sockets are closed on release". That sentence describes precisely
  the untested branch.

This gap also blocks [perf-01](perf-01-connection-pool-cap.md): the pool is about to
be reworked to add a total cap and a wait queue, and there is currently no test that
would catch the rework getting the cap wrong.

### Expected gain

No runtime gain; it closes the correctness hole under the pool changes. Concretely:
a file-descriptor leak or a broken `Close` becomes a failing test rather than a slow
production degradation, and the vacuous assertion stops providing false confidence.

### How to fix

Three tests, none of which need timing hacks:

- **Idle cap.** Drive more concurrent commands than `maxIdleConns` against a fake
  with enough latency that callers genuinely overlap (`startSlowRedis` in
  `bench_test.go` already does this), then assert `len(redis.idle) <= maxIdleConns`
  after they all finish, and that the excess sockets were actually closed rather
  than leaked. Use a goroutine count well above the cap, unlike the current
  8-goroutine test.
- **Release after `Close`.** Start a command, `Close` the client while that command
  is in flight, let it finish, and assert its socket ends up closed and `sr.idle`
  stays empty. This requires the command to be in flight *before* `Close`, which is
  what the existing test is missing; a fake that blocks until signalled makes it
  deterministic.
- **`Close` racing `borrow`.** Cover `:236-238` by closing between the idle scan and
  the dial. A test hook or a targeted unit test on `borrow` is acceptable here;
  alternatively accept this one as inherently racy and document it.

While touching this, fix the two misleading existing tests: make
`TestConcurrentCommandsStayWithinPool` assert something 8 goroutines could actually
violate, and drop or repair the vacuous final assertion in
`TestCloseDrainsIdleAndDoesNotRepool`.

### How to prove it

Coverage of block `253.47,257.3` goes from 0 to non-zero, and each new test fails
if the corresponding guard is removed: raise `maxIdleConns` handling to always pool
and the cap test fails; drop the `sr.closed` check and the shutdown test fails.
