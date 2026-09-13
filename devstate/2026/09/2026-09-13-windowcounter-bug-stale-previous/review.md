# Review

## prepare (2026-09-13)
phase: prepare
findings: qualified-with-gaps — Peek-agrees-with-Take vs no GET previous on every Peek; dest buffered share tests stay in the same window
fixed: bus, requirement, stub PR #63
skipped: product apply; other windowcounter bugs

## explore (2026-09-13)
phase: explore
findings: Take-only previous GET; Peek stays skip-storm; unit fake Redis; Peek may lag previous until Take
fixed: explore.md with ranked decisions
skipped: product apply; live e2e case

## propose (2026-09-13)
phase: propose
findings: fold sync-flush and sliding-take; Take-only GET; repro first
fixed: openspec/changes/windowcounter-stale-previous
skipped: product apply

## implement (2026-09-13)
phase: implement
findings: repro failed dest estimated=2; GET previous when expireAt>0; outage test kept
fixed: takeBufferedPreviousLocked, repro_stale_previous_test.go
skipped: live e2e; GET previous on every in-memory localDelta==0
