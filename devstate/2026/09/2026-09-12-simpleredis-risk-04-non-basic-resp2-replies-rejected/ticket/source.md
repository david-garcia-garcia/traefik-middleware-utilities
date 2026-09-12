# risk-04 — Any reply outside basic RESP2 fails the command and discards the socket

- **Axis**: Robustness / protocol coverage
- **Severity**: judgement
- **Where**: `simpleredis/resp.go:51-107` (`readReply`)
- **Status**: not applied

## What I found

`readReply` handles `+`, `:`, `-`, `$` and a **flat** `*` whose elements are `$`,
`:` or `+`. Everything else falls to a `default` that returns `errIssue` with
`clean == false`. Measured across the shapes a real engine can produce:

| Reply | Where it comes from | Result |
|---|---|---|
| `*-1` | RESP2 null array | `redis:issue?`, socket discarded |
| `*1\r\n*1\r\n$1\r\na\r\n` | Lua returning a nested table | `redis:issue?`, socket discarded |
| `*1\r\n-ERR nope\r\n` | Lua returning `{err=...}` | `redis:issue?`, socket discarded |
| `_`, `#`, `,`, `(`, `%`, `~`, `=`, `>` | RESP3 | `redis:issue?`, socket discarded |

The good news, and the reason this is judgement rather than hard: **every one of
these is marked dirty**, so the socket is closed instead of being pooled with
unread bytes on it. There is no desync here. I checked the whole `clean == true`
set separately and the only framing hole is
[bug-04](bug-04-bulk-trailer-not-verified.md).

Two consequences follow from the failures being `errIssue`:

- `errIssue` is **not retryable** (`shouldRetry`), so the command fails once.
  Correct — retrying would not change the reply shape.
- Each occurrence closes a socket and forces a redial, so a script that returns an
  unsupported shape does not just fail, it churns the pool on every call.

A flat array of bulk strings *is* supported, which is why the existing consumer
works: `tokenbucket/lua.go:50` returns
`{tostring(true), tostring(wait_duration), tostring(tokens)}`. Verified — a flat
`{1, 59}` parses fine, `{1, {2}}` does not.

## Why it matters

The exposure is `Eval`, and it is a live constraint on the roadmap rather than a
latent bug. Returning a table is the normal way a Lua limiter reports several
values at once, and the natural formulations break:

- `return {allowed, remaining, reset_at}` where the elements are Lua **numbers** is
  fine (numbers become `:` integers), but any nested table is not.
- `return {1, {2, 3}}`, or a table whose elements are themselves tables, is not.
- `return redis.error_reply(...)` inside a table is not.
- `return cjson.decode(...)` results are not, in general.

Today the only thing keeping this working is a convention: the existing script
wraps every element in `tostring`. That is a real technique, but it is undocumented
as a **requirement** and unenforced, so the next script author has no way to know
that a nested table silently costs a socket and a failed request. Two more
limiters are planned on this client
(`handoff-kong-window-limiter.md`, `handoff-leaky-bucket.md`).

The RESP3 rows are lower risk: this client never sends `HELLO`, so a Redis 6+
server stays in RESP2 for the session. They would matter if `HELLO`, client-side
caching, or pub/sub were ever added — and a push message arriving on a pooled
socket would then be read as a reply to the wrong command.

`*-1` is the one that could appear without anyone changing scripts: it is what
blocking commands return on timeout and what an aborted `EXEC` returns. None of
the current verbs produce it, so it is defensive only.

## Expected gain

No performance change. What it buys is a stated contract — either the parser
supports nested aggregates, or the constraint is documented and enforced at the
point where a script author would trip on it. Both are better than the current
state, where the limitation is discovered as an intermittent `redis:issue?` plus
unexplained connection churn.

## How to fix

Pick a scope deliberately; do not do all of it.

**Minimum (recommended): document and detect.** Add the constraint to `Eval`'s doc
comment — flat arrays of bulk strings or integers only, wrap Lua results in
`tostring` — and make the failure legible. A distinct sentinel such as
`redis:unsupported-reply` (or wrapping `errIssue` with the offending type byte)
turns "redis:issue?" into something an operator can act on. Today the same error
covers malformed framing, unparseable integers, and unsupported shapes.

**Cheap correctness win: handle `*-1` as a miss.** One case in the `'*'` branch,
returning `errMiss` with `clean == true`, since a null array is a complete,
well-framed reply and the socket is perfectly reusable. Currently it needlessly
destroys a connection.

**Larger: support nested arrays.** Recurse in the array element loop instead of
switching on a fixed set of leading bytes. The blocker is that the return type is
`[][]byte`, which cannot represent a tree — so this needs a shape change
(`type Reply struct { … }` or a flattening contract), which ripples into every
caller and into `parseIntegerReply`. Only worth it if a planned script genuinely
needs it; otherwise the `tostring` convention is cheaper and already proven.

If RESP3 ever becomes relevant, the parser must reject `>` push frames explicitly
rather than by falling through to `default`, because a push frame can arrive
*interleaved* with a reply, and closing the socket is the correct response only if
the client is not expecting more data.

## How to prove it

Table-driven tests over `readReply` for every row above, asserting the specific
error and — importantly — `clean == false`, so the no-desync property is pinned
rather than assumed. That table is the durable artefact: it documents the supported
surface in executable form.

Add the pool consequence as a separate assertion, because the error alone
understates the cost: issue an `Eval` returning a nested array against a fake and
assert the server saw the socket close and a redial on the next command. That is
what makes the churn visible.

If `*-1` is converted to a miss, assert `clean == true` and that the following
command on the **same** socket succeeds — the accept count on the fake proving no
redial happened.

Finally, a guard test for the convention: assert that the exact reply shape
`tokenbucket`'s script produces (three bulk strings) parses correctly. That pins
the one shape production depends on, which no current test isolates.
