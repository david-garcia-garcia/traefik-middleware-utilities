# feat-01 — Support `MSETEX`: many keys, one TTL, one round trip

- **Axis**: Feature
- **Severity**: judgement (enabler for the planned limiters)
- **Where**: new verb in `simpleredis/simpleredis.go` alongside `MGet` (`:99-117`) and `Set` (`:119-123`)
- **Status**: not applied
- **Upstream**: [valkey-io/valkey#2592](https://github.com/valkey-io/valkey/issues/2592) (accepted, `major-decision-approved`), shipped as [`MSETEX`](https://upstash.com/docs/redis/commands/string/msetex)

## What I found

The library can read many keys in one round trip (`MGet`) but can only *write* one
key at a time. Setting N keys with a TTL is N sequential round trips of
`SET k v EX n`, since `Set` (`:119-123`) takes a single name.

`MSETEX` closes that gap in one atomic command:

```
MSETEX <numkeys> <key> <value> [<key> <value> ...]
  [NX | XX]
  [EX <seconds> | PX <milliseconds> | EXAT <unix-time-seconds> |
   PXAT <unix-time-milliseconds> | KEEPTTL]
```

It replies with an integer: `1` if the keys were written, `0` if an `NX`/`XX`
condition prevented it. The leading `<numkeys>` is what makes the argument list
unambiguous — it separates the pairs from the trailing options.

**The engine support picture is the important part.** This is a Valkey addition, not
something every engine has:

| Engine | `MSETEX` |
|---|---|
| Valkey (versions shipping #2592) | native |
| Redis | not available — plain `MSET` has no expiration option |
| Dragonfly | not available |

The repository targets **both Redis and Dragonfly** — every handoff requires live
tests against both, table-driven by address — so a Lua fallback is not an
edge case for one engine. It is the **default path**, and the native command is the
optimisation available on some deployments.

Note the pre-`MSETEX` proposal in the issue was `MSET … EX <seconds>`, and it is worth
being glad it did not ship that way: `MSET k1 v1 EX 60` on an engine without support
is still an *even* argument count, so Redis would happily store a key literally named
`EX` with value `60` — silent data corruption instead of an error. Because `MSETEX` is
a distinct verb, an unsupporting engine returns
`ERR unknown command 'MSETEX'`, which is safe to detect. That difference is what makes
runtime capability detection viable at all.

## Why it matters

The flush paths in `handoff-leaky-bucket.md` and `handoff-traefik-token-limiter.md`
are exactly this shape: a timer wakes up and writes many tracked keys, all with the
same expiry, and all of them must land together or the limiter's state is
inconsistent. Today that is N round trips, and the obvious workarounds are each
wrong in a specific way:

- **`MSET` then N `EXPIRE`** is not atomic. Between the two, the keys exist with no
  lifetime. If the process dies or the connection drops in that window, they persist
  forever — a slow memory leak in Redis keyed by whatever the limiter tracks
  (client IP, token, route), which is unbounded input.
- **N × `SET k v EX n`** is atomic per key but not as a group, and costs N round
  trips.
- **`MULTI`/`EXEC`** would give atomicity but this library has no transaction
  support, and adding one is a bigger surface than a single verb.

So `MSETEX`, or its Lua equivalent, is the only way to get "these keys are only
meaningful together, and none of them may outlive the window" in one step.

Two behaviours to respect, both from the command's documented semantics:

- **`EXAT`/`PXAT` in the past deletes the keys and still replies `1`.** The Kong-style
  flush pattern the handoffs adopt uses `expireat`, so a clock skew or a stale
  computed deadline silently deletes limiter state rather than erroring. The wrapper
  should not hide that.
- **Omitting the expiration option strips existing TTLs**, exactly as `MSET` does.
  A wrapper that makes the TTL optional would therefore have a foot-gun default;
  better to require it, and expose `KEEPTTL` explicitly if it is ever needed.

## Expected gain

- **N round trips collapse to 1** for a group write. For a 200-key flush at 0.5 ms
  per round trip: ~100 ms → ~1 ms, a 50–100x improvement on the flush path. Same
  order as [perf-04](perf-04-pipelining.md), and for the same reason — the round trip
  dominates everything.
- **Atomic group expiry**, which removes the unbounded-key-growth failure mode of the
  `MSET`-then-`EXPIRE` workaround entirely.
- **Shorter connection hold time per flush**, which directly reduces the concurrency
  that drives [perf-01](perf-01-connection-pool-cap.md).
- On engines with the native command, it also avoids the Lua interpreter round trip
  server-side; that difference is small next to the round-trip saving, so the
  fallback is not a consolation prize — it captures nearly all the win.

## How to fix

Add one verb with two implementations and a cached capability flag.

**API.** Keep it Yaegi-safe and in the style of the existing verbs — plain
parameters, no functional options:

```go
// MSetEX writes every pair with one shared TTL in seconds, atomically.
func (sr *SimpleRedis) MSetEX(names []string, values [][]byte, seconds int64) error

// MSetEXAt writes every pair with one shared absolute Unix expiry, atomically.
func (sr *SimpleRedis) MSetEXAt(names []string, values [][]byte, unixSeconds int64) error
```

Parallel slices match `MGet`'s existing shape and avoid map iteration order
questions. Validate `len(names) == len(values)`, reject empty input like `MGet` does
(`:101-103`), and cap the pair count so one call cannot build an unbounded frame.

**Native path.** `MSETEX <numkeys> k1 v1 … EX <seconds>`. Treat the integer reply as
success on `1`; surface `0` rather than swallowing it, since without `NX`/`XX` it
should be unreachable and therefore indicates something unexpected.

**Fallback path.** One `EVAL`, atomic by virtue of being a script, and portable
across Lua 5.1 (Redis) and 5.4 (Dragonfly):

```lua
local ttl = ARGV[#KEYS + 1]
for i = 1, #KEYS do
  redis.call('SET', KEYS[i], ARGV[i], 'EX', ttl)
end
return 1
```

Deliberately avoid `unpack`/`table.unpack` — that name moved between Lua 5.1 and 5.4
and is precisely the kind of divergence the handoffs already flag with `table.maxn`.
A plain loop with `SET … EX` sidesteps it, and gets the per-key TTL applied inside
the atomic script. Use `EXAT` in the `MSetEXAt` variant.

**Capability detection.** Do *not* probe by attempting the native command blind on
every call.

- Detect once per client and cache it (guarded by the existing mutex). Either parse
  `INFO server` at first use for `server_name`/`valkey_version`, or attempt `MSETEX`
  once and treat `ERR unknown command` as "not supported" — safe here, unlike the
  rejected `MSET … EX` form.
- Cache the negative result so subsequent calls go straight to `EVAL`, and re-detect
  after a reconnect only if cheap; an engine does not gain the command mid-session,
  but a failover could move you to a different engine.
- The fallback plumbing is the same shape as the `NOSCRIPT` handling in
  [perf-05](perf-05-evalsha.md), so build them together — and once `EVALSHA` exists,
  the fallback costs a 40-byte digest rather than re-sending the script.

**Cluster note.** Both paths require all keys in one hash slot on a clustered engine;
Lua especially, as the issue itself points out. Document that callers must use hash
tags for grouped keys, or the command fails cross-slot.

## How to prove it

- Fake-server tests asserting the native argv exactly: `MSETEX`, the `numkeys` count,
  the pairs in order, then `EX`/`EXAT` and the value.
- A fake replying `-ERR unknown command 'MSETEX'` to the first attempt, asserting the
  client falls back to `EVAL`, succeeds, and uses `EVAL` directly on subsequent calls
  without re-probing.
- Equivalence: the same input through both paths leaves identical values and TTLs.
- Live e2e against **both** Redis and Dragonfly, per the handoff bar — Dragonfly
  exercises the fallback and (once available) Valkey exercises the native path. Assert
  the TTL actually landed, since that is the whole point of the verb.
- Interpreted coverage under Yaegi for both paths, matching the existing probe style.
- Edge cases the docs call out: a past `EXAT` deleting the keys while still replying
  `1`, and mismatched slice lengths being rejected before any I/O.
