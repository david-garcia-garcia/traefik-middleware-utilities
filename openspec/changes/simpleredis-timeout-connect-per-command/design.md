## Context

Dest already destroys a timed-out socket and dials on idle miss. Package constraints: stdlib only, Yaegi-safe, no extra Config surface, no background reaper (existing idle-reaper debt). Sibling agents own hunks in `commands_exec.go`, `pool.go`, and `resp.go`. See proposal.md for why. Proceed policies: `devstate/explore.md`.

## Goals / Non-Goals

**Goals:**
- Make the per-command scope of the timeout second-connection rule explicit on `std_go_simpleredis_tcp-session`.
- Write the operator composition on `knowledge/devdocs/std_go_simpleredis.md` (timeout → connect-per-command, rates, AUTH/SELECT tax, `IOTimeout` lever, `maxclients` as simultaneous).

**Non-Goals:**
- Any `simpleredis/*.go` edit.
- Circuit breaker, dial-rate limiter, cooldown timestamp, background drain, retry-on-timeout.
- Changing `defaultIOTimeout`.
- A default-suite test that bounds dials under a slow peer.
- Taking `knowledge/debt/2026-09-12-simpleredis-dial-circuit-breaker.md`.

## Decisions

1. **Document, do not govern dial rate.** Explore measured connect-per-command (15 dials / 15 commands; 9.9 dials/s at default 100ms; peak server open `2×PoolSize` concurrent). Alternatives: breaker / limiter / cooldown — rejected; new failure-memory, Config, and Yaegi state. Alternative: raise default `IOTimeout` — rejected this run; behaviour change for every zero-Config caller.

2. **ADDED spec, not MODIFIED of the I/O deadline requirement.** The existing SHALL text stays. A new requirement states that MUST-NOT-second-connection is per timed-out command, and the later command follows idle-empty dial. Alternative: rewrite the I/O requirement in place — rejected; archive would risk dropping the Yaegi `net.Error` and mapping sentences. Alternative: `skip_specs` — rejected; the current wording is what made the storm look like a spec defect.

3. **Usage gotcha carries the numbers.** Spec scenarios stay qualitative (timeout not retried; later command dials). Sequential 10 dials/s, overlap `2×PoolSize`, AUTH=N SELECT=N, Redis `maxclients` default 10000 live on the usage packet. Alternative: spec the 15-dial count as SHALL — rejected; that would forbid a later limiter the human might still commission.

4. **Exact gotcha text** (one bullet under `## Gotchas` on `knowledge/devdocs/std_go_simpleredis.md`, after the retry bullet):

   A command that hits `IOTimeout` destroys the socket (the reply is still outstanding; the socket is not on a reply boundary). The next independent command finds idle empty and dials. Sequential timed-out commands therefore run connect-per-command: about `1/IOTimeout` dials per second per goroutine (10/s at the 100ms default). Concurrent callers stay capped at `PoolSize` in-flight; Redis-side peak is about `2×PoolSize` per client while the server is still answering the timed-out command. `Pass` / `Database` add AUTH and SELECT on every one of those reconnects (or AUTH alone if the handshake is also slow). Retry backoff does not run: `redis:timeout` is never retried. Raise `IOTimeout` when a loaded Redis p99 exceeds 100ms (the probe already uses 1s for a 500ms Eval). Size `PoolSize` × instance count against Redis `maxclients` (default 10000, simultaneous, not historical). Do not expect this client to circuit-break or rate-limit dials. Changing the package default 100ms is a behaviour change for every zero-Config caller (hung-peer wait is `(MaxRetries+1)*(DialTimeout+IOTimeout)`).

5. **No product Go diff.** Sibling agents are editing the same session files. A docs-only apply cannot collide with those hunks.

## Risks / Trade-offs

- [Readers treat the new spec as requiring unbounded churn forever] → Mitigation: session is "not required to rate-limit", not "MUST NOT rate-limit". A later change can add a limiter by MODIFIED.
- [Operators still hit 100ms p99] → Mitigation: gotcha names `IOTimeout` as the lever; raising the package default stays a human decision.
- [Docs-only leaves TIME_WAIT in production] → Accepted. The simplicity gate prefers that over a breaker.

## Migration Plan

Usage packet plus spec delta. Rollback is revert. No stored data change. No session binary change.

## Open Questions

None. Proceed policies live on `devstate/explore.md`.
