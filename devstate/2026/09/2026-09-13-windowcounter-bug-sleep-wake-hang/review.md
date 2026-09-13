# Review

## prepare (2026-09-13)
phase: prepare
findings: qualified-with-gaps — spec Wake SHALL start when sync_rate > 0 has no Sleep-in-progress exception; dest stopFlushAndWait clears stop before wg.Wait
fixed: bus, requirement, stub PR #61
skipped: product apply; other windowcounter bugs

## explore (2026-09-13)
phase: explore
findings: dest race hung iteration 2 after 2s; Sleep-wins stopping flag; four assumed additive asked
fixed: explore.md
skipped: product apply
