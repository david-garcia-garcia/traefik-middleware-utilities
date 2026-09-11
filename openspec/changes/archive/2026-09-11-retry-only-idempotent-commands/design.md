## Context

Dest `exec` retries every verb after a dead reused socket (`simpleredis/simpleredis.go`). Timeouts are already skipped. AUTH/SELECT in `dial` call `do`, not `exec`. Nine wrappers call `exec`. See proposal.md for why. Proceed policies: `devstate/explore.md`. Research: `knowledge/research/ext_redis_incr/`, `knowledge/research/ext_dragonfly_eval/`.

## Goals / Non-Goals

**Goals:**
- Verb-gated retry at the nine `exec` call sites.
- Fake close-before-reply (mutate, then close; no truncated-bulk).
- Compose drop-relay in front of Redis and Dragonfly; Pester `/redis` `/dragonfly` extra headers.
- Keep Yaegi Incr/Eval happy-path.

**Non-Goals:**
- Parsing the verb inside `exec`.
- A client option or descriptor table.
- Distinguishing write-flush failure from read failure.
- Changing windowcounter, tokenbucket, pipelining, EVALSHA, go-redis, miniredis.
- Inventing the test-04 truncated-bulk harness.
- A lost-reply Yaegi interp harness.
- Renaming compose project `reclaim-e2e`.

## Decisions

1. **Bool `retryDeadPool` as the first `exec` argument.** Signature becomes `exec(retryDeadPool bool, args ...[]byte)`. Get/MGet/Set/Del/Expire/ExpireAt pass `true`. Incr/IncrBy/Eval pass `false`. The existing early return gains `|| !retryDeadPool` so a dead reused socket on a non-retry verb returns the first `do` error (`redis:unreachable`) without a second `borrow`/`do`. Alternative: inspect `args[0]` — rejected; that hides the policy in `exec` and is a descriptor table. Alternative: a struct of flags — rejected; nine call sites, one bit.

2. **Fake one-shot close-before-reply on dest `fakeRedis`.** A test-only flag (name includes test, e.g. `closeBeforeReplyOnce`) makes `serve` apply the command (including INCR/INCRBY/EVAL mutate), then close the socket without writing the reply, then clear the flag. Tests warm the pool with a successful Get first so the lost-reply command is reused. Do not add a truncated-bulk path. Alternative: client-side idle close (existing `TestStaleConnectionIsRetried`) — that never applies the write; it cannot pin stored `1`.

3. **Compose sidecar `e2e/respdroprelay`.** Tiny stdlib Go `main` (not imported by `simpleredis`). Env `UPSTREAM` (host:port), listen `:6379`. Per accepted client socket: dial upstream, then loop read-one-RESP-command, write it upstream, read one engine reply. If the command verb (first argv, compared case-insensitively) is INCR, INCRBY, or EVAL: close the client without writing that reply. Other verbs write the reply through. Two compose services, same image: `redis-drop` with `UPSTREAM=redis:6379`, `dragonfly-drop` with `UPSTREAM=dragonfly:6379`. `depends_on` the matching engine. Do not put the relay in front of happy-path Host. Alternative: DEBUG SLEEP / CLIENT KILL / a timeout — rejected; timeout is already non-retry and would not pin apply-then-drop.

4. **Probe second client `DropHost`.** `Config` gains `DropHost`. `New` Inits a second `SimpleRedis` to that host (still no dial). Happy-path `Host` stays `redis:6379` / `dragonfly:6379`. Labels: `dropHost=redis-drop:6379` and `dropHost=dragonfly-drop:6379`. On each request, after happy-path verbs: warm the drop client with a pass-through Get so the next command is reused; then Incr and Eval on distinct keys through the drop client; then Get those keys on the happy-path client (engine, not relay) for stored headers. Do not `http.Error` on expected `redis:unreachable` from the drop client. Headers: `X-SimpleRedis-DropIncr`, `X-SimpleRedis-DropIncrStored`, `X-SimpleRedis-DropEval`, `X-SimpleRedis-DropEvalStored`. Eval body stays the Kong KEYS snippet (Lua 5.1-safe). Alternative: a second HTTP route — extra routers; explore asked the same `/redis` `/dragonfly` requests.

5. **Yaegi stays `TestYaegi_IncrAndEval` happy-path.** Lost-reply is compiled fake tests plus Pester. Same `Error()` string; no interp close-before-reply.

## Risks / Trade-offs

- [False-negative: INCR whose write never landed on a reused socket also returns `redis:unreachable`] → Accepted; out of scope forbids write-vs-read as the gate.
- [Drop-relay must parse RESP enough to see the verb and one reply] → Mitigation: one-command loop, no pipeline; compose Redis/Dragonfly have no AUTH; probe drop client uses empty pass/database.
- [Drop Incr on a new dial would already not retry (`!reused`)] → Mitigation: warm the drop client with a pass-through Get before Incr/Eval.
- [Dragonfly undeclared KEYS / `table.maxn`] → Mitigation: existing Kong snippet with `KEYS[1]`.
- [Pester 502 if drop unreachable aborts the handler] → Mitigation: set drop headers from the error text; do not fail the request on expected unreachable.

## Migration Plan

Library behavior change for lost-reply INCR/INCRBY/EVAL only (those callers already fail closed on `redis:unreachable`). Rollback is revert. No production deploy.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
