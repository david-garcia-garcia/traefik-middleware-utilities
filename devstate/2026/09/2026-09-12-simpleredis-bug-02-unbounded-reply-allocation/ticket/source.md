# bug-02 — A reply header allocates unbounded memory on the server's word

- **Axis**: Correctness / robustness (DoS)
- **Severity**: critical
- **Where**: `simpleredis/resp.go:122` (`readBulk`), `simpleredis/resp.go:79` (`readReply` array path)
- **Status**: not applied

## What I found

Both aggregate RESP types size their allocation directly from the reply header,
with no upper bound and before a single payload byte has arrived.

```go
// readBulk
length, err := strconv.Atoi(string(head[1:]))
if err != nil {
	return nil, errIssue
}
if length < 0 {
	return nil, errMiss
}
data := make([]byte, length+2)          // resp.go:122
if _, err = io.ReadFull(reader, data); err != nil {
	return nil, err
}
```

```go
// readReply, '*' case
count, convErr := strconv.Atoi(string(line[1:]))
if convErr != nil || count < 0 {
	return nil, false, errIssue
}
values := make([][]byte, count)         // resp.go:79
```

The only guard is `strconv.Atoi` failing, which happens solely when the number
does not fit in an `int`. Everything from `0` to `MaxInt` is honoured verbatim.

There are two damage regimes, both measured.

**Large enough to panic.** `$1000000000000000000`, `$9223372036854775807` and the
`*` equivalents all produce `runtime error: makeslice: len out of range`. That
panic then triggers [bug-01](bug-01-panic-leaks-pool-token.md) and permanently
deadlocks the pool.

**Small enough to actually allocate — the worse case.** A **12-byte** reply:

```
wire was 12 bytes: "$268435456\r\n"
result: clean=false err=EOF
heap allocated during the parse: 268447888 bytes (256.0 MiB) for a claimed 256 MiB payload
```

No panic, no stack trace, no log line. Just 256 MiB of heap from twelve bytes.

And it amplifies. The truncated read surfaces as `io.EOF`, which `ioError`
(`resp.go:153-159`) maps to `errUnreachable`, which `shouldRetry` treats as
**retryable**. So the default ladder re-issues the command three more times:
**~1 GiB per `Get`**, times up to `PoolSize` concurrent commands.

## Why it matters

This does not require a malicious Redis. Every one of these reaches the same code:

- A middleware pointed at the wrong port — an HTTPS listener, an HTTP API, a
  Prometheus exporter. Any non-RESP bytes where a `$` or `*` header is expected.
- A desynced socket, which [bug-04](bug-04-bulk-trailer-not-verified.md) shows is
  reachable, so a *later* command misreads a stale payload as a length header.
- Corruption on the wire, or a proxy/service-mesh sidecar injecting an error page.
- A compromised or hostile Redis, which for a shared cache is a real threat model.

The consequence in Traefik is an OOM kill of the whole process — not a failed
request, not a degraded middleware, the entire proxy. `resp.go` is the one place
in this library where a remote peer controls an allocation size, so it deserves
the same scepticism as parsing untrusted input, which is effectively what it is.

Note the asymmetry with the client's own writes: `writeCommand` is bounded by
whatever the caller passes, and `maxMSetEXPairs = 1024` caps the one fan-out verb.
The read path has no equivalent ceiling.

## Expected gain

Turns a remote-triggerable process kill into a failed command on a discarded
socket. Costs two integer comparisons per reply — unmeasurable against the
~18 µs compiled round trip, and it *saves* interpreted work under Yaegi by
failing before the allocation.

It also makes [bug-01](bug-01-panic-leaks-pool-token.md) far less likely to fire
in practice, and removes the retry amplification for free, since a bounded
rejection is `errIssue` (not retryable) rather than `EOF` → `errUnreachable`.

## How to fix

Add explicit ceilings and reject rather than allocate. Redis's own
`proto-max-bulk-len` defaults to 512 MB and its client-side analogue in go-redis
is a reader limit; 64 MiB is generous for this library's verbs, which return
counters, small strings and limiter tuples.

```go
const (
	maxBulkLength  = 64 << 20 // matches nothing this client legitimately reads
	maxArrayCount  = 1 << 20  // MGET fan-out is bounded far below this
)

// readBulk, after the length < 0 miss check
if length > maxBulkLength {
	return nil, errIssue
}

// readReply, '*' case
if convErr != nil || count < 0 || count > maxArrayCount {
	return nil, false, errIssue
}
```

Three points to get right:

1. **Return `errIssue`, not an IO error.** `errIssue` already marks the socket
   dirty in `do` and is not retryable, so the connection is closed and the
   command fails once. Returning something that maps to `errUnreachable` would
   preserve the 4× amplification.
2. **Make the caps `const`, not `Config` fields.** These are protocol sanity
   bounds, not tuning knobs; exposing them invites someone to set them to zero.
   Both are also well above `maxMSetEXPairs`-shaped legitimate traffic.
3. **Guard the array element path too.** The `'$'` branch inside the array loop
   calls the same `readBulk`, so it inherits the bulk cap automatically — but
   confirm with a test, because that is the path `MGET` and the limiter scripts
   actually use.

Consider also capping the *cumulative* size of an array reply, not just each
element: `maxArrayCount` elements each at `maxBulkLength` is still enormous. A
running total compared against a single reply budget is stricter and simpler to
reason about.

## How to prove it

Table-driven tests over `readReply` with a `bufio.Reader` on a `strings.Reader` —
no server needed, and it is the cheapest way to cover the whole matrix. Assert
`errIssue` and `clean == false` for: `$` and `*` just over each cap, at
`MaxInt64`, and at values that currently panic. Keep the existing
`$99999999999999999999` case, which is already safe via `Atoi` overflow, as a
regression guard.

Add an allocation assertion for the dangerous middle of the range, since that is
the case with no visible symptom. `runtime.ReadMemStats` around a parse of
`$268435456\r\n`, asserting growth stays small, is what actually pins the fix —
a plain error-equality test would pass even if the `make` still ran. `testing.AllocsPerRun`
or a `-benchmem` guard works too.

Finally, a fuzz target over `readReply` is worth adding permanently. This defect
is exactly what fuzzing finds first, and the package has no fuzz target today
(noted in [ci-01](ci-01-no-race-detector-in-ci.md)).
