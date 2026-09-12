# risk-01 — 8.1 s of un-cancellable latency per request when Redis is unreachable

- **Axis**: Availability / API design
- **Severity**: hard
- **Where**: `simpleredis/commands_exec.go:11-37` (`exec`), `simpleredis/pool.go:157-183` (`dial`), `simpleredis/config.go:5-12` (defaults)
- **Status**: not applied

## What I found

No method in the public API takes a `context.Context`. Every verb ultimately calls
`exec`, whose only bounds are the frozen `Config` timeouts, and whose retry loop
sleeps with `time.Sleep`.

Measured with stock defaults against a black-holed address (TEST-NET-3, packets
dropped, so the dial burns its full timeout):

```
defaults: PoolSize=8 PoolTimeout=200ms DialTimeout=2s IOTimeout=1s MaxRetries=0
single Get against a black hole: err=redis:unreachable elapsed=8.1092131s
retry ladder: maxRetries=3 minBackoff=8ms maxBackoff=512ms -> 4 dial attempts
```

**8.1 seconds for one `Get`.** The arithmetic: `MaxRetries: 0` is the go-redis
sentinel for "3 extra retries", so four attempts × 2 s `DialTimeout`, plus jittered
backoff between them.

And 8.1 s is not the ceiling. A borrow that dials does the TCP connect **and then**
`AUTH` and `SELECT`, each a full `do` with its own fresh `IOTimeout` deadline:

```go
// dial, pool.go:157-183
netConn, err := dialer.Dial("tcp", sr.host)   // up to DialTimeout
// ...
if sr.pass != "" {
	if _, _, err = sr.do(conn, [][]byte{[]byte("AUTH"), []byte(sr.pass)}); err != nil {
```

So one attempt costs up to `DialTimeout + 2 × IOTimeout` = **4 s** on a
passworded, database-selected server that accepts the connection then stalls —
about **16 s** across the ladder. `PoolTimeout` (200 ms) does not bound any of
this; it only bounds waiting for a *turn*, not what happens after one is acquired.

Two mitigations already in place, worth crediting: `shouldRetry` deliberately does
**not** retry timeouts (`isCommandTimeout`), and `errPoolWait` is a distinct error
value from `errUnreachable` specifically so pool waits are not multiplied by
`MaxRetries`. I verified both hold. The exposure is dial failures, which *are*
retried, and correctly so for a stale pooled socket.

## Why it matters

This is a Traefik middleware on the request path. 8 s is far past every relevant
budget: client patience, upstream read timeouts, health-check intervals, and
whatever SLO the route has.

The absence of `context` makes it worse than a slow dependency:

- **A cancelled request keeps working.** The client hung up 7 s ago; the goroutine
  is still sleeping in a backoff before its fourth dial attempt.
- **No caller-side deadline is possible.** A middleware cannot say "I'll give
  Redis 50 ms" without spawning a goroutine and abandoning it, which leaks a
  goroutine and a pool token per request under sustained failure.
- **No circuit breaker, no failure memory.** Every concurrent request independently
  rediscovers that Redis is down, at full cost. With `PoolSize: 8` and 4 dial
  attempts each, a burst of 500 requests all queue behind 8 turns while each
  turn-holder spends seconds dialling — so `PoolTimeout` failures pile up behind
  the slow path, and the effective behaviour is a latency collapse rather than
  fast failure.
- **Goroutine pile-up.** Traefik holds a goroutine and its buffers per in-flight
  request. Requests that would have completed in microseconds now occupy memory
  for seconds, which is how a Redis outage becomes a Traefik memory problem.

The backoff `time.Sleep` is at least well placed: it happens after `release` and
before the next `borrow`, so a sleeping retry holds no pool token. Verified.

## Expected gain

Bounded, cancellable Redis latency. With `context` support plus sane defaults, a
request pays what the middleware allows it to pay — tens of milliseconds — instead
of seconds, and abandons the work when the client disconnects.

A circuit breaker turns the *N*th concurrent request during an outage from "4 dial
attempts" into "immediate `redis:unreachable`", which is what stops an outage
propagating into the proxy.

## How to fix

Three changes, in increasing order of effort. The first two are worth doing
regardless.

**1. Fix the defaults.** `DialTimeout: 2s` and `MaxRetries` defaulting to 3 are
go-redis values chosen for application clients, not for a proxy hot path. For a
Redis on the same network, `DialTimeout: 200ms`, `IOTimeout: 100ms`,
`MaxRetries: 1` are defensible, and worst case drops from ~8 s to well under a
second. Document the total worst case next to the knobs, because right now nothing
tells the operator that four fields multiply.

**2. Bound a whole `exec`, not each step.** Add an overall deadline computed once
at entry and enforced across attempts, so the ladder cannot exceed it:

```go
deadline := time.Now().Add(sr.commandTimeout)  // new, or derived from existing knobs
for attempt := 0; attempt <= maxRetries; attempt++ {
	if time.Now().After(deadline) {
		return nil, last
	}
	// ...
}
```

Also pass the remaining budget into `dial` so `AUTH`/`SELECT` cannot each add a
fresh `IOTimeout` on top.

**3. Accept a `context.Context`.** The Yaegi constraint is real but not blocking:
`context` is in Yaegi's stdlib, and the mechanism needed is
`conn.netConn.SetDeadline` (already used) plus a watcher that closes the socket on
`ctx.Done()`. Avoid interface type assertions in that path —
`resp.go:154` documents that Yaegi has panicked on a `net.Error` assert, so use
`errors.Is` as the existing code does. Keep the current signatures as
`ctx.Background()` wrappers so consumers migrate incrementally:

```go
func (sr *SimpleRedis) Get(name string) ([]byte, error) {
	return sr.GetContext(context.Background(), name)
}
```

A **circuit breaker** is the highest-leverage addition beyond this: after *k*
consecutive dial failures, fail immediately for a cooldown. It is also the cheapest
to make Yaegi-safe — an `atomic.Int64` of the last failure time and a counter, no
new imports. `atomic.Bool` as a struct field is already proven to work interpreted.

## How to prove it

Assert the worst case rather than logging it. A black-holed address
(`203.0.113.1:6379`, TEST-NET-3) plus a stopwatch, asserting one `Get` returns
within a stated budget. That single test would have made the 8.1 s visible from
day one, and it fails loudly if someone raises a default later.

For the AUTH/SELECT amplification, a fake that completes the TCP handshake and
then never replies, with `Pass` and `Database` set, asserting the elapsed time is
bounded by the overall budget rather than by `DialTimeout + 2 × IOTimeout`.

For cancellation, once `context` lands: cancel mid-command against a slow fake and
assert the call returns promptly, the token is returned to the semaphore, and the
socket was closed rather than pooled.

For the breaker: *N* concurrent callers against a dead address, asserting total
dials stays near the threshold instead of `N × (MaxRetries+1)`, and that latency
per call collapses after the breaker opens.
