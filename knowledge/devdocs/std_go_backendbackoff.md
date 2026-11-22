# Backend backoff

## Language

**Gate**:
An in-memory per-key admission gate. Package `backendbackoff`. Credit decides whether to trip. An exponential cooldown decides when to probe. It is not Traefik's CircuitBreaker middleware and not a token bucket.
_Avoid_: naming the package `circuitbreaker`; importing `tokenbucket`; reading HTTP inside the library

**Allow**:
Whether a real backend attempt for an opaque key may proceed, plus how long to wait if not. An admit returns a `Permit`. Denied requests are not backend attempts and return the zero `Permit`. The library does not sleep and does not read `context.Context`; the caller must not invoke Allow after the request context is canceled or past deadline.
_Avoid_: HTTP 503 inside the library; Reporting a denied Allow; passing `req.Context()` into Allow

**Permit**:
The identity of one admitted Allow: the key plus that key's generation at admit. `Allowed()` is false on the zero value.
_Avoid_: reconstructing it from HTTP; pairing a generation with a different key

**Report**:
The boolean outcome of a real backend attempt the gate admitted, identified by the `Permit` Allow returned. A report whose generation does not match the key's current generation is ignored (no credit, state, `n`, or TTL change). No context argument: a canceled request that already hit the backend still lands.
_Avoid_: Reporting gate denies; classifying status codes inside the library

**Credit**:
One saturating float per key. Starts and caps at `TripFailures` (`B`). Failure subtracts 1. Success adds `p/(1-p)`. Trip at `<= 0`. Independent of time.
_Avoid_: a sliding window of failures; an absolute failures-per-second bucket

## Overview

Import `github.com/david-garcia-garcia/traefik-middleware-utilities/backendbackoff`. Prefix keys in the caller. Store the Gate in a reclaim table the caller owns so a Traefik reload keeps the map. Pass `Close` as `reclaim.Hooks.Close`. No Sleep/Wake: there is no ticker. Do not import `tokenbucket`.

## How to use

- `New(Config{})` once (zeros apply defaults) or pass knobs. `FailureRatio` is `p`; `TripFailures` is how many consecutive failures trip a dead backend.
- Before each request, if `req.Context()` is already done, skip Allow and the backend call. Otherwise call `Allow(key)`. If `!p.Allowed()`, the caller may set `Retry-After` from the wait; the library does not.
- On an admitted attempt, call the backend, then `Report(p, success)`. Never Report a deny. A report from another generation is ignored.
- Prove with `go test -short ./backendbackoff/...`. No live Redis/Dragonfly (see `knowledge/devdocs/std_go_test-suites.md`).

## Pattern snippet

```go
gate, err := backendbackoff.New(backendbackoff.Config{})
if err != nil {
	return err
}
if err := req.Context().Err(); err != nil {
	return err
}
p, retryAfter, err := gate.Allow("backend:"+host)
if err != nil {
	return err
}
if !p.Allowed() {
	return errLimited
}
_ = retryAfter
ok := callBackend()
gate.Report(p, ok)
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
