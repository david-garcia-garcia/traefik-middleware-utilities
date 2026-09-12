# Explore
IssueKey: 2026-09-12-backendbackoff

## Concepts

```
  CLOSED --credit<=0--> OPEN --cooldown elapsed--> HALF-OPEN
     ^                      ^                         |  |
     |                      |              probe fail |  | probe ok
     |                      +----------- n+1 ---------+  |
     +-------- credit=B, n retained -----------------------+
     n resets after one MaxCooldown of continuous CLOSED
```

- **Gate** (`backendbackoff.Gate`): per-key in-memory admission. Not Traefik's expression CircuitBreaker (`vulcand/oxy` `cbreaker`), not `tokenbucket.Memory` (time-refill consume).
- **Allow**: admit or deny one request on an opaque key. Denied requests are not backend attempts.
- **Report**: record the boolean outcome of a real backend attempt the gate admitted (including the HALF-OPEN probe).
- **Credit**: saturating `float64`, start/cap `B`. Failure `-= 1`. Success `+= p/(1-p)` capped at `B`. Trip `<= 0`. Independent of time.
- **Cooldown**: `base·2^n + jitter`, capped at `max`. Independent of outcomes. Jitter desyncs replicas that trip together.
- **Key**: caller-owned opaque string. The library does not read HTTP, client address, user, tenant, or Host. Same owner as `tokenbucket.Allow` / `windowcounter.Take`.
- **Map**: `tokenbucket/memory.go` shape (`maxMemorySources` 65536, `dropExpired`, `dropOne`) copied, not imported. Idle longer than TTL drops the key; next sight is a healthy backend (full credit, `n` 0, CLOSED).
- **Reclaim**: store `*Gate` via existing `reclaim.Open`. No ticker → no `Sleep`/`Wake`. `Close` at most.
- **Yaegi**: stdlib only, concrete types. Prove with `go test -short` plus `limiter_yaegi_test.go` (GOPATH interp, stdlib only, `useunsafe` false). `TestAlloc*` on the warm Allow path. No `*_LIVE_*`, no e2e-redis/dragonfly, no Pester plugin (Yaegi interp is the interpreted proof, as in `tokenbucket/limiter_yaegi_test.go`).

Traefik CircuitBreaker (docs, not a library we wrap): expression default empty; example `NetworkErrorRatio() > 0.30`; `checkPeriod` 100ms; `fallbackDuration` 10s; `recoveryDuration` 10s. Recovering there is linearly increasing traffic; our HALF-OPEN is one probe. Do not copy oxy's recovering. Tokenbucket `ttl` floor is 1s (`tokenbucket/clock.go`); Traefik RateLimit in-memory source TTL is 2s when rate ≥ 1 — that is a rate-limit source expiry, too short here (it would drop credit between sparse failures and defeat `B`).

## Decisions

- Trip criterion, two clocks, `n` retain/reset, Report contract, in-memory only, no `tokenbucket` import: settled on the requirement. Do not re-open.
- Allow refreshes `expireAt` on every call, including denies, matching `tokenbucket/memory.go` `entry.expireAt = now.Add(ttl)`. Otherwise an OPEN key under denied hammering expires and the next Allow would admit as a new CLOSED key.
- `Allow(ctx, key) (bool, time.Duration, error)`: `ctx.Err()` before admit (canceled request is not a probe). Wait is `retryAfter` (0 when admitted). Error is `ctx.Err()` or closed.
- `Report(key string, success bool)`: no context. A canceled request that already hit the backend must still land. OPEN with no outstanding probe: ignore. CLOSED: apply credit (may trip). HALF-OPEN: probe outcome (success → CLOSED, credit `B`, `n` retained; failure → OPEN, `n+1`).
- HALF-OPEN admits one outstanding probe. Concurrent Allows deny. If the probe is not Reported within one `BaseCooldown` after entering HALF-OPEN, the next Allow may take the probe slot (lost Report). Idle TTL remains the last resort.
- No exported Snapshot / State getter. Observation is the Allow return (admitted + retryAfter) at decision time. A stale read API would invite callers to skip Allow.
- `maxMemorySources = 65536` duplicated as an unexported const (Traefik ttlmap / `tokenbucket/memory.go`). Do not import `tokenbucket`.
- `SetNowForTest` on `Gate`, same job as `tokenbucket.Memory.SetNowForTest`. Jitter 0 is legal and is how unit tests freeze cooldowns.
- No Pester plugin and no Go E2E / `*_LIVE_*`. Reclaim has the same shape (in-memory, no store).
- Distributed / Redis layer: not this change. Follow-up note: `knowledge/debt/2026-09-12-backendbackoff-shared-layer.md`.
- Usage packet and `std_go_backendbackoff` specs: propose writes them; explore does not invent Language before the exported names land.

