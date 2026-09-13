# Review

## prepare (2026-09-13)
phase: prepare
findings: qualified — dest waitCtx exits only on ctx.Err, so a nil-Done watcher outlives Reset
fixed: bus, requirement, stub PR #77
skipped: product apply; reproducer copy; parallel panic-ignores-enforce branches

## explore (2026-09-13)
phase: explore
findings: shape 1 finished channel; four slotGone writers; verbatim reproducer needs test-only NewTable
fixed: explore.md, deviations.md, PR #77 card
skipped: product apply

## propose (2026-09-13)
phase: propose
findings: fold std_go_reclaim_context-lease; change reclaim-nildone-watcher-exit
fixed: proposal, design, tasks, delta spec, specs.md
skipped: product apply

## implement (2026-09-13)
phase: implement
findings: shape 1 landed; TestRepro failed then passed; go test ./reclaim ok; go test ./... -short ok
fixed: table.go finished channel; verbatim reproducer; test-only NewTable; usage gotcha
skipped: Sleep-panic and Wake-panic bodies; reclaim/BUGS.md

