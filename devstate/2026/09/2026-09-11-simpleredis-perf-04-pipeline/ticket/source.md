# perf-04 — No pipelining: one round trip per command

Caller spec (`issueHost: local`). Finding file: `simpleredisfixes/perf-04-pipelining.md`. Index: `simpleredisfixes/README.md`. IssueKey: `2026-09-11-simpleredis-perf-04-pipeline`. DestBranch: `master`.

## Finding (verbatim)

- **Axis**: Performance
- **Severity**: judgement
- **Where**: `simpleredis/simpleredis.go:182-201` (`exec`), `:292-307` (`do`)
- **Status**: not applied

### What I found

Every verb is exactly one command and one round trip. `exec` borrows a connection,
calls `do` once, and releases:

```go
values, reusable, err := sr.do(conn, args)
sr.release(conn, reusable)
```

`do` writes one frame, flushes, and reads one reply. There is no way to put several
commands on the wire before reading the first reply. `MGet` is the only batching
primitive, and it only batches one command shape.

### Why it matters

The round trip is the dominant cost in the entire system: ~17,500 ns against the
loopback fake server, and typically 200,000–1,000,000 ns against a real Redis over
a network. Every other cost in this library is noise beside it — client-side encode
plus decode for a `Get` is 124 ns, about 0.7% of a loopback round trip and under
0.05% of a realistic networked one.

The planned consumers need multi-command batches:

- `handoff-leaky-bucket.md` and `handoff-traefik-token-limiter.md` both adopt Kong's
  `sync_rate` pattern: a timer flushes accumulated local deltas to Redis. A flush
  covering N distinct keys is N sequential round trips today. At 200 tracked keys
  and 0.5 ms per round trip, one flush takes 100 ms of wall time and holds a
  pooled connection the whole way.
- Any caller wanting a counter plus its TTL, or several unrelated keys of different
  shapes, pays a round trip each. `Eval` is the current escape hatch and it works,
  but it forces callers to express batching as Lua.

### Expected gain

For an N-command batch, **N round trips collapse to 1**. Concretely, for a 200-key
flush at 0.5 ms per round trip: ~100 ms becomes ~1 ms plus the encode cost, which
is roughly 200 × 77 ns ≈ 15 µs compiled. That is a 50–100x improvement on the flush
path, and it holds the pooled connection for a correspondingly shorter time, which
directly reduces the concurrency that drives [perf-01](perf-01-connection-pool-cap.md).

This is the largest single latency win available in the library, and it needs no
`unsafe` and no configuration flag.

### How to fix

Add a batch entry point alongside `exec`, reusing everything that exists:

- `ExecPipeline(commands [][][]byte) ([][][]byte, error)` (or a small `Pipeline`
  type that accumulates and then `Exec`s). Borrow one connection, write all frames
  into the writer, flush **once**, then read exactly N replies in order.
- Reuse `writeCommand` per frame and `readReply` per reply; the existing single
  `SetDeadline` per `do` becomes one deadline for the whole batch, which is the
  right semantics — bound the batch, not each element.
- Error policy needs a decision: a `-ERR` on element 3 of 10 must not abandon the
  remaining replies, or the connection is left dirty and must be destroyed. Read
  all N replies, then return the per-element errors. Only a genuine I/O or protocol
  error marks the connection unusable.
- Cap the batch size so a caller cannot build an unbounded frame in memory, and
  document the cap.
- Interaction with [test-05](test-05-non-idempotent-retry.md): a pipeline must not
  be retried wholesale after a partial read, for the same double-apply reason.

### How to prove it

A fake server that counts round trips (frames read before each reply written) and
asserts N commands produced one flush and N ordered replies. A test where element
3 returns `-ERR` asserting the other 9 replies still come back and the connection
is still reusable. A test that a mid-pipeline truncation destroys the connection
rather than leaving unread replies for the next borrower.

## HARD REQUIREMENT (caller spec)

tests MUST run against both Redis and Dragonfly. Both are supported backends. A pipeline of mixed verbs MUST be proven live on both engines (not only a fake that counts flushes). Extend compose + Pester `/redis` `/dragonfly` and `e2e/simpleredisprobe`. Lua 5.1-safe. Dragonfly KEYS required. Do not retry a pipeline wholesale after a partial read.

This is caller spec for Desired/Affected, not an extra product ask.

## Index (from `simpleredisfixes/README.md`)

perf-04 — Performance, judgement — No pipelining: one round trip per command. Suggested order groups perf-05 / perf-04 / feat-01 together; this ticket is only perf-04.
