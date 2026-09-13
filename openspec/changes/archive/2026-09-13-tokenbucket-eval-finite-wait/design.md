## Context

`Redis.Allow` already maps a 3-field Eval reply: length ≠ 3 is `errEvalLen`; `ParseFloat` error on wait is `errEvalWait`; then `waitDuration` + `allowedFromWait`. See proposal.md for why. Proceed policies: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- One finite check on the parsed wait, same sentinel as `"xyz"`.
- Tests exist and fail on dest before the production hunk.

**Non-Goals:**
- Bug 1 microsecond admit/refund.
- `Memory.Allow`, Lua `allowScript`, a second error type, deny-with-nil.

## Decisions

1. **Finite after ParseFloat, same `errEvalWait`.** `math.IsNaN(waitMicro) || math.IsInf(waitMicro, 0)` then `(false, 0, errEvalWait)`. Alternative: deny with `err=nil` — rejected; ticket forbids it. Alternative: a new error — rejected; dest already owns wait-is-not-a-number.

2. **Do not use Duration or micro compare to reject Inf/NaN.** `-Inf <= maxDelayMicro` is true; NaN/`+Inf` can become deny with `err=nil`. Alternative: wait for bug 1 — rejected; it does not close `-Inf`.

3. **Tests first in `repro_eval_nan_wait_test.go`.** Adapt dest `Allow(ctx, key) (bool, time.Duration, error)`. Reuse `startTestFakeRedis` / `setEvalReply` / `arrayBulks` / `newSimpleRedisForTest`. Assert `errors.Is(err, errEvalWait)` and wait `0`. Alternative: only extend `TestRedis_EvalBadReply` — the ticket named this file.

## Risks / Trade-offs

- [Live Lua never emits nan/inf] → Mitigation: the check still runs; proof is the fake 3-field override (`explore.md`).
- [Bug 1 later deletes `allowedFromWait`] → Mitigation: this check sits on the parsed float, before Duration mapping.

## Migration Plan

Library error path only. Rollback is revert. No deploy key.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
