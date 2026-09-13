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

