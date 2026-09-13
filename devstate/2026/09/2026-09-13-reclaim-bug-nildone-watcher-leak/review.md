# Review

## prepare (2026-09-13)
phase: prepare
findings: qualified — dest waitCtx exits only on ctx.Err, so a nil-Done watcher outlives Reset
fixed: bus, requirement, stub PR #77
skipped: product apply; reproducer copy; parallel panic-ignores-enforce branches
