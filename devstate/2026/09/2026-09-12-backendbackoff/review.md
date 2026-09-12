# Review

## prepare (2026-09-12)
phase: prepare
findings: none
fixed: none
skipped: none

## explore (2026-09-12)
phase: explore
findings: none
fixed: none
skipped: assumed defaults, names, retryAfter, no Snapshot, lost-probe lease; deferred shared layer noted

## propose (2026-09-12)
phase: propose
findings: none
fixed: none
skipped: none

## implement (2026-09-12)
phase: implement
findings: none
fixed: backendbackoff Gate Allow/Report/Close
skipped: local -race (no cgo); CI Unit race will prove it

## codereview (2026-09-12)
phase: codereview
findings: Report-after-TTL, idle proof, probe credit, lost-probe, OPEN Report ignored, cap at B, Yaegi filename, job comments, key not source
fixed: those hard/wrong items
skipped: extra New validation tests, 65536 dropOne, jitter-on/MaxCooldown math

## devdocsimpact (2026-09-12)
phase: devdocsimpact
findings: Report-after-TTL gotcha missing from usage packet
fixed: gotcha added to std_go_backendbackoff.md
skipped: none

## archive (2026-09-12)
phase: archive
findings: none
fixed: live specs std_go_backendbackoff_allow and std_go_backendbackoff_cooldown; change moved to archive/2026-09-12-backendbackoff-gate
skipped: none
