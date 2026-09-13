# Requirement
IssueKey: 2026-09-13-windowcounter-bug-fractional-window

## Problem
`slidingAt` treats any `window` with `int64(window/time.Second) >= 1` as valid. A 1500ms window is accepted but bucket math uses `windowSec == 1`, so Redis keys roll every second and TTL is `2 * windowSec` (2s), not two full window lengths. Weight uses the full `window` Duration in the numerator path while bucket boundaries use truncated seconds, so the sliding formula and key alignment disagree with the caller’s window.

## Current (code)
- `windowcounter/limiter.go` `slidingAt` — `windowSec := int64(window / time.Second)`; rejects only `windowSec < 1`; 1500ms yields `windowSec=1` with no error.
- `windowcounter/limiter.go` `slidingAt` — `windowStart := now.Unix() / windowSec * windowSec` and `ttlSec := 2 * windowSec`; fractional windows silently use truncated-second buckets.
- `windowcounter/limiter.go` `slidingAt` — `weight := 1 - float64(elapsed)/float64(window)` uses full `window` while `elapsed` is whole-second based; with `windowSec=1`, elapsed within a second is always 0 at integer Unix times.
- `windowcounter/limiter.go` `Take` / `Peek` — call `slidingAt`; no further window validation.
- `windowcounter/limiter_test.go` `TestTake_SubSecondWindow` — `500*time.Millisecond` Take must error; present on dest.
- `windowcounter/` — no `repro_fractional_window_test.go` on dest (`origin/master`).
- `openspec/specs/std_go_windowcounter_sliding-take/spec.md` — “Window length SHALL be a whole number of seconds. Sub-second windows MUST NOT be supported.”
- `knowledge/devdocs/std_go_windowcounter.md` — usage packet exists; not re-read for sub-second-only wording in this prepare pass.

## Desired
1. Tests first: add `windowcounter/repro_fractional_window_test.go` with `TestRepro_FractionalWindowAccepted` so 1500ms Take errors (must fail on unfixed code where Take succeeds).
2. `slidingAt`: return error when `window < time.Second` or `window % time.Second != 0`; do not truncate fractional windows into buckets.
3. Weight denominator and TTL stay tied to `windowSec` only (whole seconds), e.g. weight uses `float64(windowSec)*float64(time.Second)` so keys and formula share one length.
4. After fix: repro passes; `go test -short -count=1 -timeout 60s ./windowcounter` passes; keep `TestTake_SubSecondWindow`.

## Affected
- `windowcounter/limiter.go` (`slidingAt` validation and weight denominator)
- `windowcounter/repro_fractional_window_test.go` (new repro)
- `openspec/specs/std_go_windowcounter_sliding-take/spec.md` (may need explicit construction-time reject for fractional seconds — propose phase)
- `knowledge/devdocs/std_go_windowcounter.md` if it documents only `< 1s` rejection

## Out of scope
- Other `windowcounter/BUGS.md` items (expire retry, lock during GET, stale previous window, etc.)
- Changing sync flush, Peek/Take buffering, or Redis script shape
- Supporting sub-second or fractional-second windows as a feature

## Unknowns
- Exact error string/sentinel for fractional window (reuse “at least one second” vs new “whole seconds” wording).
- Whether repro asserts Redis key rollover (parent working tree has a richer repro; dest may start with error-only subtest).

## Tensions
- Spec already requires whole-second windows; dest code accepts 1500ms. Ticket wins: reject at `slidingAt`, do not silent truncate.
- Caller references `TestRepro_FractionalWindowAccepted` and optional BUGS.md row; those exist on parent checkout uncommitted, not on dest — implement adds the test file on IssueKey.
