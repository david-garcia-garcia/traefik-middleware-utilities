# SimpleRedis bugs

Date: 2026-09-13. Package `simpleredis`. Second pass, hunting **production killers only** (races, leaks, silent wrong data). Every item below was reproduced with a compiled test; nothing here is a style or micro-efficiency note. Candidates that were probed and did **not** reproduce are listed at the end with their measurements, so they do not get re-hunted.

Reproduction suite (these tests encode the spec and **fail on current code**):

```
go test -tags bugrepro -count=1 -timeout 120s -run TestBug ./simpleredis/
```

Default `go test ./simpleredis/` does not build that file (`//go:build bugrepro`). The default suite is green (91.9% statement coverage) and stays green.

Measured on this pass:

```
--- FAIL: TestBugInterpretedHandshakeFailureDefeatsUnreachableMatching
    interpreted IsUnreachable(handshake AUTH EOF) = false, want true
--- FAIL: TestBugInterpretedHandshakeFailureDefeatsNoAuthMatching
    interpreted errors.Is(handshake WRONGPASS, ErrNoAuth) = false, want true
--- FAIL: TestBugLostInUseTurnBricksPoolPermanently
    Get after 2 lost in-use turns = redis:unreachable, want success: turns=0/2, idle=0, accepts=2
--- FAIL: TestBugDesyncedSocketKeepsServingPreviousReplies
    Get(k5) = "STRAY", want "v5" or an error (command 5)
```

Ranked: 1 is live in the shipped Traefik plugin today. 2 is a latent hazard whose blast radius is unbounded. 3 needs a non-compliant peer but is silent and permanent when it happens.

---

## 1. Under Yaegi, a handshake failure is an error no matcher can classify

**Severity: high, and live in production now.** This is a regression introduced by the fix for the previous pass's bug 1.

**Spec** (`openspec/specs/std_go_simpleredis_tcp-session/spec.md`, and the package doc on `simpleredis.go:22`): "Exported sentinels. Match with `errors.Is` or `IsMiss` / `IsUnreachable` / `IsPoolWait`, **not string equality**." AUTH or SELECT peer-close is `redis:unreachable`; AUTH-class prefixes are `redis:noauth`.

**Cause.** The handshake-no-redial fix wraps AUTH/SELECT errors in a package-local struct with a custom `Unwrap`:

```10:20:simpleredis/pool.go
// handshakeFailure is an AUTH or SELECT error from dial. exec must not retry it.
type handshakeFailure struct {
	err error
}

// Error is the inner AUTH or SELECT failure text (redis:unreachable, LOADING …, redis:noauth).
func (e handshakeFailure) Error() string { return e.err.Error() }

// Unwrap is the inner AUTH or SELECT failure.
func (e handshakeFailure) Unwrap() error { return e.err }
```

`simpleredis` runs **interpreted** inside a Traefik local plugin. `errors.Is` is the *compiled* stdlib function; it discovers unwrapping with `err.(interface{ Unwrap() error })`. Yaegi does not expose an interpreted struct's `Unwrap` method through that assertion, so `errors.Is` stops at the wrapper and never reaches the inner sentinel. `dial` returns this wrapper on every handshake failure path:

```212:224:simpleredis/pool.go
	// AUTH before SELECT so a passworded server accepts the session.
	if sr.pass != "" {
		if _, _, err = sr.do(ctx, conn, [][]byte{[]byte("AUTH"), []byte(sr.pass)}); err != nil {
			conn.close()
			return nil, handshakeFailure{err: err}
		}
	}
	if sr.database != "" {
		if _, _, err = sr.do(ctx, conn, [][]byte{[]byte("SELECT"), []byte(sr.database)}); err != nil {
			conn.close()
			return nil, handshakeFailure{err: err}
		}
	}
```

`exec` returns that value to the caller unchanged, so the wrapper is what plugin code receives.

### Measured, compiled vs interpreted

Same binary, same fakes. `Error()` text is correct in both; only matching differs.

