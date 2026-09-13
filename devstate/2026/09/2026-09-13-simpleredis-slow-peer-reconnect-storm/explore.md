# Explore
IssueKey: 2026-09-13-simpleredis-slow-peer-reconnect-storm

## Concepts

**Close-on-outstanding-reply.** `do` (`simpleredis/resp.go`) sets `reusable` true only after a complete RESP value with an empty reader. A deadline (`os.ErrDeadlineExceeded` → `redis:timeout`) leaves the reply still in flight, so `runOnConn` releases with `reusable false` and `release` (`simpleredis/pool.go`) closes the TCP socket. That close is specified (`openspec/specs/std_go_simpleredis_tcp-session/spec.md`: leftover or a timeout on a reused connection must not park the socket; a timeout MUST NOT open a second connection *for that command*). It is not optional.

**Connect-per-command.** The next independent command sees an empty idle list and `dial`s. Nothing in `exec` / `shouldRetry` / `retryBackoff` governs that later dial. `redis:timeout` is never retried (documented deviation from go-redis), so backoff never runs on this path. AUTH and SELECT run once per new dial when `Pass` / `Database` are set.

**Churn, not an unbounded live set.** `PoolSize` still caps in-use turns (default 8). The storm is replacement rate plus a short overlap while Redis is still answering the timed-out command. Redis `maxclients` (default 10000 in redis.conf) is a simultaneous-client ceiling, not a historical dial counter.

**Operator lever.** `IOTimeout` default 100ms (`simpleredis/config.go`) is the cliff. go-redis on the researched pin defaults `ReadTimeout` to 5s. Raising this package’s default lengthens every hung-peer wait: `(MaxRetries+1)*(DialTimeout+IOTimeout)` (600ms at zeros). In-tree callers (`windowcounter`, `tokenbucket`) mostly use zero Config; the probe already sets 1s “so a 500ms TIME-wait Eval survives the 100ms default.”

## Decisions

- Closing the timed-out socket stays. Any “keep the socket and drain in the background” path violates the reply-boundary rule and needs a waiter goroutine (Yaegi `AfterFunc` / `_select` cost already documented on `watchConnClose`).
- Retry backoff cannot dampen this. `shouldRetry` returns false for `redis:timeout` before backoff is considered. Measured with `MaxRetries: -1`; with retries enabled the classifier would still skip the sleep.
- This package does not own dial-rate policy for a slow-but-alive peer. A breaker or limiter is new failure-memory: threshold, cooldown, fail-fast vs wait, Config surface, shared mutable state. Existing `knowledge/debt/2026-09-12-simpleredis-dial-circuit-breaker.md` already refused a similar policy for consecutive *dial* failures. The slow-peer trigger is different; the complexity cost is the same. Simplicity gate: do not ship it.
- Do not change `defaultIOTimeout` in this run. That is a behaviour change for every zero-Config caller (hung-peer budget and the 100ms tests that pin `IOTimeout()`). Human decision.
- The small coherent change is documentation: timeout → connect-per-command, extra AUTH/SELECT per reconnect, sequential dial rate ≈ `1/IOTimeout`, concurrent peak server sockets ≈ `2×PoolSize` per client from overlap, TIME_WAIT at the churn rate, Redis `maxclients` as the simultaneous ceiling. Operator raises `IOTimeout` (and sizes `PoolSize` × instance count against `maxclients`).
- No behaviour-changing product PR. No default-suite test that “bounds churn” — that test would encode a limiter this run is not adding.

## Measurements

Independent of the ticket’s tagged test. Throwaway `stormMeasureFake` against dest `simpleredis` on the original checkout (not committed). `OverFrees() == 0` on every row. Control: 15 Gets against a fast peer → 1 dial, idle 1.

| Case | Result |
| --- | --- |
| Ticket tagged test `TestBugSlowPeerDestroysEveryPooledSocket` (`IOTimeout` 30ms, delay 60ms, 15 Gets) | **15 dials, idle 0.** Reproduced. Fail as designed. |
| Serial, default `IOTimeout` 100ms, GET delay 110ms, 20 Gets, `MaxRetries: -1` | 20 timeouts, **20 dials**, idle 0, peak server open **2**, **9.9 dials/s** (≈ `1/IOTimeout`). 2.01s. |
| Same cliff just below: GET delay 80ms, default 100ms, 15 Gets | **15/15 ok, 1 dial, idle 1.** Reuse holds until the deadline is crossed. |
| AUTH+SELECT set, GET delayed 110ms, handshake not delayed, 15 Gets | **15 dials, AUTH=15, SELECT=15, GET=15**, idle 0, peak open 2. Two extra successful round trips per failed Get. |
| AUTH also delayed 110ms, 10 Gets | **AUTH=10, SELECT=0, GET=0.** Handshake timeout; GET never runs. `handshakeFailed` so not retried. |
| Concurrent 32 Gets, `PoolSize` 8, GET delay 110ms, overall budget 300ms (`MaxRetries: -1`) | **24 dials** (last 8 miss the 300ms library budget before a turn), **peak server open 16 = 2×PoolSize**, idle 0, GET=24. |
| Half-second serial at default 100ms | 5 commands, 5 dials, **9.9 cmd/s = 9.9 dials/s**. |

