# Explore

## Concepts

```
  caller (middleware)                  windowcounter.Limiter
  ───────────────────                 ────────────────────
  1. Peek(key, limit, window)  ──►     estimate, no increment
       denied → skip backend
  2. call backend
  3. on failure, Take          ──►     increment, then estimate
```

The library stays a sliding-window hit counter. Peek is Take without the increment. It is not a circuit breaker, not tokenbucket, not a second limiter.

**Window counter** / **Take** / **Sliding estimate** / **sync_rate** already live on `knowledge/devdocs/std_go_windowcounter.md`. **Peek** is the missing sibling: same args and formula as Take, no hit recorded. `Allow` stays Take.

Identity (client address, user, tenant, Host) is not reconstructed here. The caller already owns the opaque key (`openspec/specs/std_go_windowcounter_sliding-take/spec.md` Caller owns the key).

Buffered store (one map, one lock):

```
  windows[redisKey] = { redisKnown, localDelta, expireAt }

  Take:  windowLocked (GET on seed AND when localDelta==0) → localDelta++
  Peek:  seed GET + expireAt, then memory; NEVER GET just because localDelta==0
  flush: EVAL when localDelta>0; delete when localDelta==0 AND expireAt elapsed (or 0)
```

Take's GET-when-`localDelta==0` is the post-flush refresh for other instances. Reusing it for Peek would GET every skip-storm Peek (delta stays 0). `bufferedCountLocked` seeds with `expireAt==0`, and `flushPending` deletes those on the next tick — a Peek-only storm would re-GET every `sync_rate`. Peek's current-window seed must set `expireAt` like Take.

Exact mode (`sync_rate==0`): Take is INCR + GET previous. Peek cannot INCR, so it GETs current and previous every call. Miss is zero (`getCount`). No EXPIRE (Peek does not create keys).

Usage packet `std_go_windowcounter.md` documents Take only. Spec leaves `std_go_windowcounter_sliding-take` and `std_go_windowcounter_sync-flush` likewise. Peek is a delta on those units.

Kong Advanced has no Peek analog (`knowledge/research/ext_kong_rate-limiting_sliding-sync/notes.md`). Redis GET/MGET already exist on SimpleRedis; `getCount` already maps `redis:miss` to zero. No new research folder.

## Decisions

- Peek on the existing `Limiter`. Same `(key, limit, window) (allowed bool, estimated float64, err)`. Same formula and `{opaqueKey}:{windowStartUnix}` keys. No new package.
- `Allow` stays `Take`. Do not alias Peek.
- One limiter, one store. Peek does not get its own consistency mode.
- Redis errors: same strings as Take. No fail-open/fail-close.
- Tests: unit fake TCP, live Redis+Dragonfly table-driven addr, Yaegi live calling Peek and Take. CI keeps `go test` without `-short` skip of live files.
- Spec delta on existing `std_go_windowcounter_*` leaves. Update `knowledge/devdocs/std_go_windowcounter.md` (Peek vs Take; buffered hot path).
- No circuit-breaker state machine in this library.

## Open questions

- Q: Exact-mode Peek: two GET via `getCount`, or one MGet of current+previous?
  Rank: additive asked — new Peek Redis path; Desired names GET/MGET as the exact-mode hits
  Decision: assumed — two GET via existing `getCount` (current, then previous). Do not add an MGET path. Tests prove GET. Fake MGET stays unused. Reuses miss-as-zero; avoids a second integer decoder.
  By: explore

- Q: How does buffered Peek share Take's map without GET-when-`localDelta==0`, and without flush deleting Peek-seeded keys?
  Rank: additive asked — Desired names skip-storm no-flood, seed/window-roll/flush cadence, and one store; Current cites `windowLocked` GET-when-delta-0 and `bufferedCountLocked` expireAt=0
  Decision: assumed — Peek current window: under `l.mu`, GET only on first sight of that Redis key (and thus on window roll = new key); set `expireAt` like Take so flush does not delete `expireAt==0` skip-storm buffers; return `redisKnown+localDelta` with no increment. Do not call `windowLocked` from Peek. Take keeps `windowLocked` (still GETs after flush). Previous window: same `bufferedCountLocked` as Take.
  By: explore

- Q: How do unit tests prove skip-storm GET count vs exact-mode Redis hits?
  Rank: additive asked — Desired names skip-storm does not GET every Peek, and exact Peek does hit Redis (GET/MGET)
  Decision: assumed — integer GET call counter on `testFakeRedis` (test-only field + getter). Not a command log. Exact Peek asserts GET count increases; buffered skip storm after seed does not.
  By: explore

- Q: Share Take's window-start / key / weight preamble with Peek, or duplicate it?
  Rank: bounded incidental — existing Take preamble; enumerated 1 production caller (`Take`; `Allow` delegates) plus tests in `windowcounter/limiter_test.go`, `live_test.go`, `yaegi_test.go`
  Decision: assumed — extract one helper both call. Duplicating the formula would let Peek and Take diverge. Deviation recorded.
  By: explore

- Q: Which spec leaves take the Peek scenarios?
  Rank: additive asked — Desired names existing windowcounter specs, not a new package name
  Decision: resolved — `std_go_windowcounter_sliding-take`: Peek does not increment, Peek agrees with Take before increment, Peek stays denied then becomes allowed as the window slides. `std_go_windowcounter_sync-flush`: buffered skip-storm no GET every Peek; exact Peek GETs every call. Live requirement grows Peek+Take on both engines. No new spec family.
  By: explore
