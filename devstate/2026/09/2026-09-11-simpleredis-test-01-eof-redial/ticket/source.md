# test-01 — The realistic stale-connection case (server closed, `io.EOF`) is untested

Host: local
IssueKey: 2026-09-11-simpleredis-test-01-eof-redial
Finding: simpleredisfixes/test-01-server-closed-eof-redial.md
Index: simpleredisfixes/README.md

## HARD REQUIREMENT (conductor)

- Test coverage MUST run against both Redis and Dragonfly. Both are supported backends.
- The finding's fake that closes the SOCKET FROM THE SERVER SIDE is required (do not close the client).
- ALSO prove a live idle/server-close recovery path against both Redis and Dragonfly (compose services; do not skip Dragonfly).
- Extend Pester `/redis` `/dragonfly` and `e2e/simpleredisprobe` if that is dest's live harness.
- Lua 5.1-safe. Dragonfly KEYS required.

## Index context (simpleredisfixes/README.md)

Review of `simpleredis/` for hot-path efficiency and test coverage. One file per finding. Nothing here is applied to `simpleredis/simpleredis.go` yet.

| # | Axis | Severity | Finding |
|---|---|---|---|
| [test-01](test-01-server-closed-eof-redial.md) | Coverage | hard | The realistic stale-connection case (server closed, `io.EOF`) is untested |

Suggested order places **test-01 through test-05** after perf-01/perf-02: these are the failure modes perf-01 exposes.

Statement coverage was 86.1% before this review.

## Finding (simpleredisfixes/test-01-server-closed-eof-redial.md)

- **Axis**: Test coverage
- **Severity**: hard
- **Where**: `simpleredis/simpleredis.go:431-437` (`ioError`), `:182-201` (`exec` retry), `:292-307` (`do`)
- **Status**: not applied

### What I found

`ioError`'s `errUnreachable` return has **zero coverage**:

```go
func ioError(err error) error {
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return errTimeout
	}
	return errUnreachable   // simpleredis.go:436 — never executed by any test
}
```

The measured coverage profile also shows the `writeCommand` failure branch in `do`
(`:296-298`) and every internal error return in `writeCommand` (`:311-323`) never
executing.

`TestStaleConnectionIsRetried` looks like it covers this, but it does not. It closes
the pooled socket from the **client** side:

```go
redis.mu.Lock()
for _, conn := range redis.idle {
	conn.close()
}
redis.mu.Unlock()
```

A locally closed connection fails at the very first step of `do` —
`conn.netConn.SetDeadline(...)` returns `os.ErrClosed` — and takes the
`return nil, false, errUnreachable` path at `:294`, never reaching the write or the
read. So the test proves the retry *mechanism* works when `SetDeadline` fails, and
nothing about the path production actually takes.

What production takes is different. When **Redis** closes the connection — its own
`timeout` setting expiring an idle client, a restart, a failover, a proxy or load
balancer reaping the socket — the client side is still open. `SetDeadline` succeeds,
the write into the socket succeeds (a received FIN does not block writes), and the
failure surfaces on the **read** as `io.EOF`, which reaches `readReply` →
`readLine` → `ReadBytes` → `io.EOF`, returns `clean == false`, and only then maps
through `ioError` to `redis:unreachable` and triggers the retry.

### Why it matters

This is the single most common failure this library will encounter in production.
Redis ships with idle client timeouts, managed Redis services enforce them
aggressively, and failovers happen. Every one of those events lands on this exact
path, and no test exercises it.

The consequences of it being wrong are not subtle:

- If the retry does not fire, ordinary cache reads fail with `redis:unreachable`
  after every idle period, and the middleware sees intermittent errors that
  reproduce only after minutes of quiet — the worst kind of bug to diagnose.
- If `clean` is mishandled, a connection that has seen EOF goes back into the idle
  pool and poisons the next borrower.
- [perf-03](perf-03-idle-reaper-tail-only.md) makes this path *more* likely, because
  cold connections at the head of the idle list are the ones most likely to have
  been closed server-side, and they are only ever borrowed during a burst.

Note also that the retry costs the affected request a full extra dial, so this path
is a latency spike as well as a correctness question.

### Expected gain

No runtime gain — this is proof of the library's most-used recovery path. What it
buys is that a regression in the retry logic, in the `clean`/`reusable` bookkeeping,
or in the `ioError` mapping fails a test instead of failing intermittently in
production after an idle period.

It also makes the current 86.1% statement coverage honest: `ioError`'s main branch
counting as covered by nothing is the clearest example of coverage percentage
hiding a gap that matters.

### How to fix

Add a fake server that reproduces server-side closure:

- Accept a connection, answer the first command normally, then **close the socket
  without reading further**. Optionally accept a second connection and serve it
  normally.
- Assert the first `Get` succeeds, then assert the second `Get` also succeeds,
  served on a freshly dialled connection, and that the fake saw exactly 2 accepts.
- Assert the dead connection did not go back into `sr.idle`.
- Add a variant where **no** second connection is available (listener closed), so
  the retry's own `borrow` fails — that covers `:195-197`, also currently
  uncovered — and assert the error is `redis:unreachable`.

Do not simulate this by closing the client side; that is what the existing test
already does and it short-circuits at `SetDeadline`. The socket must be closed by
the peer.

A useful sharpening of the existing test: since `TestStaleConnectionIsRetried`
actually covers the `SetDeadline` arm, rename it to say so, so the two distinct
paths are visibly distinct.

### How to prove it

Coverage of `simpleredis.go:436` goes from 0 to non-zero, and the new test fails if
the retry condition at `:190` is inverted or if `release` is made to pool a
non-reusable connection.
