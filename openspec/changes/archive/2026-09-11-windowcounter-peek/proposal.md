## Why

On DestBranch, `windowcounter` can only observe the sliding estimate by calling `Take`, which always increments. A middleware that skips a backend after a failure threshold cannot tell whether the window has cooled without counting another hit.

## What Changes

- Add `Peek(key, limit, window) (allowed bool, estimated float64, err)` on the existing `Limiter`: same formula and Redis keys as Take, **no increment**.
- One limiter, one store. `sync_rate > 0`: Peek reads `redis_known + local_delta` under the same lock (memory hot path; GET on seed/window roll only). `sync_rate == 0`: Peek is exact (`GET` current and previous every call). Peek does not get its own consistency mode.
- `Allow` stays an alias of Take.
- Redis errors: same as Take (`redis:unreachable` / timeout). No fail-open/fail-close.
- Unit (fake TCP GET counter), live Redis + Dragonfly, Yaegi live Peek+Take. CI keeps running live files.
- Update `knowledge/devdocs/std_go_windowcounter.md` (Peek vs Take; buffered hot path).

## Capabilities

### New Capabilities

- None. Peek is a delta on the existing windowcounter leaves, not a new package or spec family.

### Modified Capabilities

- `std_go_windowcounter_sliding-take`: Peek is Take without increment; Peek and Take agree on estimated/allowed for the same clock and keys before the increment; after enough Takes to deny, Peek stays denied then becomes allowed as the window slides (clock + formula). Redis errors on Peek match Take. Yaegi probe calls Peek and Take. `Allow` remains Take.
- `std_go_windowcounter_sync-flush`: Buffered Peek during a skip storm (many Peeks, zero Takes) MUST NOT GET Redis every call; exact-mode Peek GET current and previous every call. Live both engines include Peek+Take. Peek uses the same store and lock as Take.

## Impact

- `windowcounter/limiter.go` (Peek; shared window-math helper; buffered seed-with-expireAt path that does not GET when `localDelta == 0`).
- `windowcounter/limiter_test.go`, `fake_redis_test.go` (GET counter), `live_test.go`, `yaegi_test.go`.
- `openspec/specs/std_go_windowcounter_sliding-take/spec.md` and `std_go_windowcounter_sync-flush/spec.md` (after archive).
- `knowledge/devdocs/std_go_windowcounter.md`.
- No new package. No circuit breaker. No tokenbucket. No HTTP/429. CI workflow unchanged except tests that already run.
