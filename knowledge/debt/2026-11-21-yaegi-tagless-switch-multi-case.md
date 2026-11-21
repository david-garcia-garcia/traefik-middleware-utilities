# Characterize the Yaegi tagless `switch` multi-expression `case` misevaluation and guard session source

IssueKey: 2026-11-21-simpleredis-bug-yaegi-select
Size: large
Action: note

## Why this follow-up
While this run built the interpreted `SaturatedGet` probe, a tagless `switch { case IsUnreachable(err), errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled): ... default: ... }` run under Yaegi v0.16.1 sent errors whose type was `context.deadlineExceededError` to `default`. Compiled, the same code matched the case. The same checks written as one `||` boolean classified every error correctly under Yaegi. This was observed, not isolated: it is not known whether the cause is the multi-expression case list, the function calls in it, or the mix of the two. The probe uses the `||` form, and a comment there records the observation.

## Why it was not taken
This is a different interpreter defect from the shared `select` case list that this run fixes. An AST scan of the non-test source in `simpleredis`, `reclaim`, `windowcounter`, `tokenbucket`, `iplookup` and `backendbackoff` found no tagless `switch` with a multi-expression `case` (0 hits), so no production path is known to be affected today. Isolating the defect in Yaegi and adding a guard is research and test work outside this ticket's scope.

## Risks
A later change that writes a tagless `switch` with a multi-expression `case` in session source could misroute errors under Traefik while every compiled test still passes.

## Context
Observed in: `simpleredis/repro_yaegi_select_probe_test.go` `SelectProbeSaturatedGet`.
Proposed: reduce the case to a minimal repro under `interp.New` and record it under `knowledge/research/` (or upstream). If the case list is confirmed as the cause, extend the `go/parser` guard next to `TestSessionSourceHasNoSelect` to reject a tagless `switch` with a multi-expression `case` in session source.
