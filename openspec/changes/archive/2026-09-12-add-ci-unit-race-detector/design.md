## Context

See proposal.md Why. Dest CI already has four jobs. Unit is `go test -short -timeout 2m -count=1 -v ./...` (`.github/workflows/ci.yml` line 38). E2E is live engines at 5m without `-race`. Ubuntu runners have gcc; this Windows workspace does not.

Explore decisions: unit job takes `-race` and 10m; e2e stays plain; reuse dest concurrent fake-TCP tests.

## Goals / Non-Goals

**Goals:**
- One Ubuntu `go test` that compiles SimpleRedis runs under the race detector on every PR.
- Timeout high enough that Yaegi unit plus `-race` does not look like a hang.

**Non-Goals:**
- Fifth CI job. `-race` on e2e. New concurrent tests. Fuzz. Pool runtime fixes. Windows/macOS CI.

## Decisions

1. **Flag on `test`, not `e2e`, not a fifth job.**
   - Why: criterion is one Ubuntu detector leg. Unit already compiles the pool and the concurrent canaries under `-short`. E2e remaining plain is the ticket's non-race fallback. A fifth job would rewrite the four-suite pin.
   - Alternative: `-race` on both Go jobs — doubles wall time and races live engines. Rejected.
   - Alternative: new `race` job — fifth suite. Rejected.

2. **Timeout 10m on the unit step only.**
   - Why: ticket example; dest unit cap is 2m; `-race` is 2–10× slower; Yaegi interprets packages.
   - Alternative: 10m on e2e as well. Not needed if e2e has no `-race`.
   - If CI times out, raise from that log (explore Q3).

3. **Reuse dest concurrent tests.**
   - `TestConcurrentCommandsStayWithinPool`, `TestBurstGetsStayWithinLiveCap`, `TestOverlappingCallersDoNotDialPastLiveCap`, `TestCloseDuringInFlightCommandClosesSocketOnRelease` already share the pool across goroutines. Risk-05 60/200-round loops stay out of scope.

## Risks / Trade-offs

- [Risk] Unit job exceeds 10m under `-race` → Mitigation: raise timeout from the measured CI log; do not silently drop `-race`.
- [Risk] A green race job is read as proof of `len(inUseTurns)` idle-cap logic → Mitigation: spec says it is not; that accounting is another finding.
- [Trade-off] E2E live Close/borrow paths are not race-instrumented. Unit fake-TCP covers the shared-state accesses the detector can see.

## Migration Plan

- Change the unit Run Tests line. Catalog `-race` in the usage packet and README Tests.
- Rollback: revert the workflow line to dest `go test -short -timeout 2m -count=1 -v ./...`.

## Open Questions

None. Explore Qs are assumed on `devstate/explore.md`.
