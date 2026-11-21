# Requirement
IssueKey: 2026-09-13-windowcounter-bug-peek-occupancy

## Problem
After exactly `limit` Takes, Peek returns allowed true with estimate equal to `limit`, and the next Take returns allowed false with estimate `limit+1`. That is occupancy. Docs, spec wording, and a parent repro currently treat Peek's allowed as “would the next Take admit.” Keep Peek's compare. Document occupancy and lock it with a test that must pass on dest code.

## Current (code)
- `windowcounter/limiter.go` Peek godoc — “whether a hit would be allowed” (next-hit wording) while the body does not add a hit.
- `windowcounter/limiter.go` `peekExact` / `peekBuffered` — `estimated <= float64(limit)` on already-used occupancy (GET, or `redis_known + local_delta`; no increment).
- `windowcounter/limiter.go` `takeExact` / `takeBuffered` — increment first, then the same `estimated <= float64(limit)` compare, so Take after occupancy `limit` denies with estimate `limit+1`.
- Dest `windowcounter/` — `repro_peek_take_boundary_test.go` not found. Parent checkout has that file; dest tests do not lock Peek-at-limit versus the next Take.
- `windowcounter/limiter_test.go` `TestPeek_AgreesWithTakeBeforeIncrement` — fills `limit-1`, then Peek then Take; asserts allowed match (increment does not cross `limit`).
- `windowcounter/limiter_test.go` `TestPeek_BufferedTakeThenPeekDenies` — Takes `limit+1`, then Peek denies (occupancy already over).
- `openspec/specs/std_go_windowcounter_sliding-take/spec.md` Requirement “Peek agrees with Take before the increment” — occupancy (“before Take increments”) plus Scenario “Take's allowed matches Peek's allowed” with no “does not cross limit” qualifier.
- `knowledge/devdocs/std_go_windowcounter.md` Language **Peek** — same observation as Take without recording a hit. How to use / snippet Peeks then Takes on backend failure. Gotchas say denied Takes increment and Peek does not; no occupancy-at-limit note.

## Desired
1. Tests first. Create `windowcounter/repro_peek_take_boundary_test.go` from the parent example, but rewrite assertions to the occupancy lock so they pass on current code: after N Takes, Peek allowed=true / est=N; next Take allowed=false / est=N+1. Not a red-fail that Peek allowed must equal next Take allowed.
2. Then docs and spec only. Peek godoc, `knowledge/devdocs/std_go_windowcounter.md` (usage + Gotchas), and the spec: Peek is already-used occupancy, not “would the next Take admit.” Scenario “Take's allowed matches Peek's allowed” applies when the increment does not cross limit. At exactly limit they may differ.
3. Do not change Peek compare (`estimated <= limit` without adding one).
4. Confirm `go test -short -count=1 -timeout 60s ./windowcounter` passes including the occupancy lock test.

## Affected
- `windowcounter/repro_peek_take_boundary_test.go` (new occupancy lock; dest does not have it)
- `windowcounter/limiter.go` Peek godoc only (not the compare)
- `openspec/specs/std_go_windowcounter_sliding-take/spec.md` Peek-agrees requirement and frozen-clock scenario
- `knowledge/devdocs/std_go_windowcounter.md` usage + Gotchas

## Out of scope
- Changing Peek to next-hit (occupancy+1)
- Other windowcounter bugs (expire retry, lock during GET, stale previous)
- Rewriting Take increment-then-compare
- Changing `TestPeek_AgreesWithTakeBeforeIncrement` beyond the non-crossing case it already covers
- Token bucket or leaky bucket

## Unknowns
- Whether the lock keeps `TestRepro_PeekAllowsWhenNextTakeDenies` or is renamed for occupancy.
- Exact Peek godoc and Gotchas wording (propose).
- Whether the spec requirement title stays “Peek agrees with Take before the increment” or is rephrased to occupancy.

## Tensions
- Godoc “whether a hit would be allowed” versus Peek compare on occupancy in `windowcounter/limiter.go`. Ticket: keep occupancy; rewrite godoc.
- Spec scenario “Take's allowed matches Peek's allowed” versus occupancy at exactly limit. Ticket: that match holds when the increment does not cross limit.
- Spec requirement “match the values Take would return before Take increments” is occupancy; the scenario overstates allowed-match. Ticket: qualify the scenario; do not change Peek.
- Parent example `repro_peek_take_boundary_test.go` currently fails if Peek allowed != next Take allowed. Ticket: rewrite so the occupancy lock passes on dest code.
- Usage snippet Peeks then Takes on backend failure (`knowledge/devdocs/std_go_windowcounter.md`): at occupancy=limit, Peek allows then Take denies and still increments. Ticket: document that; do not change Peek.
