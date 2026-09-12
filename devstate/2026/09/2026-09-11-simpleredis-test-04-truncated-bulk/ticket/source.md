# test-04 — Truncated bulk payload, the anti-corruption guard, is untested

Source: local caller spec. Finding file `simpleredisfixes/test-04-truncated-bulk-payload.md`. Index `simpleredisfixes/README.md`. Conductor extras included as asked.

- **Axis**: Test coverage
- **Severity**: hard
- **Where**: `simpleredis/simpleredis.go:389-405` (`readBulk`), `:299-306` (`do`'s `clean` handling)
- **Status**: not applied

## What I found

The short-read branch in `readBulk` has **zero coverage** (measured block
`401.52,403.3`):

```go
data := make([]byte, length+2)
if _, err = io.ReadFull(reader, data); err != nil {
	return nil, err        // never executed by any test
}
```

That error propagates to `readReply`, which returns `clean == false`, and `do` then
refuses to reuse the connection:

```go
values, clean, err := readReply(conn.reader)
if err != nil && !clean {
	if err == errIssue {
		return nil, false, errIssue
	}
	return nil, false, ioError(err)
}
```

The `reusable == false` return is what makes `release` destroy the socket. Nothing
tests that chain. The neighbouring malformed-header branches in `readBulk`
(`:390-392` for a non-`$` head, `:394-396` for an unparseable length) are equally
uncovered.

## Why it matters

This is the highest-consequence untested line in the file, because it is the guard
against **serving one request's data to another request**.

Consider a server that announces `$100\r\n` and then dies after 40 bytes — a Redis
crash mid-reply, a failover cutting the stream, a proxy truncating. The reader has
consumed a partial payload and the stream is now misaligned: whatever arrives next
will be interpreted as the start of a reply. If that connection went back into the
idle pool, the **next** request to borrow it would read the tail of the previous
request's value as its own reply. In a Traefik middleware that means one tenant's
cached value, session token, or rate-limit verdict served to another tenant. It is a
cross-request data leak, not merely a wrong answer.

The `clean` flag is the mechanism that prevents it, and `clean` is exactly what has
no test. Everything about it is currently load-bearing on code review alone:

- If `readBulk` were changed to return `clean == true` on a short read, the leak
  described above becomes reachable and every existing test still passes.
- If `do`'s `err != nil && !clean` condition were loosened, the same.
- The distinction between "protocol garbage, connection unusable" and "legitimate
  miss, connection fine" (`errMiss` at `:397-399`, which *is* covered) is subtle and
  sits three lines away from the untested branch.

A secondary concern: because `exec` retries a failed reused connection, a truncated
reply on a non-idempotent command re-sends it — see
[test-05](test-05-non-idempotent-retry.md).

## Expected gain

No runtime gain. It converts a silent cross-request data-leak risk from
"prevented by an untested branch" into "prevented by an asserted invariant". Given
the consequence, this is the coverage item with the worst downside if left alone,
even though it is less likely to be hit than [test-01](test-01-server-closed-eof-redial.md).

## How to fix

A fake server that lies about its payload length is enough:

- **Truncated payload.** Announce `$100\r\n`, write 40 bytes, close. Assert the
  command returns an error (`redis:unreachable` via `ioError`, since a short read is
  an I/O failure, not `errIssue`), and — the actual point — assert the connection was
  **not** pooled: `len(redis.idle) == 0`.
- **The leak itself.** The strongest version of this test proves the invariant rather
  than the implementation: after a truncated reply, issue a second command and assert
  it returns the *correct* value for its own key. If the poisoned connection were
  reused, this fails. That test would survive a refactor of the `clean` plumbing,
  which is what makes it worth writing.
- **Malformed bulk header.** A reply of `$abc\r\n` and one where an array element
  head is neither `$`, `:` nor `+`, asserting `redis:issue?` and again that nothing
  was pooled. This covers `:390-396` alongside.

`startStaticRedis` cannot express these because it replays one canned reply per
command; a fake that can write raw bytes and close mid-stream is needed.

## How to prove it

Coverage of block `401.52,403.3` goes from 0 to non-zero. The invariant test fails
if `readBulk` is made to return `clean == true` on a short read, or if `release` is
made to pool a connection whose `reusable` is false.

## Index (simpleredisfixes/README.md)

Review of `simpleredis/` for hot-path efficiency and test coverage. Finding
[test-04](test-04-truncated-bulk-payload.md) is Coverage / hard: Truncated bulk
payload — the anti-corruption guard — is untested. Suggested order places
test-01 through test-05 after perf-01 / perf-02.

## Conductor extras (part of the ask)

- Tests MUST run against both Redis and Dragonfly. Both are supported backends.
- Truncated-payload / clean=false must be unit-tested as specified in the finding.
- Live Get/MGet on both engines must still return the caller's own value (no cross-request leak).
- Extend compose + Pester `/redis` `/dragonfly`.
- Lua 5.1-safe. Dragonfly KEYS required.
