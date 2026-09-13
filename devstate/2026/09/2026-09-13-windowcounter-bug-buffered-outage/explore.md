# Explore

## Concepts

Buffered Take on dest (PR #30) already admits from `redisKnown + localDelta`, then returns a Redis error. The ticket’s accepted contract is the local admit with `err=nil` up to this node’s `limit`, then local deny. Exact mode (`sync_rate == 0`) still returns Redis errors.

```
  takeBuffered / peekBuffered
       │
       ▼
  windowLocked / peekCountLocked
       │  GET on first sight; Take also GETs when localDelta == 0 (share refresh)
       │
       ├─ GET err ── dest: return err
       │             agreed: local buffer (or 0), err=nil; remember outage so the
       │             next Take/Peek does not wait Redis again
       │
       ▼
  Take: localDelta++; Peek: read only
  estimated = redisKnown + localDelta + previous × weight
       │
       ▼
  bufferedOutageErrorLocked
       ├ dest: lastFlushErr, else EVAL/GET after one missed sync_rate
       └ agreed: do not return that error; do not probe on Take/Peek
```

`Kill` on dest (`windowcounter/fake_redis_test.go`) already closes the listener and accepted sockets. Parent `trackConn` / `closeListenerAndConns` are the same job under other names.

Identity is not reconstructed here. The caller already owns the opaque key (`openspec/specs/std_go_windowcounter_sliding-take/spec.md` “Caller owns the key”).

Consumed: `knowledge/devdocs/index.md` → `index_std_go.md` → `std_go_windowcounter.md` (Peek = same observation as Take without a hit; usage still tells operators to check `err` to fail closed). Research indexes: Kong `ext_kong_rate-limiting_sliding-sync` already documents OSS local `cur_usage + cur_delta` until flush — no new research write.

Measure (`go test -short -count=1 -timeout 60s ./windowcounter` filtered to dest outage tests, worktree dest): all PASS under dest’s fail-closed lock.

| Test | Dest result | Agreed contract |
|------|-------------|-----------------|
| `TestTake_BufferedPendingDeltaOutage` | PASS `wantRedisOutage` on Take and Peek after kill + one `sync_rate` | `err=nil`, admit until this node’s `limit` |
| `TestTake_BufferedFlushThenKillFailsClosed` | PASS error after successful flush then `Kill` (path is `windowLocked` GET while `localDelta == 0`, not only the probe helper) | `err=nil`, local cap |
| `TestTake_BufferedTwoInstancesOutage` | PASS: nil-error admits ≤ global `limit`, and at least one Redis error | each instance may admit its own `limit` (combined can exceed global) |
| `TestTake_BufferedSleepStoresFlushError` | PASS Take/Peek return stored flush error | `err=nil` |
| `TestPeek_BufferedEmptyFlushThenKill` | PASS Peek probe GET after empty flush | `err=nil` |
| `TestTake_Unreachable` / `TestPeek_Unreachable` | PASS exact mode (`sync_rate == 0`) | keep: still return `redis:unreachable` |

Dest currently fail-closed on buffered outage. Parent example `windowcounter/repro_buffered_outage_test.go` still asserts `redis:unreachable`; Desired 1 rewrites that lock to `err=nil` + local deny after `limit`. Dest has no `repro_buffered_outage_test.go` yet.

## Decisions

- Tests first: copy/adapt the parent repro onto dest; rewrite assertions to `err=nil`, admit until this node’s `limit`, then `allowed=false`. Rewrite dest fail-closed cases that contradict that lock (`TestTake_BufferedPendingDeltaOutage` and siblings). Call dest `Kill`, not parent names.
- Buffered Take while Redis is down: return the local admit/deny and `err=nil`. Do not GET or INCR on Take to detect outage. Do not return `redis:unreachable` because `localDelta > 0` skipped GET.
- `windowLocked` GET when `localDelta == 0` stays the healthy share-refresh. On GET error, use existing `redisKnown + localDelta` (seed 0 on first sight) and `err=nil`. Store the outage so later Take/Peek skip Redis instead of paying timeout every call.
- Exact Take/Peek still return Redis errors (`TestTake_Unreachable`, `TestPeek_Unreachable`).
- Specs `std_go_windowcounter_sliding-take` (“Redis errors propagate” buffered pending-delta) and `std_go_windowcounter_sync-flush` (retained flush error returned from Take/Peek; stale buffer probes; two instances cannot multiply the limit) are the rewrite. Ticket wins. Document the deviation in those deltas plus `knowledge/devdocs/std_go_windowcounter.md` and README after the code is true. Do not write usage now — dest still matches the fail-closed packet.
- Do not touch bugs 2–7. Do not add GET/INCR on every buffered Take. Do not change exact-mode error propagation.

## Open questions

- Q: Does buffered Peek follow Take’s nil-error per-node fallback, or stay dest fail-closed?
  Rank: bounded incidental — 1 production method pair `Peek`/`peekBuffered` in `windowcounter/limiter.go`; 24 `.Peek(` call sites, all in `windowcounter/limiter_test.go`, `limiter_yaegi_test.go`, `limiter_e2e_test.go` (searched `*.go` for `.Peek(`; `e2e/` has no windowcounter). Outage-specific Peek asserts are 3 tests in `limiter_test.go` (`TestTake_BufferedPendingDeltaOutage`, `TestTake_BufferedSleepStoresFlushError`, `TestPeek_BufferedEmptyFlushThenKill`). All migratable here. Mandate: Unknowns say the ticket names buffered Take; no Desired line names Peek’s error contract.
  Decision: assumed — Peek follows Take (`err=nil`, same local estimate, no hit). Language already defines Peek as that observation; dest `peekBuffered` already shares `bufferedOutageErrorLocked`. Leaving Peek fail-closed would split the sibling. Exact-mode Peek stays error-returning.
  By: explore

- Q: Is dest `Kill` enough for the lock test, or must this change port parent `trackConn` / `closeListenerAndConns`?
  Rank: additive asked — Desired 2 names dest `Kill` versus those parent names; dest `Kill` already closes listener and sockets (`windowcounter/fake_redis_test.go`).
  Decision: resolved — keep dest `Kill`. Copied lock test calls `Kill`. Accept already appends `conns`. Do not add parent-name aliases.
  By: explore

- Q: Are `lastFlushErr` / probe helpers removed, or only disconnected from buffered Take?
  Rank: bounded asked — Desired 3–4 and Affected `takeBuffered` / `bufferedOutageErrorLocked`; return sites `takeBuffered` and `peekBuffered`; writes in `flushPendingLocked` and `bufferedOutageErrorLocked` (`windowcounter/limiter.go`).
  Decision: assumed — disconnect Take/Peek from returning `lastFlushErr` and from the missed-`sync_rate` EVAL/GET probe. Keep storing a failed flush (Sleep/Close/flushLoop already call `flushPendingLocked`) so later buffered Take/Peek skip Redis contact and stay `err=nil` instead of timeout-looping the share-refresh GET. Remove `bufferedOutageErrorLocked` as an error-returning helper. Do not delete `lastFlushErr` while it still skips GET. Leave unread `flushFailedAt` (pre-existing, no reader). Exact mode unchanged.
  By: explore

- Q: Who already owns the client identity on a Take/Peek key?
  Rank: additive asked — sliding-take scenario “Caller owns the key”
  Decision: resolved — reuse the caller’s opaque key. The library does not reconstruct client address, user, tenant, Host, or trust hop.
  By: explore