| Scenario | inner sentinel | `Error()` | compiled | interpreted (Yaegi) |
|---|---|---|---|---|
| AUTH peer-close | `errUnreachable` | `redis:unreachable` | `IsUnreachable` **true** | `IsUnreachable` **false** |
| AUTH `WRONGPASS` | `errNoAuth` | `redis:noauth` | `Is(ErrNoAuth)` **true** | `Is(ErrNoAuth)` **false** |
| SELECT `ERR DB index…` | plain `errors.New` | `ERR DB index is out of range` | n/a | all matchers false |
| AUTH `LOADING …` | plain `errors.New` | `LOADING …` | n/a | all matchers false |

Full interpreted matrix, including controls that prove the defect is specific to `handshakeFailure`:

```
AUTH-reject      Error()="redis:noauth"        IsUnreachable=false IsMiss=false Is(ErrNoAuth)=false
AUTH-EOF         Error()="redis:unreachable"   IsUnreachable=false IsMiss=false Is(ErrNoAuth)=false
SELECT-reject    Error()="ERR DB index is out of range"  IsUnreachable=false
AUTH-LOADING     Error()="LOADING Redis is loading the dataset in memory"  IsUnreachable=false
dead-port(ctrl)  Error()="redis:unreachable"   IsUnreachable=true      <-- bare sentinel, fine
miss(ctrl)       Error()="redis:miss"          IsMiss=true             <-- bare sentinel, fine
fmt-wrap(ctrl)   Error()="ctx: redis:unreachable"  IsUnreachable=true  <-- fmt.Errorf %w, fine
```

The controls matter: Yaegi handles the **stdlib** `fmt.Errorf("%w", …)` wrapper (`*fmt.wrapError`, a compiled type) correctly, and bare sentinels correctly. Only the package's own interpreted struct is opaque.

**Why this is a production killer, not cosmetics.** `handshakeFailure` wraps `errUnreachable` exactly when Redis accepts TCP but the handshake dies: restart, failover, `LOADING` during an RDB load, `ERR max number of clients reached`, a dropped connection mid-AUTH. Those are the moments a caller's fail-open / fail-closed decision matters most. A rate limiter, circuit breaker, or cache that branches on `IsUnreachable(err)` silently takes the **wrong** branch during a Redis outage — the one failure mode the sentinel exists to catch. The package tells callers to match this way and forbids string equality, so a correct caller is the one that breaks.

In-repo blast radius is currently narrow: the only consumer is `windowcounter/limiter.go:317`, which asks `IsMiss` and so falls through to the error path either way. The exposure is the public API used by plugin authors, and any future `IsUnreachable` branch added in this repo.

The internal classifier is **not** affected: `isHandshakeFailure` uses a type assert, not `errors.As`, and Yaegi resolves an interpreted type assert inside interpreted code. Verified — interpreted AUTH EOF still opens exactly 1 TCP connection, matching compiled. So the no-redial fix works; only the caller-facing contract broke.

```187:200:simpleredis/commands_exec.go
	if isHandshakeFailure(err) {
		return false
	}
```

### Agreed how (not implemented)

