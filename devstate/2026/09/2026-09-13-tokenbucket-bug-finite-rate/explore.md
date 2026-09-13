# Explore
## Concepts

Token-bucket `rate` is requests per second on `clockConfig`. `validateClock` is the only owner of that gate; `NewMemory` and `NewRedis` both call it before storing the clock. There is no unlimited-rate constructor. Passthrough is skipping `New` (Traefik `rate.Inf` is that skip, not a limiter).

IEEE: `math.NaN() <= 0` is false; `math.Inf(1) > 0` is true; `math.Inf(-1) <= 0` is true. Dest `validateClock` is `if rate <= 0 { return errRate }` (`tokenbucket/clock.go`). It imports no `math`.

`consumeOne` refills with `tokens + limitPerMicro*elapsed`. At elapsed=0, `Inf*0` is NaN. `tokens < 0` is false for NaN, so wait stays 0 and `allowedFromWait(0, 0)` admits. NaN rate makes `limitPerMicro` NaN; `tokens < 0` never holds; every Allow admits.

Usage `knowledge/devdocs/std_go_tokenbucket.md` gotcha: `rate <= 0` fails New. Spec `std_go_tokenbucket_allow` construction SHALL fail when `rate <= 0`; scenario names 0 or a negative rate. Ticket wins; propose must widen that contract to non-finite.

```
rate
  ├─ <= 0          → errRate (dest already)
  ├─ NaN           → dest accepts → Allow always true
  ├─ +Inf          → dest accepts → second Allow elapsed=0 Inf*0=NaN fail-open
  └─ finite > 0   → constructor succeeds
```

## Decisions

- Fix only `validateClock`: `rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0)` returns `errRate`. Do not special-case `Allow` or `consumeOne`. Do not treat +Inf as unlimited.
- Keep sentinel `errRate` and its existing string. Ticket names `errRate`, not a new type or copy change.
- Tests first in named repro files the ticket named, adapted to dest `Allow(ctx, key) (bool, time.Duration, error)`. Drop `TestRepro_NaNRateInfAllow` (Inf-always-allow). After the fix, +Inf `New` must error.
- `-Inf` is already `errRate` on dest via `<= 0`. `IsInf(rate, 0)` still belongs so +Inf cannot sneak past if someone later narrows the comparison. Include `-Inf` on the constructor table; it is green-on-dest; +Inf and NaN are the fail-then-pass cases.
- Identity: callers pass an opaque key. This change does not set or reconstruct client address, user, tenant, or Host.
- No third-party research write: the gap is Go IEEE comparison in this package, not Redis/Traefik script behavior.

## Open questions

- Q: Do new tests land as the named `repro_*.go` files or fold into `limiter_test.go`?
  Rank: additive asked — requirement Desired names copy/adapt `repro_nan_rate_test.go` and `repro_hunt_inf_rate_nan_test.go`
  Decision: assumed — land `repro_nan_rate_test.go`; Inf constructor tests as `repro_inf_rate_test.go` after review renamed the hunt filename. Adapt dest Allow signature; do not fold into `limiter_test.go`.
  By: codereview

- Q: Does `-Inf` get its own constructor test, given dest already rejects it via `rate <= 0`?
  Rank: additive incidental — ticket names `math.IsInf(rate, 0)`; dest `-Inf <= 0` already returns `errRate` (measured)
  Decision: assumed — include `-Inf` as a constructor-reject case beside +Inf; do not treat dest-green `-Inf` as the fail-first proof. NaN and +Inf are the tests that must fail on dest.
  By: explore

- Q: Does `TestRepro_NaNRateAllowFailOpen` stay as a skip-after-fix witness or become New-reject only?
  Rank: additive asked — requirement Desired names that test; after the fix New rejects so Allow fail-open is not reachable
  Decision: assumed — keep the test; Skip when New rejects NaN. `TestRepro_NaNRateRejected` is the New-reject proof. Do not rewrite Allow fail-open into a second New-reject test.
  By: explore

- Q: Should `errRate` text change from "greater than 0" to mention finite?
  Rank: additive incidental — ticket returns `errRate`; no criterion names the string
  Decision: assumed — keep the existing string. Callers match `errors.Is(..., errRate)`. Spec/usage describe finite without renaming the sentinel.
  By: explore
