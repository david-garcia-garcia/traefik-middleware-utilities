# Replace the `select` in reclaim `waitCtx` and guard every production package against `select`

IssueKey: 2026-11-21-simpleredis-bug-yaegi-select
Size: large
Action: note

## Why this follow-up
`reclaim/table.go` `waitCtx` still runs two `select` statements (on `finished` and on `tick.C`) from one goroutine per holder whose `Done()` is nil. That nil-Done shape is what Background holders look like under Yaegi. Yaegi v0.16.1 shares one case slice per `select` statement across goroutines (`knowledge/research/ext_traefik_plugins_yaegi-select/`), so one holder's watcher can wait on another holder's `finished` channel or ticker. After this change, `reclaim` is the only production package that still has a `select`. The AST guard that this change adds covers only `simpleredis`.

## Why it was not taken
The requirement for this run scopes the fix to `simpleredis`. It leaves `waitCtx` for explore to decide, and rates the impact low. The cross-talk is an inference from the code and was not reproduced: those holders' `Err()` never changes, so the most it would do is keep a watcher goroutine alive until some other incarnation ends. The fix is in a different package that has its own spec (`std_go_reclaim_context-lease`) and its own repro (`reclaim/zzz_repro_nildone_watcher_test.go`). In an unattended run, only small rows on files this run created are taken.

## Risks
Under Traefik, a watcher goroutine could be retained for each Background holder for longer than its incarnation lasts. Any new `select` added to `reclaim`, `windowcounter`, `tokenbucket`, `backendbackoff`, `iplookup`, or `traefikemulator` would not be caught by a test.

## Context
Current: `reclaim/table.go` `waitCtx` (lines 149–172), which is called from `watch` (line 574).
Proposed: wait on `finished` with a plain receive, because a nil `Done()` never cancels. Alternatively, use an AfterFunc shape like `windowcounter/limiter.go`. Then add the `go/parser` guard from `simpleredis/zzz_simpleredis_test.go` to each production package.
Stale neighbour: `knowledge/debt/2026-09-13-reclaim-grace-select.md` names `waitGraceOrWake`, which no longer exists. `reclaim/table.go:651` already uses `time.AfterFunc`.