Do not fix this by adding `errors.As`, and do not tell callers to compare `Error()` text (the spec forbids that, and `redis:unreachable` is also `ErrPoolWait`'s text).

Stop carrying the "do not retry" mark in the error value's type. `dial` already knows the failure is a handshake failure at the point it returns; let it say so out of band and return the **inner** error unchanged, so the value the caller sees is the same bare sentinel that already matches under Yaegi:

- `dial` returns `(*pooledConn, error, handshake bool)`; `borrow` forwards that bool; `exec` passes it to `shouldRetry` instead of sniffing the type.
- `handshakeFailure`, `isHandshakeFailure`, and the `Unwrap` go away. Nothing then depends on Yaegi exposing an interpreted method to compiled stdlib code.
- Keep the existing no-redial tests (`TestHandshakeAuthEOFMustNotOpenSecondConnection` and siblings) unchanged; they must still see 1 accept.

Add the interpreted assertions to the permanent Yaegi suite. `yaegi_test.go` covers only happy paths today (`RoundTrip`, `IncrAndEval`, `EvalNoScript`, `MSetEXNative`, `MSetEXLua`, `MatchSentinels` on a `fmt.Errorf` wrapper). `MatchSentinels` passes precisely because it tests the one wrapper shape that works, which is why this went unnoticed. Every error path that returns a package-local error type needs an interpreted matcher assertion.

---

## 2. One lost in-use turn permanently bricks the client

**Severity: high, latent.** No reachable panic was found in compiled Go (see Rejected), so this is an unbounded blast radius on a hazard the code already acknowledges, not a live crash.

**Spec** (`std_go_simpleredis_tcp-session`, Idle connections are pooled): live sockets (idle plus checked out) SHALL not exceed `PoolSize`; when idle is empty and live sockets are at `PoolSize`, a caller waits for a released socket instead of dialing. With **zero** sockets actually live, refusing to dial is not backpressure — it is a dead client.

**Cause.** `exec` releases without `defer`, deliberately:

```45:52:simpleredis/commands_exec.go
		// If do panics under Yaegi, the process does not crash and this in-use-turn is lost.
		// Not deferred-release: https://github.com/david-garcia-garcia/traefik-middleware-utilities/pull/29
		values, reusable, err := sr.do(ctx, conn, args)
		if stop := contextStop(ctx); stop != nil {
			sr.release(conn, false)
			return nil, libraryTimeout(stop, libraryOwnsDeadline)
		}
		sr.release(conn, reusable)
```

The comment records the mechanism. What is not recorded, tested, or mitigated is that the loss is **permanent and cumulative**. `inUseTurns` is a fixed buffered channel created once in `New`; nothing ever refills it. There is no reaper, no generation counter, and no recovery on idle sweep. `OverFrees` counts *extra* returns and cannot help with *missing* ones. So each recovered panic between `borrow` and `release` shrinks the client's effective concurrency by one, forever, and after `PoolSize` of them the client is dead for the process lifetime.

This is the exact Traefik shape: Traefik recovers a panicking middleware per request and keeps serving, so the process survives and quietly loses a turn each time. A plugin that panics once an hour with `PoolSize` 8 is fully dead in 8 hours, and the symptom is `redis:unreachable` on every command with a healthy Redis and no open sockets.

### Measured

`PoolSize: 2`, one warm socket, then two recovered panics between `borrow` and `release`:

| | |
|---|---|
| in-use turns after | **0 / 2** |
| idle sockets | 0 |
| TCP accepts | 2 |
| next `Get` | `redis:unreachable`, forever |
| Test | `TestBugLostInUseTurnBricksPoolPermanently` |

The reproduction takes the turn and panics with a `recover()` in the caller, which is byte-for-byte what Traefik's recovery does to `exec`.

Yaegi panics are a demonstrated hazard in this package, not a hypothetical: three separate comments record interpreter panics that had to be designed around (`errors.As` on `handshakeFailure`, a `net.Error` type assert, and `interp._select` racing on a context channel). Any one of those recurring in a future edit converts this latent leak into an outage.

### Agreed how (not implemented)

Do not simply switch to `defer sr.release(...)`; PR 29 rejected that and the reason should be re-read before touching it. Fix the *permanence* instead, which is cheap and orthogonal:

- Make the turn a leased resource rather than a bare token: `borrow` records the taken turn on the client, and a lost lease is recoverable.
- Cheapest sufficient version: when `borrow` is about to fail with `errPoolWait`, check whether the client actually owns any socket (`len(idleConns)` plus a counter of dialed-not-yet-released sockets). Zero live sockets with zero free turns is impossible unless turns leaked; refill to the frozen cap and count it on a new `LostTurns()` metric next to `OverFrees()`.
- Either way `PoolSize` must stay the frozen live-socket cap, and `TestOverFreeAccountingStaysBalanced` must still end with `len(inUseTurns) == cap(inUseTurns)` and `OverFrees() == 0`.

Expose the leak: `LostTurns()` (or `PoolSize() - len(inUseTurns)` at rest) makes a bricked client diagnosable instead of looking like a Redis outage.

---

## 3. A desynced pooled socket is never evicted and serves the previous command's reply forever

**Severity: medium-high.** Requires a peer that writes an unsolicited reply. Silent, permanent, and returns another key's value when it happens.

**Spec** (`std_go_simpleredis_resp-commands`, Get): Get returns the value for its own key. A socket the decoder cannot prove is on a reply boundary MUST NOT keep serving commands.

**Cause.** `do` returns `reusable = true` whenever `readReply` parsed one well-formed value, and never checks whether the socket is drained:

```37:48:simpleredis/resp.go
	values, clean, err := readReply(conn.reader)
	if stop := contextStop(ctx); stop != nil {
		return nil, false, stop
	}
	if err != nil && !clean {
		if isDirtyProtocolError(err) {
			return nil, false, err
		}
		return nil, false, ioOrContext(ctx, ioBound, sr.IOTimeout(), err)
	}
	return values, true, err
}
```

One extra reply left in `conn.reader` shifts that socket one reply ahead permanently. Every later command on it reads the *previous* command's reply and returns it as its own, with `err == nil`. There is no reply-boundary check (no "buffered bytes must be zero"), no sequence tag, and no self-heal: `release` refreshes `lastUsed` on every use, so a socket kept warm by traffic never reaches `IdleTimeout` and never gets swept by `takeIdleConn`.

```170:171:simpleredis/pool.go
	conn.lastUsed = time.Now()
```

### Measured

Peer uses compliant RESP framing and answers `GET kN` with `vN`, but every 5th command it appends one extra bulk reply. `PoolSize: 1`, `MaxRetries: -1`, keys `k0..k39`:

| | |
|---|---|
| First wrong answer | `Get(k5)` returned `STRAYYY` |
| Then | `Get(k6)`→`v5`, `Get(k7)`→`v6`, `Get(k8)`→`v7`, … |
| Wrong answers | **35 of 40**, never healed |
| Errors raised | 0 — every wrong value returned with `err == nil` |
| Test | `TestBugDesyncedSocketKeepsServingPreviousReplies` |

Realistic triggers: a RESP3 server push or invalidation message on a connection the client believes is RESP2; a duplicating or error-injecting proxy (Sentinel front end, twemproxy, Envoy, the repo's own e2e RESP relay); a server-side bug on a non-Redis engine. The client never sends `HELLO`, so a compliant Redis 7 stays RESP2 and will not trigger this on its own — a compliant-peer trigger was searched for and not found (see Rejected).

The consequence is the worst class of cache bug: one tenant's cached value returned for another tenant's key, silently, until the process restarts or the socket happens to die. The decoder is otherwise strict — over-cap `$`/`*`, bad trailers, nested arrays, and errors-in-array all mark the socket dirty and destroy it — so this is the one hole in an otherwise fail-closed design.

### Agreed how (not implemented)

Make the boundary explicit rather than assumed. After a successful `readReply`, a socket is reusable only if the decoder consumed exactly the reply:

- In `do`, before returning `reusable = true`, require `conn.reader.Buffered() == 0`. Leftover bytes mean the peer is ahead of the protocol: return the decoded value to the caller (the reply itself was well-formed) but release the socket with `reusable = false` so it is destroyed instead of poisoning the pool.
- That keeps every existing pooling test green: a compliant peer writes one reply per command and leaves `Buffered() == 0` at that point. `TestConnectionIsReused` (25 sequential Gets, 1 connection) is the guard that this does not become churn.
- Do not try to drain and resynchronise; there is no way to know how many replies to discard. Destroying the socket is the only sound recovery.

---

## Rejected (probed on this pass, not bugs)

Recorded so these are not re-hunted. Measurements are from this pass.

| Candidate | Result |
|---|---|
| In-use-turn leak under a cancel/deadline storm | No leak. 24 goroutines × 3s against a peer that randomly delays, closes mid-reply, replies `LOADING`, or truncates a bulk; ~12k connections churned. `inUseTurns` ended **4/4** on every run, `OverFrees() == 0`. |
| Cross-key wrong values under that same chaos | Zero. Every non-error `Get` returned its own key's value. |
| Socket leak under chaos | None. Settled server-side open sockets `<= PoolSize` after quiescence on every run. |
| RESP decoder panic (would be a permanent turn leak via bug 2) | None found. `readReply` fuzzed **7.04M execs / 91s**, 51 corpus entries, no panic and no case returning values on a dirty stream. `parseLen` fuzzed for `length+2` overflow: none (capped by `maxBulkLength` before the allocation). |
| Goroutine or fd leak across `New`/use/`Close` cycles | None. 200 cycles × 4 concurrent Gets: goroutines 3 → 3, server-side open sockets 0. |
| `Close` during in-flight commands leaks sockets | No. 6 held Gets then `Close`: idle 0, open sockets 0, turns full. Matches the spec scenario. |
| Live sockets exceeding `PoolSize` | Not reproducible. Peak server-side open of 6 against `PoolSize` 4 is close lag (client `Close` → server `Read` error → goroutine exit), not a breach: a socket is either in `idleConns` or holds a turn, and `release` publishes before freeing. A naive `len(idleConns) + inUse` sampler reads up to 8 for the same reason and is not a valid metric. |
| `handshakeFailure` type assert panicking under Yaegi | Does not panic. Interpreted AUTH EOF opens 1 accept, same as compiled; the no-redial fix works interpreted. |
| Yaegi panic on the retry, I/O-timeout, cancel, pool-wait, or truncated-bulk paths | None. All five drive cleanly interpreted; only sentinel *matching* is wrong (bug 1). |
| Concurrent `MSetEX` unknown-command fallback racing `groupWrite` | Benign. 16 goroutines × 25 `MSetEX` against a reject-MSETEX fake: 0 failures, 16 MSETEX probes, turns full, `OverFrees() == 0`. |
| Compliant-peer trigger for the bug 3 desync | Not found. Every decoder path that cannot prove a boundary (over-cap `$`/`*`, bad CRLF trailer, nested array, error-in-array, unknown type byte, mid-array timeout, partial write) already marks the socket dirty and closes it. |
| Command injection via key, value, script, or TTL | Not possible. `writeCommand` emits length-prefixed RESP bulk strings for every argument; payload bytes are never scanned for CRLF. |
| Clean socket discarded when the deadline passes after a successful read | Real but specified. `exec`'s post-`do` `contextStop` releases with `reusable = false`; the spec requires closing the in-use socket on cancel. Churn, not a killer: honest fast peer with tight caller deadlines measured 0.31 dials per command. |
| `takeIdleConn` in-place filter (`sr.idleConns[:0]`) aliasing | Safe. Write index never exceeds the read index. Retains at most `cap - len` closed `*pooledConn` (≈8KB each, bounded by `PoolSize`); not a leak worth changing. |
| `Eval` / `MSetEX` stacking one overall deadline per hop | Intended, already commented on `Eval`, `MSetEX`, `MSetEXAt`, and `msetex` after the previous pass. |

## Previously reported, now fixed

The first pass (2026-09-12) reported three items; all three are fixed and covered by the default suite, so they are no longer reproductions. The old `bugs_repro_test.go` no longer compiled (it predated the `Eval` caller-digest signature) and has been replaced by this pass's suite.

| Previous bug | Fix | Regression cover |
|---|---|---|
| Handshake failure redialed a second TCP connection | `handshakeFailure` mark + `shouldRetry` short-circuit | `TestHandshakeAuthEOFMustNotOpenSecondConnection`, `…SelectEOF…`, `…AuthLoading…`, `…AuthMaxClients…` |
| `Eval` / `MSetEX` bound a second overall deadline | Accepted as intended; documented in comments | n/a |
| `Eval` mapped Lua `false`/`nil` (`$-1`) to `redis:miss` | `readReply` returns a nil slot; `Get` interprets it | `TestGetNullBulkIsMiss`, `commands_eval_test.go:176` |

Note that the fix for the first item is what introduced bug 1 above.
