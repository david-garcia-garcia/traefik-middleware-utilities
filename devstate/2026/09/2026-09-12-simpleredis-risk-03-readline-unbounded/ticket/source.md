# risk-03 — `readLine` has no length limit

- **Axis**: Robustness (DoS)
- **Severity**: judgement
- **Where**: `simpleredis/resp.go:130-139` (`readLine`)
- **Status**: not applied

## What I found

`readLine` reads until a newline with no ceiling:

```go
func readLine(reader *bufio.Reader) ([]byte, error) {
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	if len(line) < 2 || line[len(line)-2] != '\r' {
		return nil, errIssue
	}
	return line[:len(line)-2], nil
}
```

`bufio.Reader.ReadBytes` grows an internal buffer until it finds the delimiter or
the underlying reader errors. Measured with an 8 MiB simple-status line:

```
8 MiB single line: len(values[0])=8388608 clean=true err=<nil>
```

Accepted without complaint. A peer that sends bytes with no `\n` grows the buffer
until the process runs out of memory — bounded only by `IOTimeout`, which caps
*time*, not bytes. At a modest 100 MB/s over loopback or a fast LAN, a 1 s default
`IOTimeout` is ~100 MB per socket, times `PoolSize`.

`readLine` is called for every reply header and every array element header, so it
is on every command path.

## Why it matters

Same threat model as [bug-02](bug-02-unbounded-reply-allocation.md) — a non-RESP
peer, a wrong port, a corrupted stream, or a hostile Redis — and the same outcome,
an OOM kill of the Traefik process rather than a failed request.

It is rated judgement rather than critical because it is strictly harder to
exploit than bug-02: this requires sustaining a byte stream for the full
`IOTimeout`, whereas bug-02 needs twelve bytes. Fix bug-02 first; this is the
same hardening applied to the one remaining unbounded read.

Worth noting what it protects against concretely: pointing a middleware at an
HTTPS port. A TLS `ServerHello` is binary with no reliable early `\n`, so the first
`readLine` consumes whatever the peer streams. `IOTimeout` eventually fires, but
only after the buffer has grown.

## Expected gain

Bounds the read path's memory to a stated maximum per socket, regardless of what
the peer sends. No cost on legitimate traffic: real reply headers are well under
64 bytes, and the check is one comparison on a length already computed.

## How to fix

Every line this client legitimately reads is tiny — a status like `+OK`, an
integer, an error message, or a `$`/`*` header. A few kilobytes is generous:

```go
const maxLineLength = 64 << 10

func readLine(reader *bufio.Reader) ([]byte, error) {
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	if len(line) > maxLineLength {
		return nil, errIssue
	}
	// ...
}
```

This caps the *result* but the buffer has already grown by then, so it is only
half a fix. To bound the allocation itself, read incrementally and stop early:

```go
line, err := reader.ReadSlice('\n')
if err == bufio.ErrBufferFull {
	return nil, errIssue          // header longer than the bufio buffer: not RESP
}
```

`ReadSlice` returns `ErrBufferFull` once the delimiter is not found within the
existing buffer, so nothing grows. It returns a slice aliasing the `bufio` buffer,
which is valid until the next read — fine here, since `readLine`'s callers either
copy (`line[1:]` into a returned value) or parse immediately. Confirm each caller
before switching, and note this is the same change proposed for performance
reasons in `../simpleredisfixes/perf-07-readslice-decoding.md`, so the two should
land together.

Use `errIssue`, which already marks the socket dirty in `do` and is not retryable,
so a bad peer costs one failed command rather than four.

## How to prove it

Feed `readReply` a long line with no CRLF and assert `errIssue` with
`clean == false`. Two cases matter: a line just over the cap that *does* terminate,
and a stream that never terminates.

The second needs an allocation assertion, not just an error check — otherwise a
fix that caps the returned length while still growing the buffer passes.
`runtime.ReadMemStats` around the parse of a multi-megabyte unterminated stream,
asserting growth stays near the `bufio` buffer size, is what pins it.

Keep a regression case for the shortest legal lines (`+OK`, `:1`, `$-1`, and a
1-byte line that must still be rejected by the existing `len(line) < 2` guard) so
the cap does not break normal replies.
