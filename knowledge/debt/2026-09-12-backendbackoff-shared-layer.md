# Distributed backendbackoff layer

IssueKey: 2026-09-12-backendbackoff
Size: large
Action: note

## Why this follow-up
A shared layer so one Traefik instance can learn that a backend is unhealthy from its peers, instead of every replica independently burning `B` failures into that backend.

## Why it was not taken
The caller spec deferred it from this change to keep the first version in-memory only. The exploration concluded it must be strictly additive: local state stays authoritative and always answers, the shared layer only accelerates something the instance would have observed itself, and no Redis I/O happens on the request path. That is a new store contract, not a knob on this Gate.

## Risks
Until a shared layer exists, N replicas each admit `B` failures and each run their own jittered cooldowns. They do not share OPEN. That is the intended first-version tradeoff, not a silent gap.

## Context
Do not add a `simpleredis` dependency to `backendbackoff/`. Existing `windowcounter` / `tokenbucket` Redis paths propagate store errors to the caller; they are not precedent for surviving a store outage. `knowledge/debt/2026-09-12-simpleredis-dial-circuit-breaker.md` records that request-path store I/O during an outage becomes a latency outage — the inverse of a health gate's purpose.
