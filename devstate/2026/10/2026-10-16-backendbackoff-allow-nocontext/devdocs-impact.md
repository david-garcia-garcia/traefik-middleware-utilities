# Devdocs impact
change: backendbackoff-allow-nocontext

Pin: `origin/master` … `ee24c13dbe1c7282031bb9228a37d0e4c5b0cf97` (11 reviewable paths).

## Units
- Backend backoff — subsystem — `backendbackoff/` (`Gate.Allow`, `Gate.Report`); live contract `openspec/specs/std_go_backendbackoff_allow/spec.md` (delta in change folder until archive)

## Findings
none.

## Measure
Packet `std_go_backendbackoff.md` matches the apply: Language **Allow** documents no `context.Context` on the Allow path and caller-owned cancellation; **How to use** and the pattern snippet use `Allow(key)` with a pre-check on `req.Context().Err()`. No other `knowledge/devdocs/` packet names `backendbackoff` or stale `Allow(ctx, …)` for this unit. `README.md` remains out of scope per explore; not a usage-doc packet — no debt row required here.

## Verdict
in progress
