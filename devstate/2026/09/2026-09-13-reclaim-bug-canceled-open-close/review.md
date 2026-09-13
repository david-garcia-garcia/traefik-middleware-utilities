# Review

## prepare (2026-09-13)
phase: prepare
findings: qualified-with-gaps — spec still says every Open receives the value; ticket carves canceled bind
fixed: bus, requirement, stub PR #51
skipped: product apply; other two reclaim bugs

## explore (2026-09-13)
phase: explore
findings: dest returns (value, nil) after cancel-during-create; Close-before-return is racy; doomed pointer reproduced
fixed: explore.md with assumed ranks on test file, grace for reclaim, reclaimLocked error return, extra pre-create check, waiter test
skipped: product apply; usage rewrite (dest does not have the contract yet)

## propose (2026-09-13)
phase: propose
findings: FindSpecHost fold std_go_reclaim_context-lease; change reclaim-canceled-bind valid
fixed: proposal, delta spec, design, tasks
skipped: product apply
