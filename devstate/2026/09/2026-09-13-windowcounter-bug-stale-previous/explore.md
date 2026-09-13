# Explore
IssueKey: 2026-09-13-windowcounter-bug-stale-previous

## Concepts

Buffered sliding Take estimates `current + previous × weight` from in-memory `windowState` (`redisKnown + localDelta`) and flushes with EVAL INCRBY. Exact Take INCRs current and GETs previous every call.

`windowLocked` (buffered Take, current key) GETs Redis when the key is unseen **or** when it is in `l.windows` and `localDelta == 0`. That is how same-window share after Sleep works (`TestBuffered_TwoClientsShareWithoutLastWriteWins`).

`bufferedCountLocked` (previous on buffered Take **and** Peek) returns memory if the key is already in `l.windows`. No GET. After a window roll the previous Redis key is the same string that was current; it is already in the map, so previous never refreshes.

```
window 1 (weight near 0, then roll)
  A.Take → current localDelta=1
  B.Take → current localDelta=1
  A.Sleep → Redis=1, A's redisKnown=1, localDelta=0
  B.Sleep → Redis=2, B's redisKnown=2
window 2 start (weight=1)
  A's previous key is still the window-1 key in memory (redisKnown=1)
  A's current is unseen → GET 0, then localDelta++
  estimated = 1 + 1×1 = 2  → admit (limit 2)
  Redis previous is 2 → should be 1 + 2×1 = 3 → deny
```

`peekCountLocked` (buffered Peek current) GETs only on first sight; comment forbids GET because `localDelta == 0`. Ticket keeps that for previous Peek.

Do not call `windowLocked` for previous: Take passes current-window `expireAt`, which would rewrite the previous key's flush EXPIREAT.

Identity: callers own the opaque key. The library never reads HTTP, client address, user, tenant, or Host (`std_go_windowcounter_sliding-take`).

## Decisions

- Repro first, then `limiter.go`. Unit fake Redis. File: `windowcounter/repro_stale_previous_test.go`. Deny at estimated 3 after A.Sleep then B.Sleep then next-window Take (limit 2, weight 1).
- On buffered Take only: if previous is in memory and `localDelta == 0`, GET and set `redisKnown`. If `localDelta > 0`, keep memory (unflushed delta is not in Redis).
- Do not change `bufferedCountLocked` used by Peek. GET-when-`localDelta==0` on that helper would GET previous on every Peek after flush (skip storm). Take-only refresh, same GET rule as current `windowLocked`, without copying current `expireAt` onto previous.
- Failed previous GET on Take returns the GET error (same as `windowLocked` current). Do not store it as `lastFlushErr` or wait for the outage probe.
- Do not INCR previous. Do not add a live e2e case this change (unit fake is the proof; dest e2e sliding boundary is exact-only).
- Peek at the rolled boundary may still see stale previous until a Take refreshes. Honour no-Peek-GET. Spec occupancy: Peek agrees with Take before increment except this buffered previous lag, which Take owns.
- Spec delta: `std_go_windowcounter_sync-flush` "later Take sees the shared count" must cover the rolled previous key, not only same-window. Sliding-take dump-at-boundary already requires deny; dest exact tests pass, buffered does not.
- Usage (`knowledge/devdocs/std_go_windowcounter.md`) still says buffered Peek is memory after first sight; it does not say Take GETs previous when `localDelta == 0`. Produce after the apply.

## Open questions

- Q: Who already owns identity (client address, user, tenant, Host, trust hop) for the window key?
  Rank: additive asked — sliding-take "Caller owns the key"; no reconstruct in this library
  Decision: resolved — the caller builds the opaque key; the limiter must not read HTTP or host identity.
  By: explore

- Q: Does Take-only previous GET live in `bufferedCountLocked` or a Take-only sibling so Peek stays skip-storm?
  Rank: additive asked — Desired 2 GET on buffered Take; Desired 4 do not GET previous on every Peek
  Decision: assumed — Take-only sibling (`takeBufferedPreviousLocked`). GET when previous `expireAt > 0` and `localDelta == 0`. Leave `bufferedCountLocked` for Peek.
  By: implement

- Q: Does a failed GET of previous on buffered Take propagate like `windowLocked`, or is it treated as Redis-down / pending-delta outage?
  Rank: additive asked — Desired 5 do not fold into Redis-down; current GET already returns
  Decision: assumed — return the GET error from Take like `windowLocked`. Do not set `lastFlushErr`. Redis is up in the repro.
  By: explore

- Q: Is unit fake Redis enough, or must live e2e add a buffered two-client window-roll case?
  Rank: additive incidental — Desired 1 names the unit repro; live e2e not in Desired
  Decision: assumed — unit fake Redis only (`repro_stale_previous_test.go`). Do not add `limiter_e2e_test.go` this change.
  By: explore

- Q: Peek-then-Take at the rolled clock: document occupancy miss, or GET previous on Peek too?
  Rank: bounded asked — Desired 4 no GET previous on every Peek; sliding-take Peek-agrees-with-Take is the tension; 1 Peek previous call site (`peekBuffered` → `bufferedCountLocked`)
  Decision: assumed — no Peek GET; Take refreshes; spec/usage may name that Peek can lag previous until Take. Do not fold Peek into the Take GET.
  By: explore
