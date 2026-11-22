# Backend backoff

## Language

**Gate**:
An in-memory per-key admission gate. Package `backendbackoff`. Credit decides whether to trip. An exponential cooldown decides when to probe. One map entry per opaque key, cap 65536, expiry-ordered list. It is not Traefik's CircuitBreaker middleware and not a token bucket.
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

Import `github.com/david-garcia-garcia/traefik-middleware-utilities/backendbackoff`. Prefix keys in the caller. Store the Gate in a reclaim table the caller owns so a Traefik reload keeps the map. Pass `Close` as `reclaim.Hooks.Close`. No Sleep/Wake: there is no ticker. The map holds at most 65536 keys in an expiry-ordered list, so a new key is O(1). Do not import `tokenbucket`.

## How to use

- `New(Config{})` once (zeros apply defaults) or pass knobs. `FailureRatio` is `p`; `TripFailures` is how many consecutive failures trip a dead backend.
- Before each request, if `req.Context()` is already done, skip Allow and the backend call. Otherwise call `Allow(key)`. If `!p.Allowed()`, the caller may set `Retry-After` from the wait; the library does not.
- On an admitted attempt, call the backend, then `Report(p, success)`. Never Report a deny. A report from another generation is ignored and `Report` returns `applied` false (use it, not your own flag, to log that a probe closed the gate).
- Prove with `go test -short ./backendbackoff/...`. No live Redis/Dragonfly (see `knowledge/devdocs/std_go_test-suites.md`).
- Benchmark new-key admission with `go test -run '^$' -bench 'BenchmarkAllow/new-key-full-table' ./backendbackoff/` (compiled) and `-bench 'BenchmarkYaegiAllow/new-key-full-table'` (interpreted; writes a test-only `zz_bench_hooks.go` into the interpreter GOPATH copy). CI does not run benchmarks.

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
_, _ = gate.Report(p, ok)
```

## Key files

- `backendbackoff/` — in-memory backoff gate
- `backendbackoff/zzz_bench_test.go`, `zzz_bench_yaegi_test.go` — compiled and interpreted new-key-at-full-table benches
- `openspec/specs/std_go_backendbackoff_allow/spec.md`, `openspec/specs/std_go_backendbackoff_cooldown/spec.md`

## Gotchas

- Report only real attempts the gate admitted. Denied and zero permits are ignored.
- Report of an admitted attempt still records after idle TTL has elapsed; expire is Allow's job.
- After Close, Allow and Report return an error.
- Idle longer than TTL drops the key; the next Allow treats the backend as healthy.
- HALF-OPEN admits one probe. Concurrent Allows during that probe are denied.
- Two Gate instances do not share state. N Traefik replicas each burn `TripFailures` locally.
- When `Jitter` is greater than 0, OPEN waits are uniform in `[wait*(1-Jitter), min(wait*(1+Jitter), MaxCooldown))`; they never pile on `MaxCooldown`. `Jitter` 0 returns `wait` exactly.
- `SetNowForTest` is tests only.
- The Gate holds at most 65536 keys. A new key first removes keys whose TTL elapsed, then one live key only if the store is still at the cap. Expired keys leave even below the cap. Which live key goes is not specified (the code removes the expiry-list head). A removed key comes back as a healthy CLOSED key with full credit. Keys sit in an expiry-ordered list, so a new key costs O(1), not a map walk. A test clock that moves backwards can leave an expired key behind the head until it is looked up; at the cap that can remove a live key while the expired one stays stored. `time.Now` does not move backwards, so production keeps expired-first.
- Every Allow (including denies) and every Report that lands writes `expireAt = now + TTL` and moves the key to the tail. `moveToBackLocked` relinks in place instead of calling `unlinkLocked` then `pushBackLocked`, because each interpreted method call costs microseconds under Yaegi. Keep that path free of extra calls.
