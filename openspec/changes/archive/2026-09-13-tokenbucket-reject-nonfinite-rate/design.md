## Context

`validateClock` is the shared New gate (`tokenbucket/clock.go`). Dest checks `rate <= 0` only. IEEE: `NaN <= 0` is false; `+Inf > 0` is true; `-Inf <= 0` is already true. See proposal.md for why. Proceed policies: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- One constructor predicate that rejects non-finite rate as `errRate`.
- Tests fail on dest for NaN and +Inf, then pass after the gate.
- Usage gotcha names non-finite rate, not only `rate <= 0`.

**Non-Goals:**
- Special-casing `Allow` or `consumeOne` for NaN tokens.
- Treating +Inf as unlimited / adding an unlimited-rate constructor.
- Changing Allow wait mapping, Eval wait, ttl, Lua fill, or last rewind.
- Renaming `errRate` or its string.

## Decisions

1. **Reject at `validateClock`, not in Allow.** Both constructors already call it. Alternative: NaN-token checks in `consumeOne` — rejected; ticket forbids papering over a constructor that should have failed.

2. **Predicate is `rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0)`.** `IsInf(rate, 0)` covers +Inf and -Inf. `-Inf` already fails `<= 0`; keep `IsInf` so +Inf cannot pass if the comparison is later narrowed. Alternative: `!math.IsInf && rate > 0` without `IsNaN` — rejected; NaN still compares false.

3. **Keep `errRate` string.** Callers match `errors.Is`. Alternative: mention "finite" in the message — rejected; explore assumed keep the string.

4. **Named repro files, dest Allow signature.** `repro_nan_rate_test.go` (`TestRepro_NaNRateRejected`, `TestRepro_NaNRateAllowFailOpen` Skip-after-fix) and `repro_inf_rate_test.go` (`TestRepro_InfRateRejected`, including -Inf as a dest-green case). Drop Inf-always-allow. Tests MUST use `Allow(ctx, key) (bool, time.Duration, error)`. Alternative: fold into `limiter_test.go` — rejected; ticket copy/adapt names those files. Review renamed the Inf file so `hunt`/`nan` do not hide the constructor-reject job.

5. **Tests first.** Write the tests, `go test -short ./tokenbucket` FAIL, then change `validateClock`, then PASS. Do not weaken assertions to pass on dest.

## Risks / Trade-offs

- [A caller currently passing +Inf as "unlimited" starts getting `errRate`] → Mitigation: this package has no unlimited constructor; passthrough is skipping New. That break is the fix.
- [`TestRepro_NaNRateAllowFailOpen` stays as Skip after the fix] → Mitigation: `TestRepro_NaNRateRejected` is the New-reject proof; Skip documents dest fail-open is unreachable.

## Migration Plan

Library constructor validation only. Rollback is revert. No deploy key.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
