## Context

DestBranch already has `windowcounter.Limiter` with `Take`/`Allow`, exact INCR and buffered `redis_known`/`local_delta`, and `SetNowForTest`. There is no `Peek`. `windowLocked` GETs whenever `localDelta == 0`; `flushPending` deletes states with `localDelta == 0` and `expireAt == 0`. SimpleRedis already has `Get` (and unused-by-this-package `MGet`). See proposal.md for why. Specs: `std_go_windowcounter_sliding-take`, `std_go_windowcounter_sync-flush`. Explore: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Peek as Take without increment on the same type, formula, keys, and store.
- Buffered skip storm stays in memory after seed; exact Peek GETs every call.
- Tests at today's bar, including a fake GET counter.

**Non-Goals:**
- Circuit-breaker state machine, half-open, consecutive-failure, error-rate.
- Peek that always hits Redis in buffered mode.
- Making `Allow` mean Peek.
- MGET, tokenbucket, HTTP, 429, fail-open/fail-close.

## Decisions

1. **Peek method on `Limiter`.** Same signature as Take. Alternative: a separate observer type — rejected; one limiter, one store.

2. **Shared window-math helper.** Extract window start, Redis keys, weight, and TTL from Take's preamble; Take and Peek both call it. Alternative: duplicate the preamble — rejected; the formula is one job (deviation recorded).

3. **Exact Peek: two GET via `getCount`.** Current then previous. Miss is zero. No INCR, no EXPIRE. Alternative: MGet — rejected; `getCount` already owns miss-as-zero and the fake has GET, not MGET.

4. **Buffered Peek current window: seed GET + expireAt, then memory.** Under `l.mu`. GET only when the Redis key is absent from `windows` (first sight / window roll). Set `expireAt` so flush does not delete skip-storm buffers. Do not call `windowLocked` (that GETs when `localDelta == 0`). Previous window: existing `bufferedCountLocked`. Take is unchanged. Alternative: reuse `windowLocked` — rejected; skip storm would GET every Peek.

5. **Fake GET counter.** Test-only integer on `testFakeRedis`. Exact Peek asserts GETs; skip storm after seed does not. Name says test.

6. **Yaegi probe grows Peek+Take.** Same GOPATH copy pattern as `takeprobeSrc`. Compiled test owns start/skip.

## Risks / Trade-offs

- [Buffered Peek misses other instances' Takes until a local Take or window roll] → Mitigation: that is the locked skip-storm cadence; Take still GETs after flush.
- [Flush deletes Peek-seeded keys if expireAt stays 0] → Mitigation: Peek current-window seed sets expireAt like Take.
- [Clock tests wait a real window] → Mitigation: `SetNowForTest`; advance `now` until the formula admits; do not sleep `2×window`.
- [Live tests skip in CI] → Mitigation: keep existing CI env; do not add `-short` to the test job.

## Migration Plan

Additive method on an existing type. Rollback is revert. Callers keep using Take until they opt into Peek.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