**Cost at a request rate.** Sequential: extra TCP per timed-out command; max serial rate is ~`1/IOTimeout` (10/s at the default) because each command blocks on the deadline. Concurrent: in-flight dials still ≤ `PoolSize`; timeout throughput ≈ `PoolSize/IOTimeout` (80/s at defaults 8 / 100ms) while callers overlap. Extra connections per second ≈ that timeout throughput, not the offered HTTP rate when the pool is saturated (waiters hit `PoolTimeout` / the command budget instead).

**AUTH+SELECT tax.** Fast handshake: +2 Redis round trips per reconnect on top of the timed-out GET (3 wire commands vs 1 on a reused socket). Slow handshake: AUTH alone times out; SELECT and GET never run; the storm is failed AUTHs.

**maxclients.** Peak simultaneous Redis-side sockets measured at **2×PoolSize** per client (server still sleeping on the timed-out command while the replacement is already accepted). Not unbounded. Default Redis `maxclients` is 10000 ([redis.conf](https://github.com/redis/redis/blob/8.10.0/redis.conf)). Pressure is `instances × ~2 × PoolSize` plus TIME_WAIT at the churn rate on the closer (the client). `ERR max number of clients reached` is retried on the command path and **not** retried on handshake (`handshakeFailed`).

What the ticket got right: reuse drops to zero; default 100ms is inside loaded-Redis p99; AUTH/SELECT multiply work; TIME_WAIT accumulates with churn. What it overstates: live connection count is still pool-capped; the overlap is a small integer multiple of `PoolSize`, not a connection bomb toward 10000 from one client.

## Options

| Option | What it does | Cost | Verdict |
| --- | --- | --- | --- |
| A. Document | Usage gotcha: timeout destroys the socket (required); next command dials; raise `IOTimeout`; size `PoolSize` × instances vs `maxclients`; AUTH/SELECT per reconnect | One packet (`knowledge/devdocs/std_go_simpleredis.md`). No runtime state. Matches package identity. | **Take.** |
| B. Raise `defaultIOTimeout` | Moves the cliff (go-redis is 5s; probe already uses 1s) | Behaviour change for every zero-Config caller. Hung-peer wait becomes `(MaxRetries+1)*(DialTimeout+newIOTimeout)`. Tests pin 100ms. Human decision, out of scope this run. | **Do not take.** |
| C. Retry timeouts / use existing backoff | Sleep between attempts of the *same* command | Spec forbids retrying `redis:timeout` and forbids a second connection for that timed-out command. INCR/EVAL double-apply. Does not govern the *next* command. | **Impossible under current spec.** |
| D. Cooldown timestamp after timeout | Atomic last-timeout; next dial waits or fail-fasts for ~`IOTimeout` | Shared mutable failure-memory; new wait vs fail-fast policy; concurrent `PoolSize` still bursts together; Yaegi-sensitive if a timer goroutine appears. Hidden limiter. | **Not small.** |
| E. Circuit breaker / dial-rate limiter | Fail fast after k timeouts; Config knobs | Threshold, cooldown, half-open, new errors. Same class as existing large dial-failure debt. Background or atomics. Distorts a stdlib-only Yaegi client. | **Not elegant. Do not ship to close the ticket.** |
| F. Background drain, keep the socket | Wait for the late reply, then park | Violates reply-boundary / no-drain spec. Extra goroutine per timeout. | **Reject.** |

## Recommendation

Stop the code path at the simplicity gate. Close-on-timeout is correct behaviour, not a defect in isolation. The operational shape (connect-per-command when the peer is merely slow) is real and measured; the owner of that shape is the operator’s `IOTimeout` / `PoolSize` / Redis `maxclients`, not a new client policy. Propose a documentation-only change. Do not open a behaviour-changing PR. Do not add a default-suite churn-bound test (that would specify a limiter).

## Open questions

- Q: Who owns dial-rate policy for a slow-but-alive Redis peer?
  Rank: additive asked — Desired asks whether this package is the right owner; close-on-outstanding-reply stays required
  Decision: resolved — the operator (`IOTimeout`, `PoolSize`, instance count, Redis `maxclients`). This package owns destroying a socket that is not on a reply boundary. It does not own a breaker or dial-rate limiter.
  By: explore

- Q: Should this run change `defaultIOTimeout`?
  Rank: additive asked — Desired forbids an agent decision; names it a behaviour change for every existing caller
  Decision: resolved — no. Human only. Raising it would lengthen hung-peer wait `(MaxRetries+1)*(DialTimeout+IOTimeout)` and move tests that pin 100ms.
  By: explore

- Q: Is there a small, coherent, Yaegi-safe code fix this run should take?
  Rank: additive asked — Desired: implement only if one falls out; simplicity gate prefers documenting
  Decision: resolved — no. Every code option that governs dial rate is new failure-memory (D/E) or a spec violation (C/F). The small coherent change is a usage gotcha (A).
  By: explore

- Q: Should a default-suite test bound dials under a slow peer when no limiter lands?
  Rank: additive asked — Desired requires that test only if implementing a code fix
  Decision: resolved — no. A passing bound without a limiter would be a lie; a failing bound would keep CI red.
  By: explore