## Open questions

- Q: What defaults for `p`, `B`, base cooldown, max cooldown, jitter fraction, and idle TTL?
  Rank: additive asked — requirement Unknowns names these knobs; new Config this change creates
  Decision: assumed — `FailureRatio` `p=0.30` (Traefik CircuitBreaker docs example `NetworkErrorRatio() > 0.30`); `TripFailures` `B=5` (requirement illustration: consecutive failures that trip a dead backend); `BaseCooldown=1s` (first OPEN wait, so exponential has steps before the cap); `MaxCooldown=10s` (Traefik `fallbackDuration` default); `Jitter=0.10` (cooldown × (1 + jitter×(2u−1)), `u∈[0,1)`); `TTL=60s` (idle drop; must outlive sparse traffic so `B` can accumulate; tokenbucket/Traefik 2s source TTL would reset credit between slow failures). Zero `Config` fields apply these. Construction fails when `p` not in (0,1), `B<1`, `BaseCooldown<=0`, `MaxCooldown < BaseCooldown`, `Jitter` not in [0,1), `TTL < 1s`.
  By: explore

- Q: Does Allow return a retryAfter duration beside the boolean?
  Rank: additive asked — requirement Unknowns; inclination yes
  Decision: assumed — yes. `Allow` returns `(allowed, retryAfter, err)` mirroring `tokenbucket.Allow`. `retryAfter` is remaining OPEN cooldown, or remaining HALF-OPEN probe lease when a probe is outstanding; 0 when admitted. The library does not set `Retry-After` and does not sleep.
  By: explore

- Q: Exported names for the knobs and the gate type?
  Rank: additive asked — requirement Unknowns; new identifiers this change creates
  Decision: assumed — type `Gate`; `New(Config) (*Gate, error)`; knobs `FailureRatio` (`p`), `TripFailures` (`B`, consecutive failures that trip a dead backend), `BaseCooldown`, `MaxCooldown`, `Jitter`, `TTL`; methods `Allow`, `Report`, `Close`. Package `backendbackoff`. Not `CircuitBreaker` (Traefik's HTTP middleware name).
  By: explore

- Q: Does the gate expose current state for observability?
  Rank: additive asked — requirement Unknowns
  Decision: assumed — no. Callers observe via Allow's `(allowed, retryAfter)`. A Snapshot/State getter would be a stale read and would invite skipping Allow. Metrics stay the caller's job.
  By: explore

- Q: Who already owns the admission key (client address, user, tenant, Host, trust hop)?
  Rank: additive asked — requirement Desired: library never reads HTTP; tokenbucket/windowcounter already assign key ownership to the caller (`openspec/specs/std_go_tokenbucket_allow/spec.md` Caller owns the key)
  Decision: resolved — the caller. Pass an opaque key; prefix in the caller when sharing `reclaim.Default`. The library does not reconstruct identity.
  By: explore

- Q: What if a HALF-OPEN probe is never Reported?
  Rank: additive asked — requirement Desired: HALF-OPEN admits a probe; no lost-probe rule written
  Decision: assumed — at most one outstanding probe. Concurrent Allows deny with `retryAfter` = remaining probe lease (`BaseCooldown` from HALF-OPEN entry). If that lease elapses with no Report, the next Allow may probe. Idle TTL drop is the last recovery (next sight is CLOSED).
  By: explore
