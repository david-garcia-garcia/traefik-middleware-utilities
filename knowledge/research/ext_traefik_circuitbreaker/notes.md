# CircuitBreaker middleware

Facts about Traefik’s first-party HTTP CircuitBreaker middleware: published defaults, trip expression, and Closed/Open/Recovering. Not this product’s `backendbackoff` package. Not a library this product imports. Pin: [traefik/traefik@5b53bae](https://github.com/traefik/traefik/tree/5b53bae42d2dab453b8f932db760b874f99ee984) (v3.3.0). Implementation is [vulcand/oxy/v2@v2.0.0](https://github.com/vulcand/oxy/tree/07821e22d8655dcce7d23d1038a8d325e5dc234b) (`07821e22`).

## Defaults (official)

Docs table ([CircuitBreaker docs](https://doc.traefik.io/traefik/reference/routing-configuration/http/middlewares/circuitbreaker/), [.sources/circuitbreaker-docs.md](.sources/circuitbreaker-docs.md)):

| Field | Default | Required |
| --- | --- | --- |
| `expression` | `""` | No |
| `checkPeriod` | `100ms` | No |
| `fallbackDuration` | `10s` | No |
| `recoveryDuration` | `10s` | No |
| `responseCode` | `503` | No |

Example trip condition in the metrics table: `NetworkErrorRatio() > 0.30`. Config examples use `LatencyAtQuantileMS(50.0) > 100`. Fallback while open is HTTP 503 unless `responseCode` is set. Each router gets its own instance (state is not shared). The breaker only sees what happens **after** its position in the middleware chain.

`SetDefaults` on the Traefik config struct matches the duration and status defaults: CheckPeriod 100ms, FallbackDuration 10s, RecoveryDuration 10s, ResponseCode 503. It does **not** fill `Expression`. ([traefik@5b53bae:pkg/config/dynamic/middlewares.go](https://github.com/traefik/traefik/blob/5b53bae42d2dab453b8f932db760b874f99ee984/pkg/config/dynamic/middlewares.go), [.sources/middlewares.go.md](.sources/middlewares.go.md))

## Wrapper (durations applied only when > 0)

`circuit_breaker.go` always installs an oxy Fallback that writes `responseCode` plus `http.StatusText(responseCode)` as the body. CheckPeriod / FallbackDuration / RecoveryDuration are passed to oxy **only when the config duration is > 0**. Empty `Expression` is passed through to `cbreaker.New`. ([traefik@5b53bae:pkg/middlewares/circuitbreaker/circuit_breaker.go](https://github.com/traefik/traefik/blob/5b53bae42d2dab453b8f932db760b874f99ee984/pkg/middlewares/circuitbreaker/circuit_breaker.go), [.sources/circuit_breaker.go.md](.sources/circuit_breaker.go.md))

When those durations stay 0 (SetDefaults not applied), oxy `New` still uses 100ms / 10s / 10s — same published numbers. ([oxy@07821e22:cbreaker/cbreaker.go](https://github.com/vulcand/oxy/blob/07821e22d8655dcce7d23d1038a8d325e5dc234b/cbreaker/cbreaker.go), [.sources/cbreaker.go.md](.sources/cbreaker.go.md))

**Conflict (empty `expression`):** official lists default `""` and Required: No. oxy `New` parses the string as a Go predicate; `parser.ParseExpr("")` fails, so Traefik `New` returns that error. Follow **source** for this pin: a running CircuitBreaker needs a parseable expression. Official disagrees on “usable with empty.” ([oxy@07821e22:cbreaker/predicates.go](https://github.com/vulcand/oxy/blob/07821e22d8655dcce7d23d1038a8d325e5dc234b/cbreaker/predicates.go), [.sources/predicates.go.md](.sources/predicates.go.md); [vulcand/predicate@18a87524:parse.go](https://github.com/vulcand/predicate/blob/18a87524aab1abdcd4b908ec2cebdbc611692fee/parse.go), [.sources/parse.go.md](.sources/parse.go.md))

## Expression

Three metric functions, combinable with `&&` / `||`, compared with `>`, `>=`, `<`, `<=`, `==`, `!=`. ([CircuitBreaker docs](https://doc.traefik.io/traefik/reference/routing-configuration/http/middlewares/circuitbreaker/), [.sources/circuitbreaker-docs.md](.sources/circuitbreaker-docs.md); same names in [.sources/predicates.go.md](.sources/predicates.go.md))

- `NetworkErrorRatio()` — docs: network-error ratio. Source: `count(502)+count(504)` over total recorded requests; 0 when total is 0. ([oxy@07821e22:memmetrics/roundtrip.go](https://github.com/vulcand/oxy/blob/07821e22d8655dcce7d23d1038a8d325e5dc234b/memmetrics/roundtrip.go), [.sources/roundtrip.go.md](.sources/roundtrip.go.md))
- `ResponseCodeRatio(from, to, dividedByFrom, dividedByTo)` — `sum(from inclusive → to exclusive) / sum(dividedByFrom → dividedByTo)`. Denominator 0 → 0. Example: `ResponseCodeRatio(500, 600, 0, 600) > 0.25`.
- `LatencyAtQuantileMS(q)` — quantile **must** be a float literal with trailing `.0` (`50.0`, not `50`) because the parser types `50` as int. Returns milliseconds at that quantile; missing quantile → 0 (does not trip `>`).

## Closed / Open / Recovering

Docs names vs oxy names (same machine): Closed = standby, Open = tripped, Recovering = recovering. Zero-value state is standby (Closed). ([CircuitBreaker docs](https://doc.traefik.io/traefik/reference/routing-configuration/http/middlewares/circuitbreaker/); [oxy@07821e22:cbreaker/cbreaker.go](https://github.com/vulcand/oxy/blob/07821e22d8655dcce7d23d1038a8d325e5dc234b/cbreaker/cbreaker.go), [.sources/cbreaker.go.md](.sources/cbreaker.go.md))

- **Closed:** every request goes to the next handler. After each response, `checkAndSet` evaluates `expression` at most once per `checkPeriod`. Match → Open, metrics reset, `until = now + fallbackDuration`.
- **Open:** fallback for all requests until `fallbackDuration`. Then Recovering (`until = now + recoveryDuration`).
- **Recovering:** a linear ratio controller admits some requests to next; the rest get fallback. If `expression` matches on a passed request’s check, back to Open. If `now` is after `until` without a re-trip, Closed. Time expiry (not a successful re-check of the expression) is what closes.

**Recovery ramp (source, not in the docs table):** `allowedRequestsRatio = 0.5 * (Now - StartRecovery) / RecoveryDuration`. The 0.5 cap is equilibrium (`allowed == denied` ⇒ later requests all pass). Official only says “linearly increasing amounts.” Follow **source** for the formula. ([oxy@07821e22:cbreaker/ratio.go](https://github.com/vulcand/oxy/blob/07821e22d8655dcce7d23d1038a8d325e5dc234b/cbreaker/ratio.go), [.sources/ratio.go.md](.sources/ratio.go.md))
