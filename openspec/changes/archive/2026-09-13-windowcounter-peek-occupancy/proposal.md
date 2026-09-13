## Why

Peek already reports already-used occupancy (`estimated <= limit` without adding a hit). After exactly `limit` Takes, Peek allows with estimate `limit` and the next Take denies with estimate `limit+1`. Godoc, usage, and the sliding-take scenario still read as “would the next Take admit,” so a later change can flip Peek to occupancy+1 or a caller can treat Peek allowed as a reservation.

## What Changes

- Tests first: `windowcounter/repro_peek_take_boundary_test.go` locks occupancy. After N Takes, Peek allowed true / estimate N; next Take allowed false / estimate N+1. Exact and buffered. The test MUST pass on dest Peek. Do not assert that Peek allowed equals the next Take's allowed.
- Then Peek godoc, `knowledge/devdocs/std_go_windowcounter.md` (How to use + Gotchas), and this spec: Peek is occupancy, not next-hit. Scenario “Take's allowed matches Peek's allowed” applies when the increment does not cross limit. At exactly limit they may differ.
- Do not change Peek compare.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `std_go_windowcounter_sliding-take`: Peek's allowed is occupancy (`estimated <= limit` without adding a hit). Allowed-match with the next Take holds when that increment does not cross limit. At occupancy equal to limit, Peek MAY allow and the next Take MUST deny.

## Impact

- `windowcounter/repro_peek_take_boundary_test.go` (new occupancy lock)
- `windowcounter/limiter.go` Peek godoc only
- `openspec/specs/std_go_windowcounter_sliding-take/spec.md` (after archive)
- `knowledge/devdocs/std_go_windowcounter.md`
- Peek compare, Take increment-then-compare, tokenbucket, other windowcounter bugs: unchanged
