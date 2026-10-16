## Why

`Gate.Allow` currently inspects `context.Context` only to reject callers whose request is already canceled or past deadline. That couples synchronous admission logic to request lifecycle and duplicates what middleware can decide before invoking the gate. The library should stay a pure state machine under its mutex; callers own whether to attempt admission after cancellation.

## What Changes

- **BREAKING**: `Allow(key string)` replaces `Allow(ctx context.Context, key string)`; remove the `ctx.Err()` early return and drop unused `context` import from `allow.go`.
- Keep `Report(key, success bool)` unchanged.
- No changes to credit math, cooldown scheduling, map TTL/cap, or mutex design.
- Update all in-package `Allow` call sites in tests, Yaegi probe, and benchmarks.
- Remove `TestAllow_CanceledContext`; do not add middleware-simulation tests in this package.
- Fold delta into live spec `std_go_backendbackoff_allow`: new signature, caller-owned cancellation before `Allow`, remove canceled-context scenario and related test obligations.
- Update `knowledge/devdocs/std_go_backendbackoff.md` usage (no `req.Context()` on `Allow`; document checking cancellation before call).
- `README.md` stays out of minimum scope per explore (optional alignment in implement).

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `std_go_backendbackoff_allow`: Admission API is context-free; cancellation is enforced by the caller before `Allow`; tests and Yaegi obligations no longer require canceled-context denial inside the library.

## Impact

- **API**: Breaking change for any external copy of `backendbackoff` that still passes `context.Context` into `Allow` (out-of-repo consumers migrate on their own cycle).
- **Code**: `backendbackoff/allow.go`, `gate_test.go`, `gate_yaegi_test.go`, `bench_test.go`.
- **Docs**: live spec (via archive delta), `knowledge/devdocs/std_go_backendbackoff.md`.
- **Unchanged**: `Report`, other packages' `Allow(ctx, …)` signatures, token bucket/window counter.
