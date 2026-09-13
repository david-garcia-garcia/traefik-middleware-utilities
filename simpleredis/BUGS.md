# SimpleRedis bugs

Date: 2026-09-13. Package `simpleredis`. Second pass, hunting **production killers only** (races, leaks, silent wrong data). Candidates that were probed and did **not** reproduce are listed under Rejected with their measurements and the test that now locks them, so they do not get re-hunted.

This file is the written record of that pass. The three real defects are owned by other PRs; this PR does not change product source.

## Recorded, fixed in other PRs

These were reproduced on dest at the time of the hunt. Product fixes land in the named branches, not here.

### 1. Under Yaegi, a handshake failure is an error no matcher can classify

**PR:** `2026-09-13-simpleredis-handshake-sentinel-match`

`handshakeFailure` wraps AUTH/SELECT errors in a package-local struct. Compiled `errors.Is` unwraps it; Yaegi does not expose that interpreted `Unwrap` to compiled stdlib, so `IsUnreachable` / `errors.Is(..., ErrNoAuth)` are false on handshake failure even though `Error()` text is correct. Controls (bare sentinel, `fmt.Errorf("%w")`) still match interpreted.

Measured: AUTH peer-close `IsUnreachable` compiled true / interpreted false; AUTH `WRONGPASS` `errors.Is(ErrNoAuth)` compiled true / interpreted false. Internal `isHandshakeFailure` type assert still works interpreted (1 accept).

This coverage PR does **not** assert interpreted AUTH/SELECT matcher results; that would be red until that PR lands.

### 2. Interpreted defer runs on panic under Yaegi

**Proof:** `simpleredis/yaegi_defer_test.go`

Interpreted `defer` does run on panic under Yaegi v0.16.1, for an explicit interpreted panic, the interpreter's own `errors.As` panic, and a nil-map write. Closed PR 29 rejected deferred `release` on the opposite belief; do not treat that close as policy. `runOnConn` defers `release`, `borrow` defers `freeInUseTurn` when it does not hand a socket off, and `release` unlocks the idle-list mutex via defer inside the keep-or-close owner.

Measured before deferred `release`: PoolSize 2, two recovered panics → turns 0/2, idle 0, next Get `redis:unreachable`.

### 3. A desynced pooled socket is never evicted and serves the previous command's reply forever

**PR:** `2026-09-13-simpleredis-desync-boundary-check`

`do` returns `reusable = true` after one well-formed `readReply` without requiring `Buffered() == 0`. An extra reply left in the reader shifts that socket one reply ahead permanently. Wrong values return with `err == nil`.

Measured: extra bulk every 5th command, PoolSize 1, keys k0..k39 → `Get(k5)` returned `STRAYYY`, then 35 of 40 wrong, 0 errors.

---

## Rejected (probed on this pass, not bugs)

Recorded so these are not re-hunted. Measurements are from the hunt. **Lock** is the permanent test on dest.

