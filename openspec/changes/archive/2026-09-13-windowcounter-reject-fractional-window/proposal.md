## Why

On DestBranch, `slidingAt` only rejects a window after integer division when `windowSec < 1`. A 1500ms window is accepted and silently uses 1-second Redis buckets and a 2s TTL. The spec already says window length SHALL be a whole number of seconds.

## What Changes

- `slidingAt` returns an error when `window < time.Second` **or** `window % time.Second != 0`. Do not truncate fractional windows into `windowSec` buckets.
- Weight denominator and TTL stay on `windowSec` only (whole seconds).
- Tests first: `TestRepro_FractionalWindowAccepted` subtest A — 1500ms Take MUST error. Keep `TestTake_SubSecondWindow`.
- Spec scenario: Take/Peek reject a non-whole-second window (1500ms), not only sub-second.
- Usage gotcha: reject, do not truncate.

## Capabilities

### New Capabilities

- None. This is a delta on the existing sliding-take leaf, not a new package or spec family.

### Modified Capabilities

- `std_go_windowcounter_sliding-take`: Window length SHALL be a whole number of seconds. Take and Peek MUST error when the window is shorter than one second **or** not divisible by one second. They MUST NOT truncate into second buckets. Weight and TTL use whole `windowSec` only.

## Impact

- `windowcounter/limiter.go` (`slidingAt` validation and weight denom).
- `windowcounter/repro_fractional_window_test.go` (new). `TestTake_SubSecondWindow` stays.
- `openspec/specs/std_go_windowcounter_sliding-take/spec.md` (after archive).
- `knowledge/devdocs/std_go_windowcounter.md` gotcha if it still reads as `< 1s` only.
- Take/Peek signatures unchanged. No other windowcounter bugs. No tokenbucket. No Redis PEXPIRE.
