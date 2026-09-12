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

## implement (2026-09-12)
phase: implement
findings: buffered Take/Peek return lastFlushErr after k=1; tests passed
fixed: limiter.go, killable fake, four proofs, docs
skipped: none

## codereview (2026-09-12)
phase: codereview
findings: nitpicks 1 done; coverage 2 done; dead 1 skipped (flushFailedAt)
fixed: early-return membership; Sleep and Peek probe tests
skipped: flushFailedAt unused (Desired names the store; LastFlushError out of scope)
