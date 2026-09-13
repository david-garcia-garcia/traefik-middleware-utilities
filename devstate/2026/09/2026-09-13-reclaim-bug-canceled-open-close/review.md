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

## implement (2026-09-13)
phase: implement
findings: new tests failed on dest (nil err); after finishBind they pass; waiter test needed positive grace
fixed: finishBind at put, awake bind, reclaimLocked; waitGraceOrWake; usage packet
skipped: abort put before create (would starve waiters)

## codereview (2026-09-13)
phase: codereview
findings: P1 0, P2 0; Standards 1 hard Leave-a-trail on drop comment
fixed: drop/Table overview comments (`341db7d`)
skipped: none

## devdocsimpact (2026-09-13)
phase: devdocsimpact
findings: none — Reclaim packet already matched the apply
fixed: none needed
skipped: none

## archive (2026-09-13)
phase: archive
findings: fold std_go_reclaim_context-lease; map and names OK
fixed: main spec synced; change archived as 2026-09-13-reclaim-canceled-bind
skipped: none

## pullrequest (2026-09-13)
phase: pullrequest
findings: PR #51 summary set; CI 34742827420 success on 9df18e4; token cannot undraft
fixed: ready title already set; delivery card on pr-body
skipped: draft:false (PAT Resource not accessible)
