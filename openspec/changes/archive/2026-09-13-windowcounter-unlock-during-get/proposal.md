## Why

Buffered Take and Peek hold the limiter mutex across Redis GET and Eval. A delayed GET on one opaque key blocks Take on every other key on that limiter for the full round trip. Fast Take must finish well under that delay.

## What Changes

- Unit fake can hold a GET whose Redis key matches a prefix until release or a timeout.
- A failing-then-passing repro proves a buffered Take on an unrelated key does not wait for that held GET.
- Buffered Take, Peek, flush, and outage probe MUST NOT call Redis while holding the limiter mutex.
- After a successful flush Eval, `local_delta` subtracts the flushed amount instead of clearing blindly, so a Take that landed during Eval is not dropped.
- Exact mode is unchanged.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `std_go_windowcounter_sliding-take`: buffered Take/Peek MUST NOT wait on another key's Redis GET; unit fake MUST be able to hold a GET by key prefix.
- `std_go_windowcounter_sync-flush`: flush MUST NOT Eval while holding the limiter mutex; after a successful Eval, `redis_known` is the EVAL return and `local_delta` decreases by the flushed amount (not set to zero blindly).

## Impact

- `windowcounter/limiter.go` buffered Take/Peek/flush/outage probe lock scope
- `windowcounter/fake_redis_test.go` GET-hold helpers
- `windowcounter/repro_lock_during_get_test.go` (new)
- `openspec/specs/std_go_windowcounter_sliding-take/spec.md`
- `openspec/specs/std_go_windowcounter_sync-flush/spec.md`
- `knowledge/devdocs/std_go_windowcounter.md` if usage still implies GET under the Take lock
