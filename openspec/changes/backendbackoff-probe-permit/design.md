## Context

See proposal.md. Options on `devstate/explore.md`: generation, opaque token, lease id. In-tree there is no attempt identity today (`memEntry` has only `probeOutstanding` and `probeUntil`). Credit arithmetic lives only in CLOSED; HALF-OPEN with no outstanding probe is already a no-op.

## Goals / Non-Goals

**Goals:**

- A HALF-OPEN outcome is decided only by the report of the active probe.
- A report from an attempt admitted in an earlier era cannot trip, close, reopen, or change `n` or credit of the current era.
- Yaegi-safe: concrete struct and integers only, no generics, no `select`.

**Non-Goals:**

- Changing credit math, cooldown math, the probe lease, TTL, or the cap.
- Sharing state across replicas.
- Keeping the old `Allow`/`Report` signatures beside the new ones.

## Decisions

1. **One generation per entry, from a gate-wide counter.** `Gate` gets `seq uint64`; `memEntry` gets `gen uint64`. `gen = ++g.seq` when the entry is created and on every state transition: trip (CLOSED→OPEN), OPEN→HALF-OPEN in Allow, a replacement probe in HALF-OPEN after the lease expired, HALF-OPEN→CLOSED and HALF-OPEN→OPEN in Report. CLOSED admits do not bump it, so every permit of the current CLOSED era matches. Gate-wide (not per entry) so a permit from an expired or evicted entry never matches a recreated entry with the same key. Alternative: per-entry counter — rejected, a recreated entry restarts at the same values.

2. **`Permit` is a value struct, not a pointer or interface.** `type Permit struct { key string; gen uint64 }`, `func (p Permit) Allowed() bool { return p.gen != 0 }`. Generation 0 is never issued (`seq` pre-increments), so the zero value is "denied". Alternative: return `(bool, Permit, time.Duration, error)` — rejected, two values that must agree. Alternative: opaque `uint64` token plus key — rejected, the caller could pair a token with another key.

3. **`Report(p Permit, success bool)`.** Order of checks: closed gate → `errClosed`; `!p.Allowed()` → nil (denied Allows are ignored, as the spec already says); entry missing → nil; `entry.gen != p.gen` → nil, no TTL refresh. Then the existing per-state logic runs unchanged, with the transitions bumping `gen`. In HALF-OPEN a matching generation implies the probe is outstanding.

4. **Stale reports are a no-op, not CLOSED arithmetic.** A pre-trip failure that lands after the key re-closed must not count against the new era (it could trip a healthy backend). Explore question "CLOSED arithmetic or no-op" → no-op.

5. **Tests.** New self-contained `backendbackoff/zzz_probe_permit_test.go`, helpers suffixed `ProbePermit`: (a) the reproduced sequence — the real probe's failure reopens with `n+1` and the stale success is ignored; (b) expired lease — probe A, lease expires, probe B, A's late success is ignored, B's report decides; (c) a pre-trip failure reported after the gate re-closed does not change credit; (d) permit of an expired-and-recreated key is ignored; (e) zero permit Report is ignored. Migrate all existing tests, benches (`TestAllocAllowWarm` ceiling must hold or be re-recorded with evidence), and the Yaegi probe.

## Risks / Trade-offs

- [Breaking API for importers outside this repo] → Owner accepts breaking changes when they are the clean option; no legacy methods.
- [`Permit` holds a string, so returning it may allocate or grow the warm path] → The key string is the caller's; storing it copies a header, not bytes. Measure `TestAllocAllowWarm` compiled and Yaegi.
- [A caller that drops a permit and never reports] → Same as today: the probe lease expires and the next Allow takes the slot, with a new generation.

## Migration Plan

Library API change. Callers replace `ok, wait, err := g.Allow(k)` with `p, wait, err := g.Allow(k)`, `ok := p.Allowed()`, and `g.Report(k, success)` with `g.Report(p, success)`. Rollback is revert.

## Open Questions

None.