| Candidate | Result | Lock |
|---|---|---|
| In-use-turn leak under a cancel/deadline storm | No leak. 24 goroutines × 3s against a peer that randomly delays, closes mid-reply, replies `LOADING`, or truncates a bulk; ~12k connections churned. `inUseTurns` ended **4/4** on every run, `OverFrees() == 0`. | `TestChaosPoolInvariants` |
| Cross-key wrong values under that same chaos | Zero. Every non-error `Get` returned its own key's value. | `TestChaosPoolInvariants` |
| Socket leak under chaos | None. Settled server-side open sockets `<= PoolSize` after quiescence on every run. | `TestChaosPoolInvariants` |
| RESP decoder panic (would be a permanent turn leak via bug 2) | None found. `readReply` fuzzed **7.04M execs / 91s**, 51 corpus entries, no panic and no case returning values on a dirty stream. `parseLen` fuzzed for `length+2` overflow: none (capped by `maxBulkLength` before the allocation). | `FuzzReadReply`, `FuzzParseLen` |
| Goroutine or fd leak across `New`/use/`Close` cycles | None. 200 cycles × 4 concurrent Gets: goroutines 3 → 3, server-side open sockets 0. | `TestLifecycleNewUseCloseDoesNotLeak` |
| `Close` during in-flight commands leaks sockets | No. 6 held Gets then `Close`: idle 0, open sockets 0, turns full. Matches the spec scenario. | `TestCloseDuringHeldGetsReturnsTurns` |
| Live sockets exceeding `PoolSize` | Not reproducible. Peak server-side open of 6 against `PoolSize` 4 is close lag (client `Close` → server `Read` error → goroutine exit), not a breach: a socket is either in `idleConns` or holds a turn, and `release` publishes before freeing. A naive `len(idleConns) + inUse` sampler reads up to 8 for the same reason and is not a valid metric. | `TestChaosPoolInvariants` (settled sockets only; see comment in that test) |
| `handshakeFailure` type assert panicking under Yaegi | Does not panic. Interpreted AUTH EOF opens 1 accept, same as compiled; the no-redial fix works interpreted. | `TestHandshakeAuthEOFMustNotOpenSecondConnection` (and siblings). This PR does not re-assert interpreted handshake matching. |
| Yaegi panic on the retry, I/O-timeout, cancel, pool-wait, or truncated-bulk paths | None. All five drive cleanly interpreted; only sentinel *matching* is wrong (bug 1). | `TestYaegiErrorpath_DialRetry`, `TestYaegiErrorpath_StallTimeout`, `TestYaegiErrorpath_CancelMidCommand`, `TestYaegiErrorpath_PoolWait`, `TestYaegiErrorpath_TruncatedBulk` |
| Concurrent `MSetEX` unknown-command fallback racing `groupWrite` | Benign. 16 goroutines × 25 `MSetEX` against a reject-MSETEX fake: 0 failures, 16 MSETEX probes, turns full, `OverFrees() == 0`. | `TestConcurrentMSetEXUnknownCommandFallback` |
| Compliant-peer trigger for the bug 3 desync | Not found. Every decoder path that cannot prove a boundary (over-cap `$`/`*`, bad CRLF trailer, nested array, error-in-array, unknown type byte, mid-array timeout, partial write) already marks the socket dirty and closes it. | dest `resp_test.go` / `pool_test.go` dirty-reply cases (not expanded here) |
| Command injection via key, value, script, or TTL | Not possible. `writeCommand` emits length-prefixed RESP bulk strings for every argument; payload bytes are never scanned for CRLF. | `TestWriteCommandDoesNotInjectCommands` |
| Clean socket discarded when the deadline passes after a successful read | Real but specified. `exec`'s post-`do` `contextStop` releases with `reusable = false`; the spec requires closing the in-use socket on cancel. Churn, not a killer: honest fast peer with tight caller deadlines measured 0.31 dials per command. | dest `TestGetCancelFreesTurnAndDoesNotPool` |
| `takeIdleConn` in-place filter (`sr.idleConns[:0]`) aliasing | Safe. Write index never exceeds the read index. Retains at most `cap - len` closed `*pooledConn` (≈8KB each, bounded by `PoolSize`); not a leak worth changing. | dest idle-sweep tests |
| `Eval` / `MSetEX` stacking one overall deadline per hop | Intended, already commented on `Eval`, `MSetEX`, `MSetEXAt`, and `msetex` after the previous pass. | n/a (comments) |

`TestChaosPoolInvariants` and the lifecycle tests skip under `go test -short` so CI Unit stays fast. Seed-corpus `Fuzz*` functions still run as unit tests.

## Previously reported, now fixed

The first pass (2026-09-12) reported three items; all three are fixed and covered by the default suite, so they are no longer reproductions. The old `bugs_repro_test.go` no longer compiled (it predated the `Eval` caller-digest signature) and has been replaced by this pass's suite.

| Previous bug | Fix | Regression cover |
|---|---|---|
| Handshake failure redialed a second TCP connection | `handshakeFailure` mark + `shouldRetry` short-circuit | `TestHandshakeAuthEOFMustNotOpenSecondConnection`, `…SelectEOF…`, `…AuthLoading…`, `…AuthMaxClients…` |
| `Eval` / `MSetEX` bound a second overall deadline | Accepted as intended; documented in comments | n/a |
| `Eval` mapped Lua `false`/`nil` (`$-1`) to `redis:miss` | `readReply` returns a nil slot; `Get` interprets it | `TestGetNullBulkIsMiss`, `commands_eval_test.go:176` |

Note that the fix for the first item is what introduced recorded defect 1 above.
