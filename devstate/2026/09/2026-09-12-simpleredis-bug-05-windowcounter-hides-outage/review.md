# Review

## prepare (2026-09-12)
phase: prepare
findings: qualified-with-gaps — buffered flush errors discarded; Take silent while localDelta > 0; three surfaces unpicked
fixed: bus, requirement, stub PR #30
skipped: none

## explore (2026-09-12)
phase: explore
findings: surface = staleness k=1 + lastFlushErr on Take/Peek; Peek same; keep 3-tuple
fixed: explore.md decisions assumed so implement can proceed
skipped: Kong flush-fail clone

## propose (2026-09-12)
phase: propose
findings: fold sliding-take and sync-flush; change windowcounter-buffered-flush-error
fixed: proposal, design, tasks, two delta specs
skipped: none
