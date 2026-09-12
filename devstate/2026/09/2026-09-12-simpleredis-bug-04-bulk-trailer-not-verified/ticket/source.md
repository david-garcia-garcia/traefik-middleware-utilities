# bug-04 — `readBulk` never verifies the bulk trailer, so a reply can be declared clean while desynced

- **Axis**: Correctness (protocol framing)
- **Severity**: hard
- **Where**: `simpleredis/resp.go:122-126` (`readBulk`), `simpleredis/resp.go:14-29` (`do`, the `reusable` contract)
- **Status**: not applied

## What I found

`readBulk` reads the payload **plus two bytes**, then returns a slice that drops
those two bytes — asserting by construction that they were CRLF, without ever
checking:

```go
data := make([]byte, length+2)
if _, err = io.ReadFull(reader, data); err != nil {
	return nil, err
}
return data[:length], nil
```

When the trailer is not CRLF, those two bytes belong to whatever came next. The
parse still succeeds, and `do` reports the stream as reusable, so the socket goes
back into `idleConns` carrying a partial reply.

Measured directly, with a reply missing its trailer followed by the next reply:

```
wire="$5\r\nhello+OK\r\n" -> values=["hello"] clean=true err=<nil>
CONFIRMED: reply accepted as clean; "K\r\n" left on a socket returned to the pool (desync)
```

`"hello"` was returned as a perfectly good value, `+O` was silently eaten as the
trailer, and `"K\r\n"` was left on a pooled socket for an unrelated future command
to misread.

This is the only place in the parser where `clean == true` can coexist with
leftover bytes. I checked every other `clean == true` return — `+`, `:`, `-`, a
complete `$`, `$-1` misses, and the array path including the `errMiss` `continue`
— and all of them consume exactly one reply. Everything the parser cannot
represent (`*-1`, nested arrays, RESP3 types) is correctly marked dirty; see
[risk-04](risk-04-non-basic-resp2-replies-rejected.md).

## Why it matters

Response desync is the worst failure mode a pooled client can have, because it
breaks the one invariant everything above it assumes: that the bytes you read
answer the command you sent. Once a socket is poisoned, a later request gets a
**different request's data**. For a cache that is a wrong-value-for-key bug; for
the rate limiters it is a counter read from the wrong window.

It is also the failure mode with no useful diagnostics. The command that caused
the desync succeeds. The command that suffers it fails — or worse, succeeds with
the wrong value — arbitrarily later, on a different key, in a different
middleware, with a healthy Redis and clean logs.

Reaching it requires a server or path that mis-frames a bulk reply, which real
Redis will not do. What makes it worth fixing anyway:

- It is the cheapest guard in the parser: two byte comparisons on data already in
  hand, on a path that is already touching those bytes.
- It closes a feedback loop with [bug-02](bug-02-unbounded-reply-allocation.md). A
  desynced socket means a later command reads arbitrary payload bytes as a length
  header, which is precisely how the unbounded allocation gets triggered without
  a hostile server.
- The failure is asymmetric: the cost of the check is two comparisons, the cost of
  omitting it is undebuggable cross-request data corruption.
- It hardens the client against non-Redis peers, which is the realistic trigger
  (wrong port, proxy error page, TLS handshake bytes).

## Expected gain

No throughput change — the bytes are already read and already in the buffer. What
it buys is a hard guarantee that a socket returned to `idleConns` is positioned
exactly at a reply boundary, which is the property the whole pool depends on and
currently only assumes.

## How to fix

Verify the trailer and mark the stream dirty when it is wrong:

```go
data := make([]byte, length+2)
if _, err = io.ReadFull(reader, data); err != nil {
	return nil, err
}
if data[length] != '\r' || data[length+1] != '\n' {
	return nil, errIssue
}
return data[:length], nil
```

`errIssue` is the correct signal: `do` already maps it to `reusable == false`, so
the socket is closed rather than pooled, and it is not retryable, so the command
fails once instead of amplifying.

Note the ordering constraint with [bug-02](bug-02-unbounded-reply-allocation.md):
the length cap must be applied **before** the `make`, whereas this check
necessarily comes after the read. Both edits land in the same short function, so
do them together.

If `../simpleredisfixes/perf-07-readslice-decoding.md` (switching `readLine` to
`ReadSlice`) is adopted, keep this check — a zero-copy read path makes trailer
verification *more* important, not less, because a mis-framed slice would then
alias buffer contents that get reused.

## How to prove it

Two tests, and the second is the one that matters.

**Unit:** feed `readBulk` a payload whose trailer is not CRLF and assert
`errIssue` with `clean == false`. Cover a trailer that is `\n\r`, one that is two
payload bytes, and a zero-length bulk `$0\r\n\r\n` as the regression guard that
the check does not reject legitimate empty values.

**Desync (the money shot):** a fake server that answers the first command with a
trailer-less bulk and the second command normally, on a client with `PoolSize: 1`
so reuse is forced. Assert the second command does **not** return the first
reply's remnants — and assert the fake saw two `Accept`s, proving the poisoned
socket was discarded rather than pooled. Without the accept assertion the test
passes for the wrong reason.

The existing `startStaticRedis` helper in `simpleredis/fake_redis_test.go` is
close to what the first case needs; the desync case needs a per-command scripted
server, which is also what
`../simpleredisfixes/test-04-truncated-bulk-payload.md` calls for. Build one
harness for both.
