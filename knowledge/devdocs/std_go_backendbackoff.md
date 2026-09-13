# Backend backoff

## Language

**Gate**:
An in-memory per-key admission gate. Package `backendbackoff`. Credit decides whether to trip. An exponential cooldown decides when to probe. It is not Traefik's CircuitBreaker middleware and not a token bucket.
_Avoid_: naming the package `circuitbreaker`; importing `tokenbucket`; reading HTTP inside the library

**Allow**:
Whether a real backend attempt for an opaque key may proceed, plus how long to wait if not. Denied requests are not backend attempts. The library does not sleep.
_Avoid_: HTTP 503 inside the library; Reporting a denied Allow

**Report**:
The boolean outcome of a real backend attempt the gate admitted. No context argument: a canceled request that already hit the backend still lands.
_Avoid_: Reporting gate denies; classifying status codes inside the library

**Credit**:
One saturating float per key. Starts and caps at `TripFailures` (`B`). Failure subtracts 1. Success adds `p/(1-p)`. Trip at `<= 0`. Independent of time.
_Avoid_: a sliding window of failures; an absolute failures-per-second bucket

## Overview

Import `github.com/david-garcia-garcia/traefik-middleware-utilities/backendbackoff`. Prefix keys in the caller. Store the Gate in a reclaim table the caller owns so a Traefik reload keeps the map. Pass `Close` as `reclaim.Hooks.Close`. No Sleep/Wake: there is no ticker. Do not import `tokenbucket`.

## How to use

- `New(Config{})` once (zeros apply defaults) or pass knobs. `FailureRatio` is `p`; `TripFailures` is how many consecutive failures trip a dead backend.
- Call `Allow(ctx, key)` per request. Pass `req.Context()` on the request path; pass `context.Background()` when there is no deadline. If not allowed, the caller may set `Retry-After` from the wait; the library does not.
- On an admitted attempt, call the backend, then `Report(key, success)`. Never Report a deny.
- Prove with `go test -short ./backendbackoff/...`. No live Redis/Dragonfly (see `knowledge/devdocs/std_go_test-suites.md`).

## Pattern snippet

```go
gate, err := backendbackoff.New(backendbackoff.Config{})
if err != nil {
	return err
}
allowed, retryAfter, err := gate.Allow(req.Context(), "backend:"+host)
if err != nil {
	return err
}
if !allowed {
	return errLimited
}
_ = retryAfter
ok := callBackend()
gate.Report("backend:"+host, ok)
```

## Key files

- `backendbackoff/` — in-memory backoff gate
- `openspec/specs/std_go_backendbackoff_allow/spec.md`, `openspec/specs/std_go_backendbackoff_cooldown/spec.md`

## Gotchas

- Report only real attempts the gate admitted. Reporting denies poisons credit and can strand OPEN.
- Report of an admitted attempt still records after idle TTL has elapsed; expire is Allow's job.
- After Close, Allow and Report return an error.
- Idle longer than TTL drops the key; the next Allow treats the backend as healthy.
- HALF-OPEN admits one probe. Concurrent Allows during that probe are denied.
- Two Gate instances do not share state. N Traefik replicas each burn `TripFailures` locally.
- `SetNowForTest` is tests only.
