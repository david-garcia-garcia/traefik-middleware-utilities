## Context

DestBranch `slidingAt` computes `windowSec := int64(window / time.Second)` and errors only when `windowSec < 1`. Take and Peek both call it; Allow aliases Take. Weight uses `float64(window)`; keys and TTL use `windowSec`. See proposal.md for why. Spec: `std_go_windowcounter_sliding-take`. Explore: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Reject non-whole-second windows at the shared Take/Peek gate.
- Keys, weight, and TTL share one whole-second length.
- Repro-first: 1500ms Take errors; 500ms still errors.

**Non-Goals:**
- Sub-second or fractional-second windows as a feature (PEXPIRE, ms buckets).
- Other windowcounter bugs (expire retry, lock during GET, Peek/Take occupancy).
- Peek-only extra tests (shared gate).
- Changing Take/Peek signatures.

## Decisions

1. **Validate on the Duration before `windowSec` buckets.** Error if `window < time.Second` **or** `window % time.Second != 0`. Do not truncate. Alternative: floor 1500ms to 1s and keep accepting — rejected; spec forbids the window; silent 1s buckets is the bug.

2. **Weight denom is `float64(windowSec) * float64(time.Second)` (or equivalent).** TTL stays `2 * windowSec`. Alternative: keep `float64(window)` after the reject — rejected; once only whole seconds pass, keys and formula must still share `windowSec` so a later Duration/second mismatch cannot return.

3. **Two error strings.** Keep `windowcounter: window must be at least one second` for `window < time.Second`. Remainder uses `windowcounter: window must be a whole number of seconds`. Tests assert non-nil only. Alternative: one string for both — rejected; "at least one second" is false for 1500ms (`skill:opd-commandments:Leave a trail`).

4. **Tests first.** Add `windowcounter/repro_fractional_window_test.go` `TestRepro_FractionalWindowAccepted` with subtest A as the lock (1500ms Take must error) against a fake Redis so dest today FAIL (Take succeeds). Subtest B is dest-fail diagnostic and is skipped when A’s error is set. Keep `TestTake_SubSecondWindow`. Alternative: assert Redis key rollover after the fix — rejected; explore resolved A as the contract.

5. **Usage gotcha.** `knowledge/devdocs/std_go_windowcounter.md` already says whole seconds. Add that Take/Peek reject a non-whole-second window; they do not truncate.

## Risks / Trade-offs

- [Callers who passed 1500ms and relied on silent 1s buckets start getting errors] → Mitigation: that path was already forbidden by spec; fail closed is the ticket.
- [Error string change for 500ms if someone later unifies messages] → Mitigation: keep the existing sub-second string; `TestTake_SubSecondWindow` only checks non-nil.

## Migration Plan

No signature change. Rollback is revert. Callers already required to pass whole-second windows.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
